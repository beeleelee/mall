package wishlist

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"

	domain "github.com/beeleelee/mall/domain/wishlist"
	"github.com/beeleelee/mall/domain/kernel"
)

type wishlistRow struct {
	ID        int64           `db:"id"`
	UserID    int64           `db:"user_id"`
	Items     json.RawMessage `db:"items"`
	CreatedAt time.Time       `db:"created_at"`
	UpdatedAt time.Time       `db:"updated_at"`
}

func (r wishlistRow) toDomain() (*domain.Wishlist, error) {
	var items []domain.WishlistItem
	if len(r.Items) > 0 {
		if err := json.Unmarshal(r.Items, &items); err != nil {
			return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "unmarshal wishlist items", err)
		}
	}

	w := &domain.Wishlist{
		AggregateRoot: kernel.NewAggregateRoot(kernel.ID(r.ID)),
		UserID:        kernel.ID(r.UserID),
		Items:         items,
	}
	w.CreatedAt = r.CreatedAt
	w.UpdatedAt = r.UpdatedAt
	return w, nil
}

func fromDomain(w *domain.Wishlist) (wishlistRow, error) {
	items, err := json.Marshal(w.Items)
	if err != nil {
		return wishlistRow{}, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal wishlist items", err)
	}
	return wishlistRow{
		ID:        w.ID.Int64(),
		UserID:    w.UserID.Int64(),
		Items:     items,
		CreatedAt: w.CreatedAt,
		UpdatedAt: w.UpdatedAt,
	}, nil
}

type PostgresWishlistRepository struct {
	db *sqlx.DB
}

func NewPostgresWishlistRepository(db *sqlx.DB) *PostgresWishlistRepository {
	return &PostgresWishlistRepository{db: db}
}

func (r *PostgresWishlistRepository) Save(ctx context.Context, wishlist *domain.Wishlist) error {
	row, err := fromDomain(wishlist)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO wishlists (id, user_id, items, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			items = EXCLUDED.items,
			updated_at = NOW()
	`, row.ID, row.UserID, row.Items, row.CreatedAt, row.UpdatedAt)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save wishlist", err)
	}
	return nil
}

func (r *PostgresWishlistRepository) FindByUserID(ctx context.Context, userID kernel.ID) (*domain.Wishlist, error) {
	var row wishlistRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM wishlists WHERE user_id = $1`, userID.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "wishlist not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find wishlist by user id", err)
	}
	return row.toDomain()
}

func (r *PostgresWishlistRepository) Delete(ctx context.Context, id kernel.ID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM wishlists WHERE id = $1`, id.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "delete wishlist", err)
	}
	return nil
}
