package subscription

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/subscription"
)

type planRow struct {
	ID            int64           `db:"id"`
	Name          string          `db:"name"`
	Description   string          `db:"description"`
	Amount        int64           `db:"amount"`
	Interval      string          `db:"interval"`
	IntervalCount int             `db:"interval_count"`
	TrialDays     int             `db:"trial_days"`
	Features      json.RawMessage `db:"features"`
	Status        string          `db:"status"`
	CreatedAt     time.Time       `db:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at"`
}

func (r planRow) toDomain() (*domain.Plan, error) {
	var features []string
	if len(r.Features) > 0 {
		if err := json.Unmarshal(r.Features, &features); err != nil {
			return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "unmarshal plan features", err)
		}
	}
	return &domain.Plan{
		Entity: kernel.Entity{
			ID:        kernel.ID(r.ID),
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		},
		Name:          r.Name,
		Description:   r.Description,
		Amount:        r.Amount,
		Interval:      r.Interval,
		IntervalCount: r.IntervalCount,
		TrialDays:     r.TrialDays,
		Features:      features,
		Status:        domain.PlanStatus(r.Status),
	}, nil
}

func (r planRow) fromDomain(p *domain.Plan) planRow {
	features, _ := json.Marshal(p.Features)
	return planRow{
		ID:            p.ID.Int64(),
		Name:          p.Name,
		Description:   p.Description,
		Amount:        p.Amount,
		Interval:      p.Interval,
		IntervalCount: p.IntervalCount,
		TrialDays:     p.TrialDays,
		Features:      features,
		Status:        string(p.Status),
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
}

type PostgresPlanRepository struct {
	db  *sqlx.DB
	rdb *redis.Client
}

func NewPostgresPlanRepository(db *sqlx.DB, rdb *redis.Client) *PostgresPlanRepository {
	return &PostgresPlanRepository{db: db, rdb: rdb}
}

func (r *PostgresPlanRepository) Save(ctx context.Context, plan *domain.Plan) error {
	row := planRow{}.fromDomain(plan)
	query := `INSERT INTO subscription_plans (id, name, description, amount, interval, interval_count, trial_days, features, status, created_at, updated_at)
		VALUES (:id, :name, :description, :amount, :interval, :interval_count, :trial_days, :features, :status, :created_at, :updated_at)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name, description = EXCLUDED.description, amount = EXCLUDED.amount,
			interval = EXCLUDED.interval, interval_count = EXCLUDED.interval_count,
			trial_days = EXCLUDED.trial_days, features = EXCLUDED.features,
			status = EXCLUDED.status, updated_at = EXCLUDED.updated_at`
	if _, err := r.db.NamedExecContext(ctx, query, row); err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save plan", err)
	}
	return nil
}

func (r *PostgresPlanRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.Plan, error) {
	var row planRow
	if err := r.db.GetContext(ctx, &row, "SELECT * FROM subscription_plans WHERE id = $1", id.Int64()); err != nil {
		if err == sql.ErrNoRows {
			return nil, kernel.NewDomainError(kernel.ErrNotFound, "plan not found")
		}
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find plan by id", err)
	}
	return row.toDomain()
}

func (r *PostgresPlanRepository) FindAll(ctx context.Context) ([]*domain.Plan, error) {
	var rows []planRow
	if err := r.db.SelectContext(ctx, &rows, "SELECT * FROM subscription_plans ORDER BY amount ASC"); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find all plans", err)
	}
	plans := make([]*domain.Plan, 0, len(rows))
	for _, row := range rows {
		p, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, nil
}

func (r *PostgresPlanRepository) FindActive(ctx context.Context) ([]*domain.Plan, error) {
	var rows []planRow
	if err := r.db.SelectContext(ctx, &rows, "SELECT * FROM subscription_plans WHERE status = 'active' ORDER BY amount ASC"); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find active plans", err)
	}
	plans := make([]*domain.Plan, 0, len(rows))
	for _, row := range rows {
		p, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, nil
}

type subscriptionRow struct {
	ID                 int64      `db:"id"`
	UserID             int64      `db:"user_id"`
	PlanID             int64      `db:"plan_id"`
	Status             string     `db:"status"`
	CurrentPeriodStart time.Time  `db:"current_period_start"`
	CurrentPeriodEnd   time.Time  `db:"current_period_end"`
	TrialEndsAt        *time.Time `db:"trial_ends_at"`
	CancelledAt        *time.Time `db:"cancelled_at"`
	PaymentToken       string     `db:"payment_token"`
	CreatedAt          time.Time  `db:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at"`
}

func (r subscriptionRow) toDomain() *domain.Subscription {
	return &domain.Subscription{
		AggregateRoot: kernel.AggregateRoot{
			Entity: kernel.Entity{
				ID:        kernel.ID(r.ID),
				CreatedAt: r.CreatedAt,
				UpdatedAt: r.UpdatedAt,
			},
		},
		UserID:             kernel.ID(r.UserID),
		PlanID:             kernel.ID(r.PlanID),
		Status:             domain.SubscriptionStatus(r.Status),
		CurrentPeriodStart: r.CurrentPeriodStart,
		CurrentPeriodEnd:   r.CurrentPeriodEnd,
		TrialEndsAt:        r.TrialEndsAt,
		CancelledAt:        r.CancelledAt,
		PaymentToken:       r.PaymentToken,
	}
}

func (r subscriptionRow) fromDomain(s *domain.Subscription) subscriptionRow {
	return subscriptionRow{
		ID:                 s.ID.Int64(),
		UserID:             s.UserID.Int64(),
		PlanID:             s.PlanID.Int64(),
		Status:             string(s.Status),
		CurrentPeriodStart: s.CurrentPeriodStart,
		CurrentPeriodEnd:   s.CurrentPeriodEnd,
		TrialEndsAt:        s.TrialEndsAt,
		CancelledAt:        s.CancelledAt,
		PaymentToken:       s.PaymentToken,
		CreatedAt:          s.CreatedAt,
		UpdatedAt:          s.UpdatedAt,
	}
}

type PostgresSubscriptionRepository struct {
	db  *sqlx.DB
	rdb *redis.Client
}

func NewPostgresSubscriptionRepository(db *sqlx.DB, rdb *redis.Client) *PostgresSubscriptionRepository {
	return &PostgresSubscriptionRepository{db: db, rdb: rdb}
}

func (r *PostgresSubscriptionRepository) Save(ctx context.Context, sub *domain.Subscription) error {
	row := subscriptionRow{}.fromDomain(sub)
	query := `INSERT INTO subscriptions (id, user_id, plan_id, status, current_period_start, current_period_end, trial_ends_at, cancelled_at, payment_token, created_at, updated_at)
		VALUES (:id, :user_id, :plan_id, :status, :current_period_start, :current_period_end, :trial_ends_at, :cancelled_at, :payment_token, :created_at, :updated_at)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status, plan_id = EXCLUDED.plan_id,
			current_period_start = EXCLUDED.current_period_start, current_period_end = EXCLUDED.current_period_end,
			trial_ends_at = EXCLUDED.trial_ends_at, cancelled_at = EXCLUDED.cancelled_at,
			payment_token = EXCLUDED.payment_token,
			updated_at = EXCLUDED.updated_at`
	if _, err := r.db.NamedExecContext(ctx, query, row); err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save subscription", err)
	}
	return nil
}

func (r *PostgresSubscriptionRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.Subscription, error) {
	var row subscriptionRow
	if err := r.db.GetContext(ctx, &row, "SELECT * FROM subscriptions WHERE id = $1", id.Int64()); err != nil {
		if err == sql.ErrNoRows {
			return nil, kernel.NewDomainError(kernel.ErrNotFound, "subscription not found")
		}
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find subscription by id", err)
	}
	return row.toDomain(), nil
}

func (r *PostgresSubscriptionRepository) FindByUserID(ctx context.Context, userID kernel.ID) ([]*domain.Subscription, error) {
	var rows []subscriptionRow
	if err := r.db.SelectContext(ctx, &rows, "SELECT * FROM subscriptions WHERE user_id = $1 ORDER BY created_at DESC", userID.Int64()); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find subscriptions by user", err)
	}
	subs := make([]*domain.Subscription, 0, len(rows))
	for _, row := range rows {
		subs = append(subs, row.toDomain())
	}
	return subs, nil
}

func (r *PostgresSubscriptionRepository) FindActiveByUserID(ctx context.Context, userID kernel.ID) (*domain.Subscription, error) {
	var row subscriptionRow
	err := r.db.GetContext(ctx, &row,
		`SELECT * FROM subscriptions WHERE user_id = $1 AND status IN ('active', 'trialing') ORDER BY created_at DESC LIMIT 1`,
		userID.Int64())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, kernel.NewDomainError(kernel.ErrNotFound, "no active subscription")
		}
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find active subscription", err)
	}
	return row.toDomain(), nil
}

func (r *PostgresSubscriptionRepository) FindDueForBilling(ctx context.Context, now time.Time) ([]*domain.Subscription, error) {
	var rows []subscriptionRow
	if err := r.db.SelectContext(ctx, &rows,
		`SELECT * FROM subscriptions WHERE current_period_end < $1 AND status = 'active'`,
		now); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find subscriptions due for billing", err)
	}
	subs := make([]*domain.Subscription, 0, len(rows))
	for _, row := range rows {
		subs = append(subs, row.toDomain())
	}
	return subs, nil
}

func (r *PostgresSubscriptionRepository) FindTrialsEnded(ctx context.Context, now time.Time) ([]*domain.Subscription, error) {
	var rows []subscriptionRow
	if err := r.db.SelectContext(ctx, &rows,
		`SELECT * FROM subscriptions WHERE status = 'trialing' AND trial_ends_at < $1`,
		now); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find trials ended", err)
	}
	subs := make([]*domain.Subscription, 0, len(rows))
	for _, row := range rows {
		subs = append(subs, row.toDomain())
	}
	return subs, nil
}
