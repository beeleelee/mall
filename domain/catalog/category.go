package catalog

import (
	"context"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type Category struct {
	kernel.Entity
	Name      string
	Slug      string
	ParentID  kernel.ID
	SortOrder int
}

func NewCategory(id kernel.ID, name, slug string, parentID kernel.ID, sortOrder int) (*Category, error) {
	if name == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "category name must not be empty")
	}
	if slug == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "category slug must not be empty")
	}

	c := &Category{
		Entity:    kernel.NewEntity(id),
		Name:      name,
		Slug:      slug,
		ParentID:  parentID,
		SortOrder: sortOrder,
	}

	return c, nil
}

func (c *Category) Update(name, slug string, parentID kernel.ID, sortOrder int) error {
	if name == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "category name must not be empty")
	}
	if slug == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "category slug must not be empty")
	}
	c.Name = name
	c.Slug = slug
	c.ParentID = parentID
	c.SortOrder = sortOrder
	c.UpdatedAt = time.Now()
	return nil
}

type CategoryRepository interface {
	Save(ctx context.Context, category *Category) error
	FindByID(ctx context.Context, id kernel.ID) (*Category, error)
	FindBySlug(ctx context.Context, slug string) (*Category, error)
	FindAll(ctx context.Context) ([]*Category, error)
	FindChildren(ctx context.Context, parentID kernel.ID) ([]*Category, error)
	Delete(ctx context.Context, id kernel.ID) error
}
