package discount

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type DiscountType string

const (
	DiscountTypeFlat       DiscountType = "flat"
	DiscountTypePercentage DiscountType = "percentage"
)

type DiscountCode struct {
	kernel.AggregateRoot
	Code        string
	Type        DiscountType
	Value       int64
	MinPurchase int64
	MaxUsages   int
	UsedCount   int
	Expiry      time.Time
	Active      bool
	Stackable   bool
}

func NewDiscountCode(id kernel.ID, code string, discountType DiscountType, value, minPurchase int64, maxUsages int, expiry time.Time, stackable bool) (*DiscountCode, error) {
	if code == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "code must not be empty")
	}
	if value <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "value must be positive")
	}
	if discountType == DiscountTypePercentage && value > 100 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "percentage value must be <= 100")
	}

	return &DiscountCode{
		AggregateRoot: kernel.NewAggregateRoot(id),
		Code:          code,
		Type:          discountType,
		Value:         value,
		MinPurchase:   minPurchase,
		MaxUsages:     maxUsages,
		Expiry:        expiry,
		Active:        true,
		Stackable:     stackable,
	}, nil
}

func (d *DiscountCode) IsValid(subtotal int64) bool {
	if !d.Active {
		return false
	}
	if d.MaxUsages > 0 && d.UsedCount >= d.MaxUsages {
		return false
	}
	if !d.Expiry.IsZero() && time.Now().After(d.Expiry) {
		return false
	}
	if subtotal < d.MinPurchase {
		return false
	}
	return true
}

func (d *DiscountCode) Apply(subtotal int64) int64 {
	if !d.IsValid(subtotal) {
		return subtotal
	}

	switch d.Type {
	case DiscountTypeFlat:
		if d.Value > subtotal {
			return 0
		}
		return subtotal - d.Value
	case DiscountTypePercentage:
		discount := subtotal * d.Value / 100
		return subtotal - discount
	default:
		return subtotal
	}
}

func (d *DiscountCode) Use() {
	d.UsedCount++
}

func (d *DiscountCode) Deactivate() {
	d.Active = false
}
