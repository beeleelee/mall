package review

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/review"
)

type reviewRow struct {
	ID        int64     `db:"id"`
	ProductID int64     `db:"product_id"`
	UserID    int64     `db:"user_id"`
	Rating    int       `db:"rating"`
	Title     string    `db:"title"`
	Content   string    `db:"content"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (r reviewRow) toDomain() *domain.Review {
	rating, _ := domain.NewRating(r.Rating)
	return &domain.Review{
		AggregateRoot: kernel.NewAggregateRoot(kernel.ID(r.ID)),
		ProductID:     kernel.ID(r.ProductID),
		UserID:        kernel.ID(r.UserID),
		Rating:        rating,
		Title:         r.Title,
		Content:       r.Content,
		Status:        domain.ReviewStatus(r.Status),
	}
}

func fromDomain(d *domain.Review) reviewRow {
	return reviewRow{
		ID:        d.ID.Int64(),
		ProductID: d.ProductID.Int64(),
		UserID:    d.UserID.Int64(),
		Rating:    int(d.Rating),
		Title:     d.Title,
		Content:   d.Content,
		Status:    string(d.Status),
	}
}

type PostgresReviewRepository struct {
	db *sqlx.DB
}

func NewPostgresReviewRepository(db *sqlx.DB) *PostgresReviewRepository {
	return &PostgresReviewRepository{db: db}
}

func (r *PostgresReviewRepository) Save(ctx context.Context, review *domain.Review) error {
	row := fromDomain(review)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO reviews (id, product_id, user_id, rating, title, content, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			rating = EXCLUDED.rating,
			title = EXCLUDED.title,
			content = EXCLUDED.content,
			status = EXCLUDED.status,
			updated_at = NOW()
	`, row.ID, row.ProductID, row.UserID, row.Rating, row.Title, row.Content, row.Status)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save review", err)
	}
	return nil
}

func (r *PostgresReviewRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.Review, error) {
	var row reviewRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM reviews WHERE id = $1`, id.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "review not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find review by id", err)
	}
	return row.toDomain(), nil
}

func (r *PostgresReviewRepository) FindByProduct(ctx context.Context, productID kernel.ID, opts domain.ReviewQueryOptions) (*domain.ReviewListResult, error) {
	where := "WHERE product_id = $1"
	args := []any{productID.Int64()}
	if opts.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", len(args)+1)
		args = append(args, string(opts.Status))
	}

	return r.queryList(ctx, where, args, opts)
}

func (r *PostgresReviewRepository) FindByUser(ctx context.Context, userID kernel.ID, opts domain.ReviewQueryOptions) (*domain.ReviewListResult, error) {
	where := "WHERE user_id = $1"
	args := []any{userID.Int64()}
	if opts.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", len(args)+1)
		args = append(args, string(opts.Status))
	}

	return r.queryList(ctx, where, args, opts)
}

func (r *PostgresReviewRepository) FindByProductAndUser(ctx context.Context, productID, userID kernel.ID) (*domain.Review, error) {
	var row reviewRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM reviews WHERE product_id = $1 AND user_id = $2`, productID.Int64(), userID.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "review not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find review by product and user", err)
	}
	return row.toDomain(), nil
}

func (r *PostgresReviewRepository) FindAll(ctx context.Context, opts domain.ReviewQueryOptions) (*domain.ReviewListResult, error) {
	where := "WHERE 1=1"
	args := []any{}
	if opts.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", len(args)+1)
		args = append(args, string(opts.Status))
	}

	return r.queryList(ctx, where, args, opts)
}

func (r *PostgresReviewRepository) queryList(ctx context.Context, whereClause string, args []any, opts domain.ReviewQueryOptions) (*domain.ReviewListResult, error) {
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM reviews %s", whereClause)
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "count reviews", err)
	}

	query := fmt.Sprintf("SELECT * FROM reviews %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		whereClause, len(args)+1, len(args)+2)
	queryArgs := append(args, opts.Limit, opts.Offset)

	var rows []reviewRow
	if err := r.db.SelectContext(ctx, &rows, query, queryArgs...); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "list reviews", err)
	}

	reviews := make([]*domain.Review, len(rows))
	for i, row := range rows {
		reviews[i] = row.toDomain()
	}

	return &domain.ReviewListResult{Reviews: reviews, Total: total}, nil
}

func (r *PostgresReviewRepository) Delete(ctx context.Context, id kernel.ID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM reviews WHERE id = $1`, id.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "delete review", err)
	}
	return nil
}

func (r *PostgresReviewRepository) GetAverageRating(ctx context.Context, productID kernel.ID) (float64, error) {
	var avg sql.NullFloat64
	err := r.db.GetContext(ctx, &avg, `SELECT AVG(rating::float) FROM reviews WHERE product_id = $1 AND status = $2`,
		productID.Int64(), string(domain.ReviewStatusApproved))
	if err != nil {
		return 0, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "get average rating", err)
	}
	if avg.Valid {
		return avg.Float64, nil
	}
	return 0, nil
}
