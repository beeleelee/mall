package order

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/order"
)

type webhookRow struct {
	ID        int64           `db:"id"`
	UserID    int64           `db:"user_id"`
	URL       string          `db:"url"`
	Secret    string          `db:"secret"`
	Events    json.RawMessage `db:"events"`
	Active    bool            `db:"active"`
	CreatedAt time.Time       `db:"created_at"`
	UpdatedAt time.Time       `db:"updated_at"`
}

func (r webhookRow) toDomain() *domain.Webhook {
	var events []string
	if len(r.Events) > 0 {
		_ = json.Unmarshal(r.Events, &events)
	}
	if events == nil {
		events = []string{}
	}
	w := &domain.Webhook{
		AggregateRoot: kernel.NewAggregateRoot(kernel.ID(r.ID)),
		UserID:        kernel.ID(r.UserID),
		URL:           r.URL,
		Secret:        r.Secret,
		Events:        events,
		Active:        r.Active,
	}
	w.CreatedAt = r.CreatedAt
	w.UpdatedAt = r.UpdatedAt
	return w
}

func fromDomainWebhook(w *domain.Webhook) webhookRow {
	events, _ := json.Marshal(w.Events)
	return webhookRow{
		ID:        w.ID.Int64(),
		UserID:    w.UserID.Int64(),
		URL:       w.URL,
		Secret:    w.Secret,
		Events:    events,
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
	err := r.db.SelectContext(ctx, &rows, `SELECT * FROM webhooks WHERE active = true AND EXISTS (SELECT 1 FROM jsonb_array_elements_text(events) AS e WHERE e = $1)`, event)
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

func deliveryLogEntryToRow(e *domain.DeliveryLogEntry) deliveryLogRow {
	var errPtr *string
	if e.Error != "" {
		errPtr = &e.Error
	}
	return deliveryLogRow{
		ID:        e.ID,
		WebhookID: e.WebhookID,
		Event:     e.Event,
		Payload:   e.Payload,
		Status:    e.Status,
		Error:     errPtr,
		Attempts:  e.Attempts,
		NextRetry: e.NextRetry,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

func (r deliveryLogRow) toDomain() domain.DeliveryLogEntry {
	var errStr string
	if r.Error != nil {
		errStr = *r.Error
	}
	return domain.DeliveryLogEntry{
		ID:        r.ID,
		WebhookID: r.WebhookID,
		Event:     r.Event,
		Payload:   r.Payload,
		Status:    r.Status,
		Error:     errStr,
		Attempts:  r.Attempts,
		NextRetry: r.NextRetry,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

type deliveryLogRow struct {
	ID        int64      `db:"id"`
	WebhookID int64      `db:"webhook_id"`
	Event     string     `db:"event"`
	Payload   []byte     `db:"payload"`
	Status    string     `db:"status"`
	Error     *string    `db:"error"`
	Attempts  int        `db:"attempts"`
	NextRetry *time.Time `db:"next_retry"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
}

type PostgresWebhookDeliveryLogRepository struct {
	db *sqlx.DB
	sf *kernel.Snowflake
}

func NewPostgresWebhookDeliveryLogRepository(db *sqlx.DB, sf *kernel.Snowflake) *PostgresWebhookDeliveryLogRepository {
	return &PostgresWebhookDeliveryLogRepository{db: db, sf: sf}
}

func (r *PostgresWebhookDeliveryLogRepository) Save(ctx context.Context, entry *domain.DeliveryLogEntry) error {
	if entry.ID == 0 {
		id, err := r.sf.NextID()
		if err != nil {
			return err
		}
		entry.ID = id.Int64()
	}

	row := deliveryLogEntryToRow(entry)
	_, dbErr := r.db.ExecContext(ctx, `
		INSERT INTO webhook_delivery_log (id, webhook_id, event, payload, status, error, attempts, next_retry, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			error = EXCLUDED.error,
			attempts = EXCLUDED.attempts,
			next_retry = EXCLUDED.next_retry,
			updated_at = NOW()
	`, row.ID, row.WebhookID, row.Event, row.Payload, row.Status, row.Error, row.Attempts, row.NextRetry)
	if dbErr != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save delivery log", dbErr)
	}
	return nil
}

func (r *PostgresWebhookDeliveryLogRepository) MarkRetried(ctx context.Context, logID int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE webhook_delivery_log SET status = 'retried', updated_at = NOW() WHERE id = $1", logID)
	return err
}

func (r *PostgresWebhookDeliveryLogRepository) MarkDelivered(ctx context.Context, logID int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE webhook_delivery_log SET status = 'delivered', updated_at = NOW() WHERE id = $1", logID)
	return err
}

func (r *PostgresWebhookDeliveryLogRepository) ListFailed(ctx context.Context, limit int) ([]domain.DeliveryLogEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []deliveryLogRow
	err := r.db.SelectContext(ctx, &rows, "SELECT * FROM webhook_delivery_log WHERE status = 'failed' ORDER BY created_at DESC LIMIT $1", limit)
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "list failed deliveries", err)
	}
	result := make([]domain.DeliveryLogEntry, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, nil
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
	signature := domain.SignWebhookPayload(webhook.Secret, payload)
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

	maxRetries := 3
	var lastErr error
	for i := 0; i < maxRetries; i++ {
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
			if i < maxRetries-1 {
				backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		lastErr = kernel.NewDomainError(kernel.ErrUnavailable, fmt.Sprintf("webhook returned status %d", resp.StatusCode))
		if i < maxRetries-1 {
			backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return kernel.NewDomainErrorWithCause(kernel.ErrUnavailable, "webhook delivery failed after 3 retries", lastErr)
}
