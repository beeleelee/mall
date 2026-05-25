package order

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/order"
)

type webhookRow struct {
	ID        int64     `db:"id"`
	UserID    int64     `db:"user_id"`
	URL       string    `db:"url"`
	Secret    string    `db:"secret"`
	Events    []string  `db:"events"`
	Active    bool      `db:"active"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (r webhookRow) toDomain() *domain.Webhook {
	w := &domain.Webhook{
		AggregateRoot: kernel.NewAggregateRoot(kernel.ID(r.ID)),
		UserID:        kernel.ID(r.UserID),
		URL:           r.URL,
		Secret:        r.Secret,
		Events:        r.Events,
		Active:        r.Active,
	}
	w.CreatedAt = r.CreatedAt
	w.UpdatedAt = r.UpdatedAt
	return w
}

func fromDomainWebhook(w *domain.Webhook) webhookRow {
	return webhookRow{
		ID:        w.ID.Int64(),
		UserID:    w.UserID.Int64(),
		URL:       w.URL,
		Secret:    w.Secret,
		Events:    w.Events,
		Active:    w.Active,
		CreatedAt: w.CreatedAt,
		UpdatedAt: w.UpdatedAt,
	}
}

type PostgresWebhookRepository struct {
	db *sqlx.DB
}

func NewPostgresWebhookRepository(db *sqlx.DB) *PostgresWebhookRepository {
	return &PostgresWebhookRepository{db: db}
}

func (r *PostgresWebhookRepository) Save(ctx context.Context, webhook *domain.Webhook) error {
	row := fromDomainWebhook(webhook)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO webhooks (id, user_id, url, secret, events, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			url = EXCLUDED.url,
			secret = EXCLUDED.secret,
			events = EXCLUDED.events,
			active = EXCLUDED.active,
			updated_at = EXCLUDED.updated_at
	`, row.ID, row.UserID, row.URL, row.Secret, row.Events, row.Active, row.CreatedAt, row.UpdatedAt)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save webhook", err)
	}
	return nil
}

func (r *PostgresWebhookRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.Webhook, error) {
	var row webhookRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM webhooks WHERE id = $1`, id.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "webhook not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find webhook by id", err)
	}
	return row.toDomain(), nil
}

func (r *PostgresWebhookRepository) FindByUserID(ctx context.Context, userID kernel.ID) ([]*domain.Webhook, error) {
	var rows []webhookRow
	err := r.db.SelectContext(ctx, &rows, `SELECT * FROM webhooks WHERE user_id = $1 ORDER BY created_at DESC`, userID.Int64())
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find webhooks by user", err)
	}

	result := make([]*domain.Webhook, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, nil
}

func (r *PostgresWebhookRepository) FindByEvent(ctx context.Context, event string) ([]*domain.Webhook, error) {
	var rows []webhookRow
	err := r.db.SelectContext(ctx, &rows, `SELECT * FROM webhooks WHERE active = true AND $1 = ANY(events)`, event)
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find webhooks by event", err)
	}

	result := make([]*domain.Webhook, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, nil
}

func (r *PostgresWebhookRepository) Delete(ctx context.Context, id kernel.ID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = $1`, id.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "delete webhook", err)
	}
	return nil
}

func SignWebhookPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

type WebhookDeliverer struct {
	client *http.Client
}

func NewWebhookDeliverer() *WebhookDeliverer {
	return &WebhookDeliverer{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (d *WebhookDeliverer) Deliver(ctx context.Context, webhook *domain.Webhook, event string, payload []byte) error {
	signature := SignWebhookPayload(webhook.Secret, payload)
	timestamp := time.Now().UnixMilli()

	body := map[string]any{
		"event":     event,
		"timestamp": timestamp,
		"payload":   json.RawMessage(payload),
	}

	data, err := json.Marshal(body)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal webhook payload", err)
	}

	var lastErr error
	for i := 0; i < 3; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook.URL, bytes.NewReader(data))
		if err != nil {
			return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "create webhook request", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Signature-256", signature)
		req.Header.Set("X-Signature-Timestamp", fmt.Sprintf("%d", timestamp))

		resp, err := d.client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Second)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		lastErr = kernel.NewDomainError(kernel.ErrUnavailable, fmt.Sprintf("webhook returned status %d", resp.StatusCode))
		time.Sleep(time.Second)
	}

	return kernel.NewDomainErrorWithCause(kernel.ErrUnavailable, "webhook delivery failed after 3 retries", lastErr)
}
