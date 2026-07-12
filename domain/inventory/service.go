package inventory

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type InventoryService struct {
	repo   InventoryRepository
	logger kernel.Logger
}

func NewInventoryService(repo InventoryRepository, logger kernel.Logger) *InventoryService {
	return &InventoryService{repo: repo, logger: logger}
}

func (s *InventoryService) SetStock(ctx context.Context, id kernel.ID, productID kernel.ID, quantity int, lowStockThreshold int) (*InventoryItem, error) {
	s.logger.Info(ctx, "inventory.set_stock", kernel.Field("product_id", productID.String()), kernel.Field("quantity", quantity))

	item, err := NewInventoryItem(id, productID, quantity, lowStockThreshold)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}

func (s *InventoryService) UpdateStock(ctx context.Context, productID kernel.ID, quantity int) (*InventoryItem, error) {
	s.logger.Info(ctx, "inventory.update_stock", kernel.Field("product_id", productID.String()), kernel.Field("quantity", quantity))

	item, err := s.repo.FindByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}

	if err := item.SetStock(quantity); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}

func (s *InventoryService) Reserve(ctx context.Context, productID kernel.ID, quantity int) (*InventoryItem, error) {
	s.logger.Info(ctx, "inventory.reserve", kernel.Field("product_id", productID.String()), kernel.Field("quantity", quantity))

	item, err := s.repo.FindByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}

	if err := item.Reserve(quantity); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}

func (s *InventoryService) ReleaseReservation(ctx context.Context, productID kernel.ID, quantity int) (*InventoryItem, error) {
	s.logger.Info(ctx, "inventory.release_reservation", kernel.Field("product_id", productID.String()), kernel.Field("quantity", quantity))

	item, err := s.repo.FindByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}

	if err := item.ReleaseReservation(quantity); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}

func (s *InventoryService) ConfirmReservation(ctx context.Context, productID kernel.ID, quantity int) (*InventoryItem, error) {
	s.logger.Info(ctx, "inventory.confirm_reservation", kernel.Field("product_id", productID.String()), kernel.Field("quantity", quantity))

	item, err := s.repo.FindByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}

	if err := item.ConfirmReservation(quantity); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}

func (s *InventoryService) Restock(ctx context.Context, productID kernel.ID, quantity int) (*InventoryItem, error) {
	s.logger.Info(ctx, "inventory.restock", kernel.Field("product_id", productID.String()), kernel.Field("quantity", quantity))

	item, err := s.repo.FindByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}

	if err := item.Restock(quantity); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}

func (s *InventoryService) GetStock(ctx context.Context, productID kernel.ID) (*InventoryItem, error) {
	return s.repo.FindByProductID(ctx, productID)
}

func (s *InventoryService) ListLowStock(ctx context.Context, threshold int) ([]*InventoryItem, error) {
	if threshold <= 0 {
		threshold = 10
	}
	return s.repo.FindLowStock(ctx, threshold)
}
