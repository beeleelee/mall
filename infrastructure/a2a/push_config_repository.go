package a2a

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"

	domain "github.com/beeleelee/mall/domain/a2a"
	"github.com/beeleelee/mall/domain/kernel"
)

type pushConfigRow struct {
	ID              int64     `db:"id"`
	TaskID          int64     `db:"task_id"`
	URL             string    `db:"url"`
	AuthScheme      string    `db:"auth_scheme"`
	AuthCredentials string    `db:"auth_credentials"`
	CreatedAt       time.Time `db:"created_at"`
}

func (r *pushConfigRow) toDomain() *domain.PushNotificationConfig {
	return &domain.PushNotificationConfig{
		AggregateRoot: kernel.NewAggregateRoot(kernel.ID(r.ID)),
		TaskID:        kernel.ID(r.TaskID),
		URL:           r.URL,
		AuthInfo: domain.AuthInfo{
			Scheme:      r.AuthScheme,
			Credentials: r.AuthCredentials,
		},
	}
}

func fromPushConfigDomain(cfg *domain.PushNotificationConfig) *pushConfigRow {
	return &pushConfigRow{
		ID:              cfg.ID.Int64(),
		TaskID:          cfg.TaskID.Int64(),
		URL:             cfg.URL,
		AuthScheme:      cfg.AuthInfo.Scheme,
		AuthCredentials: cfg.AuthInfo.Credentials,
	}
}

type PostgresPushNotificationConfigRepository struct {
	db *sqlx.DB
}

func NewPostgresPushNotificationConfigRepository(db *sqlx.DB) *PostgresPushNotificationConfigRepository {
	return &PostgresPushNotificationConfigRepository{db: db}
}

func (r *PostgresPushNotificationConfigRepository) Save(ctx context.Context, cfg *domain.PushNotificationConfig) error {
	row := fromPushConfigDomain(cfg)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO a2a_push_notification_configs (id, task_id, url, auth_scheme, auth_credentials)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			task_id = EXCLUDED.task_id,
			url = EXCLUDED.url,
			auth_scheme = EXCLUDED.auth_scheme,
			auth_credentials = EXCLUDED.auth_credentials
	`, row.ID, row.TaskID, row.URL, row.AuthScheme, row.AuthCredentials)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save a2a push config", err)
	}
	return nil
}

func (r *PostgresPushNotificationConfigRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.PushNotificationConfig, error) {
	var row pushConfigRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM a2a_push_notification_configs WHERE id = $1`, id.Int64())
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find a2a push config by id", err)
	}
	return row.toDomain(), nil
}

func (r *PostgresPushNotificationConfigRepository) FindByTaskID(ctx context.Context, taskID kernel.ID) ([]*domain.PushNotificationConfig, error) {
	var rows []pushConfigRow
	err := r.db.SelectContext(ctx, &rows, `SELECT * FROM a2a_push_notification_configs WHERE task_id = $1`, taskID.Int64())
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find a2a push configs by task", err)
	}
	configs := make([]*domain.PushNotificationConfig, 0, len(rows))
	for _, row := range rows {
		configs = append(configs, row.toDomain())
	}
	return configs, nil
}

func (r *PostgresPushNotificationConfigRepository) Delete(ctx context.Context, id kernel.ID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM a2a_push_notification_configs WHERE id = $1`, id.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "delete a2a push config", err)
	}
	return nil
}
