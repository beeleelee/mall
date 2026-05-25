package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	domain "github.com/beeleelee/mall/domain/identity"
	"github.com/beeleelee/mall/domain/kernel"
)

type userRow struct {
	ID           int64           `db:"id"`
	Email        string          `db:"email"`
	Name         string          `db:"name"`
	PasswordHash string          `db:"password_hash"`
	Status       string          `db:"status"`
	Roles        json.RawMessage `db:"roles"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`
}

func (r userRow) toDomain() (*domain.User, error) {
	var roles []domain.UserRole
	if len(r.Roles) > 0 {
		var strs []string
		if err := json.Unmarshal(r.Roles, &strs); err != nil {
			return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "unmarshal roles", err)
		}
		roles = make([]domain.UserRole, len(strs))
		for i, s := range strs {
			roles[i] = domain.UserRole(s)
		}
	}

	u := &domain.User{
		AggregateRoot: kernel.NewAggregateRoot(kernel.ID(r.ID)),
		Email:         r.Email,
		Name:          r.Name,
		Status:        domain.UserStatus(r.Status),
		Roles:         roles,
	}
	u.CreatedAt = r.CreatedAt
	u.UpdatedAt = r.UpdatedAt
	u.SetPasswordHash(r.PasswordHash)

	return u, nil
}

func fromDomain(u *domain.User) userRow {
	roles := make([]string, len(u.Roles))
	for i, r := range u.Roles {
		roles[i] = string(r)
	}
	rolesJSON, _ := json.Marshal(roles)

	return userRow{
		ID:           u.ID.Int64(),
		Email:        u.Email,
		Name:         u.Name,
		PasswordHash: u.PasswordHash(),
		Status:       string(u.Status),
		Roles:        rolesJSON,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

type PostgresUserRepository struct {
	db    *sqlx.DB
	redis *redis.Client
	ttl   time.Duration
}

func NewPostgresUserRepository(db *sqlx.DB, redis *redis.Client) *PostgresUserRepository {
	return &PostgresUserRepository{
		db:    db,
		redis: redis,
		ttl:   15 * time.Minute,
	}
}

func (r *PostgresUserRepository) Save(ctx context.Context, user *domain.User) error {
	row := fromDomain(user)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, email, name, password_hash, status, roles, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			email = EXCLUDED.email,
			name = EXCLUDED.name,
			password_hash = EXCLUDED.password_hash,
			status = EXCLUDED.status,
			roles = EXCLUDED.roles,
			updated_at = EXCLUDED.updated_at
	`, row.ID, row.Email, row.Name, row.PasswordHash,
		row.Status, row.Roles, row.CreatedAt, row.UpdatedAt)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save user", err)
	}

	r.invalidateCache(ctx, user.ID, user.Email)

	return nil
}

func (r *PostgresUserRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.User, error) {
	cacheKey := r.idCacheKey(id)
	if user, err := r.readCache(ctx, cacheKey); err == nil && user != nil {
		return user, nil
	}

	return r.querySingle(ctx, "SELECT * FROM users WHERE id = $1", id.Int64())
}

func (r *PostgresUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	cacheKey := r.emailCacheKey(email)
	if user, err := r.readCache(ctx, cacheKey); err == nil && user != nil {
		return user, nil
	}

	return r.querySingle(ctx, "SELECT * FROM users WHERE email = $1", email)
}

func (r *PostgresUserRepository) Delete(ctx context.Context, id kernel.ID) error {
	email, err := r.getEmailByID(ctx, id)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "delete user", err)
	}

	r.invalidateCache(ctx, id, email)

	return nil
}

func (r *PostgresUserRepository) getEmailByID(ctx context.Context, id kernel.ID) (string, error) {
	var email string
	err := r.db.GetContext(ctx, &email, "SELECT email FROM users WHERE id = $1", id.Int64())
	if err == sql.ErrNoRows {
		return "", kernel.NewDomainError(kernel.ErrNotFound, "user not found")
	}
	if err != nil {
		return "", kernel.NewDomainErrorWithCause(kernel.ErrInternal, "get user email", err)
	}
	return email, nil
}

func (r *PostgresUserRepository) querySingle(ctx context.Context, query string, args ...any) (*domain.User, error) {
	row, err := r.scanSingle(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	user, err := row.toDomain()
	if err != nil {
		return nil, err
	}

	r.writeCache(ctx, r.idCacheKey(user.ID), user)
	r.writeCache(ctx, r.emailCacheKey(user.Email), user)

	return user, nil
}

func (r *PostgresUserRepository) scanSingle(ctx context.Context, query string, args ...any) (userRow, error) {
	var row userRow
	err := r.db.GetContext(ctx, &row, query, args...)
	if err == sql.ErrNoRows {
		return row, kernel.NewDomainError(kernel.ErrNotFound, "user not found")
	}
	if err != nil {
		return row, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "query user", err)
	}
	return row, nil
}

func (r *PostgresUserRepository) idCacheKey(id kernel.ID) string {
	return fmt.Sprintf("identity:user:id:%d", id.Int64())
}

func (r *PostgresUserRepository) emailCacheKey(email string) string {
	return fmt.Sprintf("identity:user:email:%s", email)
}

func (r *PostgresUserRepository) readCache(ctx context.Context, key string) (*domain.User, error) {
	data, err := r.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var row userRow
	if err := json.Unmarshal(data, &row); err != nil {
		return nil, err
	}

	return row.toDomain()
}

func (r *PostgresUserRepository) writeCache(ctx context.Context, key string, user *domain.User) {
	row := fromDomain(user)
	data, err := json.Marshal(row)
	if err != nil {
		return
	}
	r.redis.Set(ctx, key, data, r.ttl)
}

func (r *PostgresUserRepository) invalidateCache(ctx context.Context, id kernel.ID, email string) {
	r.redis.Del(ctx, r.idCacheKey(id))
	if email != "" {
		r.redis.Del(ctx, r.emailCacheKey(email))
	}
}
