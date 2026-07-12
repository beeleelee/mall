package inventory

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type InventoryItem struct {
	kernel.AggregateRoot
	ProductID         kernel.ID
	QuantityAvailable int
	ReservedQuantity  int
	LowStockThreshold int
}

func NewInventoryItem(id kernel.ID, productID kernel.ID, quantity int, lowStockThreshold int) (*InventoryItem, error) {
	if quantity < 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "quantity must not be negative")
	}
	if lowStockThreshold < 0 {
		lowStockThreshold = 10
	}
	if productID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "product id must be positive")
	}

	item := &InventoryItem{
		AggregateRoot:     kernel.NewAggregateRoot(id),
		ProductID:         productID,
		QuantityAvailable: quantity,
		ReservedQuantity:  0,
		LowStockThreshold: lowStockThreshold,
	}

	item.AddEvent(&StockUpdated{
		ProductID: productID,
		Quantity:  quantity,
	})

	return item, nil
}

func (i *InventoryItem) SetStock(quantity int) error {
	if quantity < 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "quantity must not be negative")
	}
	i.QuantityAvailable = quantity
	i.UpdatedAt = time.Now()
	i.AddEvent(&StockUpdated{
		ProductID: i.ProductID,
		Quantity:  quantity,
	})
	i.checkLowStock()
	return nil
}

func (i *InventoryItem) Reserve(quantity int) error {
	if quantity <= 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "reserve quantity must be positive")
	}
	available := i.QuantityAvailable - i.ReservedQuantity
	if available < quantity {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "insufficient stock")
	}
	i.ReservedQuantity += quantity
	i.UpdatedAt = time.Now()
	i.AddEvent(&StockReserved{
		ProductID: i.ProductID,
		Quantity:  quantity,
	})
	return nil
}

func (i *InventoryItem) ReleaseReservation(quantity int) error {
	if quantity <= 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "release quantity must be positive")
	}
	if i.ReservedQuantity < quantity {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot release more than reserved")
	}
	i.ReservedQuantity -= quantity
	i.UpdatedAt = time.Now()
	i.AddEvent(&StockReservationReleased{
		ProductID: i.ProductID,
		Quantity:  quantity,
	})
	return nil
}

func (i *InventoryItem) ConfirmReservation(quantity int) error {
	if quantity <= 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "confirm quantity must be positive")
	}
	if i.ReservedQuantity < quantity {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot confirm more than reserved")
	}
	i.QuantityAvailable -= quantity
	i.ReservedQuantity -= quantity
	i.UpdatedAt = time.Now()
	i.AddEvent(&StockReservationConfirmed{
		ProductID: i.ProductID,
		Quantity:  quantity,
	})
	i.checkLowStock()
	return nil
}

func (i *InventoryItem) AvailableQuantity() int {
	return i.QuantityAvailable - i.ReservedQuantity
}

func (i *InventoryItem) IsLowStock() bool {
	return i.QuantityAvailable <= i.LowStockThreshold
}

func (i *InventoryItem) IsOutOfStock() bool {
	return i.QuantityAvailable <= 0
}

func (i *InventoryItem) Restock(quantity int) error {
	if quantity <= 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "restock quantity must be positive")
	}
	i.QuantityAvailable += quantity
	i.UpdatedAt = time.Now()
	i.AddEvent(&StockRestocked{
		ProductID: i.ProductID,
		Quantity:  quantity,
	})
	return nil
}

func (i *InventoryItem) checkLowStock() {
	if i.IsLowStock() {
		i.AddEvent(&StockLow{
			ProductID: i.ProductID,
			Quantity:  i.QuantityAvailable,
		})
	}
	if i.IsOutOfStock() {
		i.AddEvent(&StockOutOfStock{
			ProductID: i.ProductID,
		})
	}
}
