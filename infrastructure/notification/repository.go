package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/notification"
)

type notificationRow struct {
	ID        int64     `db:"id"`
	UserID    int64     `db:"user_id"`
	Type      string    `db:"type"`
	Title     string    `db:"title"`
	Body      string    `db:"body"`
	Read      bool      `db:"read"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (r notificationRow) toDomain() *domain.Notification {
	return &domain.Notification{
		AggregateRoot: kernel.AggregateRoot{
			Entity: kernel.Entity{
				ID:        kernel.ID(r.ID),
				CreatedAt: r.CreatedAt,
				UpdatedAt: r.UpdatedAt,
			},
		},
		UserID: kernel.ID(r.UserID),
		Type:   domain.NotificationType(r.Type),
		Title:  r.Title,
		Body:   r.Body,
		Read:   r.Read,
	}
}

func fromNotification(n *domain.Notification) notificationRow {
	return notificationRow{
		ID:        n.ID.Int64(),
		UserID:    n.UserID.Int64(),
		Type:      string(n.Type),
		Title:     n.Title,
		Body:      n.Body,
		Read:      n.Read,
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
}

type PostgresNotificationRepository struct {
	db *sqlx.DB
}

func NewPostgresNotificationRepository(db *sqlx.DB) *PostgresNotificationRepository {
	return &PostgresNotificationRepository{db: db}
}

func (r *PostgresNotificationRepository) Write(ctx context.Context, n *domain.Notification) error {
	return r.Save(ctx, n)
}

func (r *PostgresNotificationRepository) Save(ctx context.Context, n *domain.Notification) error {
	row := fromNotification(n)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO notifications (id, user_id, type, title, body, read, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			body = EXCLUDED.body,
			read = EXCLUDED.read,
			updated_at = NOW()
	`, row.ID, row.UserID, row.Type, row.Title, row.Body, row.Read)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save notification", err)
	}
	return nil
}

func (r *PostgresNotificationRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.Notification, error) {
	var row notificationRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM notifications WHERE id = $1`, id.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "notification not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find notification by id", err)
	}
	return row.toDomain(), nil
}

func (r *PostgresNotificationRepository) FindByUserID(ctx context.Context, userID kernel.ID, limit int) ([]*domain.Notification, error) {
	if limit <= 0 {
		limit = 50
	}
	rows := []notificationRow{}
	if err := r.db.SelectContext(ctx, &rows,
		`SELECT * FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`,
		userID.Int64(), limit); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find notifications by user", err)
	}
	result := make([]*domain.Notification, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, nil
}

func (r *PostgresNotificationRepository) MarkRead(ctx context.Context, id kernel.ID, userID kernel.ID) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET read = TRUE, updated_at = NOW() WHERE id = $1 AND user_id = $2`,
		id.Int64(), userID.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "mark notification read", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return kernel.NewDomainError(kernel.ErrNotFound, "notification not found")
	}
	return nil
}

func (r *PostgresNotificationRepository) MarkAllRead(ctx context.Context, userID kernel.ID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET read = TRUE, updated_at = NOW() WHERE user_id = $1 AND read = FALSE`,
		userID.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "mark all notifications read", err)
	}
	return nil
}

func (r *PostgresNotificationRepository) UnreadCount(ctx context.Context, userID kernel.ID) (int, error) {
	var count int
	if err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = FALSE`, userID.Int64()); err != nil {
		return 0, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "count unread notifications", err)
	}
	return count, nil
}

type preferenceRow struct {
	ID           int64           `db:"id"`
	UserID       int64           `db:"user_id"`
	EmailEnabled bool            `db:"email_enabled"`
	InAppEnabled bool            `db:"in_app_enabled"`
	Types        json.RawMessage `db:"types"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`
}

func (r preferenceRow) toDomain() (*domain.NotificationPreferences, error) {
	var types []domain.NotificationType
	if len(r.Types) > 0 {
		if err := json.Unmarshal(r.Types, &types); err != nil {
			return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "unmarshal notification preference types", err)
		}
	}
	prefs := &domain.NotificationPreferences{
		Entity: kernel.Entity{
			ID:        kernel.ID(r.ID),
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		},
		UserID:       kernel.ID(r.UserID),
		EmailEnabled: r.EmailEnabled,
		InAppEnabled: r.InAppEnabled,
		Types:        &types,
	}
	return prefs, nil
}

func fromPreferences(p *domain.NotificationPreferences) (preferenceRow, error) {
	types := []domain.NotificationType{}
	if p.Types != nil {
		types = *p.Types
	}
	raw, err := json.Marshal(types)
	if err != nil {
		return preferenceRow{}, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal notification preference types", err)
	}
	return preferenceRow{
		ID:           p.ID.Int64(),
		UserID:       p.UserID.Int64(),
		EmailEnabled: p.EmailEnabled,
		InAppEnabled: p.InAppEnabled,
		Types:        raw,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}, nil
}

type PostgresNotificationPreferenceRepository struct {
	db *sqlx.DB
}

func NewPostgresNotificationPreferenceRepository(db *sqlx.DB) *PostgresNotificationPreferenceRepository {
	return &PostgresNotificationPreferenceRepository{db: db}
}

func (r *PostgresNotificationPreferenceRepository) Get(ctx context.Context, userID kernel.ID) (*domain.NotificationPreferences, error) {
	var row preferenceRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM notification_preferences WHERE user_id = $1`, userID.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "notification preferences not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "get notification preferences", err)
	}
	return row.toDomain()
}

func (r *PostgresNotificationPreferenceRepository) Upsert(ctx context.Context, prefs *domain.NotificationPreferences) error {
	row, err := fromPreferences(prefs)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO notification_preferences (id, user_id, email_enabled, in_app_enabled, types, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			email_enabled = EXCLUDED.email_enabled,
			in_app_enabled = EXCLUDED.in_app_enabled,
			types = EXCLUDED.types,
			updated_at = NOW()
	`, row.ID, row.UserID, row.EmailEnabled, row.InAppEnabled, row.Types)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "upsert notification preferences", err)
	}
	return nil
}
