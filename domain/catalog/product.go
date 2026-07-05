package catalog

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type SKU string

type Money struct {
	Amount   int64
	Currency string
}

type ProductStatus string

const (
	ProductStatusActive       ProductStatus = "active"
	ProductStatusInactive     ProductStatus = "inactive"
	ProductStatusDraft        ProductStatus = "draft"
	ProductStatusDiscontinued ProductStatus = "discontinued"
)

type Product struct {
	kernel.AggregateRoot
	SKU         SKU
	Name        string
	Description string
	Category    string
	CategoryID  kernel.ID
	Price       Money
	Status      ProductStatus
	Attributes  map[string]any
}

func NewProduct(id kernel.ID, sku SKU, name, description, category string, categoryID kernel.ID, price Money, attributes map[string]any) (*Product, error) {
	if sku == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "sku must not be empty")
	}
	if name == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "product name must not be empty")
	}
	if price.Amount < 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "price must not be negative")
	}
	if price.Currency == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "currency must not be empty")
	}
	if attributes == nil {
		attributes = make(map[string]any)
	}

	p := &Product{
		AggregateRoot: kernel.NewAggregateRoot(id),
		SKU:           sku,
		Name:          name,
		Description:   description,
		Category:      category,
		CategoryID:    categoryID,
		Price:         price,
		Status:        ProductStatusActive,
		Attributes:    attributes,
	}

	p.AddEvent(&ProductCreated{
		ProductID: id,
		SKU:       sku,
	})

	return p, nil
}

func (p *Product) ChangePrice(price Money) error {
	if price.Amount < 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "price must not be negative")
	}
	if price.Currency == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "currency must not be empty")
	}
	old := p.Price
	p.Price = price
	p.UpdatedAt = time.Now()
	p.AddEvent(&ProductPriceChanged{
		ProductID: p.ID,
		OldPrice:  old,
		NewPrice:  price,
	})
	return nil
}

func (p *Product) ChangeStatus(status ProductStatus) error {
	if status == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "status must not be empty")
	}
	old := p.Status
	p.Status = status
	p.UpdatedAt = time.Now()
	p.AddEvent(&ProductStatusChanged{
		ProductID: p.ID,
		OldStatus: old,
		NewStatus: status,
	})
	return nil
}

func (p *Product) UpdateDetails(name, description, category string, categoryID kernel.ID) error {
	if name == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "product name must not be empty")
	}
	p.Name = name
	p.Description = description
	p.Category = category
	p.CategoryID = categoryID
	p.UpdatedAt = time.Now()
	p.AddEvent(&ProductUpdated{
		ProductID: p.ID,
	})
	return nil
}

type ProductCreated struct {
	ProductID kernel.ID
	SKU       SKU
}

func (e *ProductCreated) EventName() string      { return "catalog.product.created" }
func (e *ProductCreated) OccurredAt() time.Time  { return time.Now() }
func (e *ProductCreated) AggregateID() kernel.ID { return e.ProductID }

type ProductPriceChanged struct {
	ProductID kernel.ID
	OldPrice  Money
	NewPrice  Money
}

func (e *ProductPriceChanged) EventName() string      { return "catalog.product.price_changed" }
func (e *ProductPriceChanged) OccurredAt() time.Time  { return time.Now() }
func (e *ProductPriceChanged) AggregateID() kernel.ID { return e.ProductID }

type ProductStatusChanged struct {
	ProductID kernel.ID
	OldStatus ProductStatus
	NewStatus ProductStatus
}

func (e *ProductStatusChanged) EventName() string      { return "catalog.product.status_changed" }
func (e *ProductStatusChanged) OccurredAt() time.Time  { return time.Now() }
func (e *ProductStatusChanged) AggregateID() kernel.ID { return e.ProductID }

type ProductUpdated struct {
	ProductID kernel.ID
}

func (e *ProductUpdated) EventName() string      { return "catalog.product.updated" }
func (e *ProductUpdated) OccurredAt() time.Time  { return time.Now() }
func (e *ProductUpdated) AggregateID() kernel.ID { return e.ProductID }
