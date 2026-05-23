package catalog

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type CatalogService struct {
	repo   ProductRepository
	logger kernel.Logger
}

func NewCatalogService(repo ProductRepository, logger kernel.Logger) *CatalogService {
	return &CatalogService{
		repo:   repo,
		logger: logger,
	}
}

func (s *CatalogService) Search(ctx context.Context, query string, opts SearchOptions) (*SearchResult, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 20
	}
	if opts.Status == "" {
		opts.Status = ProductStatusActive
	}

	s.logger.Debug(ctx, "catalog.search", kernel.Field("query", query), kernel.Field("opts", opts))

	result, err := s.repo.Search(ctx, query, opts)
	if err != nil {
		s.logger.Error(ctx, "catalog.search failed", err, kernel.Field("query", query))
		return nil, err
	}

	s.logger.Debug(ctx, "catalog.search completed", kernel.Field("hits", len(result.Products)))
	return result, nil
}

func (s *CatalogService) Lookup(ctx context.Context, sku SKU) (*Product, error) {
	if sku == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "sku must not be empty")
	}

	product, err := s.repo.FindBySKU(ctx, sku)
	if err != nil {
		return nil, err
	}
	return product, nil
}

func (s *CatalogService) GetProduct(ctx context.Context, id kernel.ID) (*Product, error) {
	if id <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "product id must be positive")
	}

	product, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return product, nil
}

func (s *CatalogService) CreateProduct(ctx context.Context, id kernel.ID, sku SKU, name, description, category string, price Money, attributes map[string]any) (*Product, error) {
	s.logger.Info(ctx, "catalog.create_product", kernel.Field("sku", sku), kernel.Field("id", id))

	product, err := NewProduct(id, sku, name, description, category, price, attributes)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, product); err != nil {
		return nil, err
	}

	return product, nil
}
