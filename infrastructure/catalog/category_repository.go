package catalog

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"

	domain "github.com/beeleelee/mall/domain/catalog"
	"github.com/beeleelee/mall/domain/kernel"
)

type categoryRow struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	Slug      string    `db:"slug"`
	ParentID  *int64    `db:"parent_id"`
	SortOrder int       `db:"sort_order"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (r categoryRow) toDomain() *domain.Category {
	parentID := kernel.ID(0)
	if r.ParentID != nil {
		parentID = kernel.ID(*r.ParentID)
	}

	c := &domain.Category{
		Entity:    kernel.NewEntity(kernel.ID(r.ID)),
		Name:      r.Name,
		Slug:      r.Slug,
		ParentID:  parentID,
		SortOrder: r.SortOrder,
	}
	c.CreatedAt = r.CreatedAt
	c.UpdatedAt = r.UpdatedAt
	return c
}

func fromCategory(c *domain.Category) categoryRow {
	var parentID *int64
	if c.ParentID > 0 {
		v := c.ParentID.Int64()
		parentID = &v
	}

	return categoryRow{
		ID:        c.ID.Int64(),
		Name:      c.Name,
		Slug:      c.Slug,
		ParentID:  parentID,
		SortOrder: c.SortOrder,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

type PostgresCategoryRepository struct {
	db *sqlx.DB
}

func NewPostgresCategoryRepository(db *sqlx.DB) *PostgresCategoryRepository {
	return &PostgresCategoryRepository{db: db}
}

func (r *PostgresCategoryRepository) Save(ctx context.Context, category *domain.Category) error {
	row := fromCategory(category)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO categories (id, name, slug, parent_id, sort_order, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			slug = EXCLUDED.slug,
			parent_id = EXCLUDED.parent_id,
			sort_order = EXCLUDED.sort_order,
			updated_at = EXCLUDED.updated_at
	`, row.ID, row.Name, row.Slug, row.ParentID, row.SortOrder, row.CreatedAt, row.UpdatedAt)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save category", err)
	}
	return nil
}

func (r *PostgresCategoryRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.Category, error) {
	var row categoryRow
	err := r.db.GetContext(ctx, &row, "SELECT * FROM categories WHERE id = $1", id.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "category not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find category by id", err)
	}
	return row.toDomain(), nil
}

func (r *PostgresCategoryRepository) FindBySlug(ctx context.Context, slug string) (*domain.Category, error) {
	var row categoryRow
	err := r.db.GetContext(ctx, &row, "SELECT * FROM categories WHERE slug = $1", slug)
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "category not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find category by slug", err)
	}
	return row.toDomain(), nil
}

func (r *PostgresCategoryRepository) FindAll(ctx context.Context) ([]*domain.Category, error) {
	var rows []categoryRow
	err := r.db.SelectContext(ctx, &rows, "SELECT * FROM categories ORDER BY sort_order, name")
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "list categories", err)
	}

	categories := make([]*domain.Category, len(rows))
	for i, row := range rows {
		categories[i] = row.toDomain()
	}
	return categories, nil
}

func (r *PostgresCategoryRepository) FindChildren(ctx context.Context, parentID kernel.ID) ([]*domain.Category, error) {
	var rows []categoryRow
	err := r.db.SelectContext(ctx, &rows, "SELECT * FROM categories WHERE parent_id = $1 ORDER BY sort_order, name", parentID.Int64())
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find children categories", err)
	}

	categories := make([]*domain.Category, len(rows))
	for i, row := range rows {
		categories[i] = row.toDomain()
	}
	return categories, nil
}

func (r *PostgresCategoryRepository) Delete(ctx context.Context, id kernel.ID) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM categories WHERE id = $1", id.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "delete category", err)
	}
	return nil
}
