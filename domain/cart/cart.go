package cart

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type CartStatus string

const (
	CartStatusActive    CartStatus = "active"
	CartStatusMerged    CartStatus = "merged"
	CartStatusAbandoned CartStatus = "abandoned"
)

type CartItem struct {
	ProductID kernel.ID `json:"product_id"`
	SKU       string    `json:"sku"`
	Name      string    `json:"name"`
	Quantity  int       `json:"quantity"`
	UnitPrice int64     `json:"unit_price"`
	ImageURL  string    `json:"image_url,omitempty"`
}

func (i CartItem) TotalPrice() int64 {
	return i.UnitPrice * int64(i.Quantity)
}

type CartTotal struct {
	Subtotal  int64 `json:"subtotal"`
	ItemCount int   `json:"item_count"`
}

type Cart struct {
	kernel.AggregateRoot
	UserID kernel.ID
	Items  []CartItem
	Status CartStatus
}

func NewCart(id, userID kernel.ID) (*Cart, error) {
	if userID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}
	c := &Cart{
		AggregateRoot: kernel.NewAggregateRoot(id),
		UserID:        userID,
		Items:         []CartItem{},
		Status:        CartStatusActive,
	}
	c.AddEvent(CartCreatedEvent{CartID: id, UserID: userID})
	return c, nil
}

func NewCartFromSnapshot(id, userID kernel.ID, items []CartItem, status CartStatus, createdAt, updatedAt time.Time) *Cart {
	c := &Cart{
		AggregateRoot: kernel.NewAggregateRoot(id),
		UserID:        userID,
		Items:         items,
		Status:        status,
	}
	c.CreatedAt = createdAt
	c.UpdatedAt = updatedAt
	return c
}

func (c *Cart) AddItem(item CartItem) error {
	if item.ProductID <= 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "product_id must be positive")
	}
	if item.Quantity <= 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "quantity must be positive")
	}
	if item.UnitPrice < 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "unit_price must not be negative")
	}

	for i, existing := range c.Items {
		if existing.ProductID == item.ProductID {
			c.Items[i].Quantity += item.Quantity
			c.Items[i].Name = item.Name
			c.Items[i].UnitPrice = item.UnitPrice
			c.Items[i].ImageURL = item.ImageURL
			c.touch()
			c.AddEvent(CartUpdatedEvent{CartID: c.ID, UserID: c.UserID})
			return nil
		}
	}

	c.Items = append(c.Items, item)
	c.touch()
	c.AddEvent(CartUpdatedEvent{CartID: c.ID, UserID: c.UserID})
	return nil
}

func (c *Cart) UpdateQuantity(productID kernel.ID, quantity int) error {
	if productID <= 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "product_id must be positive")
	}
	if quantity < 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "quantity must not be negative")
	}

	if quantity == 0 {
		return c.RemoveItem(productID)
	}

	for i, item := range c.Items {
		if item.ProductID == productID {
			c.Items[i].Quantity = quantity
			c.touch()
			c.AddEvent(CartUpdatedEvent{CartID: c.ID, UserID: c.UserID})
			return nil
		}
	}

	return kernel.NewDomainError(kernel.ErrNotFound, "item not found in cart")
}

func (c *Cart) RemoveItem(productID kernel.ID) error {
	if productID <= 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "product_id must be positive")
	}

	for i, item := range c.Items {
		if item.ProductID == productID {
			c.Items = append(c.Items[:i], c.Items[i+1:]...)
			c.touch()
			c.AddEvent(CartUpdatedEvent{CartID: c.ID, UserID: c.UserID})
			return nil
		}
	}

	return kernel.NewDomainError(kernel.ErrNotFound, "item not found in cart")
}

func (c *Cart) Clear() {
	c.Items = []CartItem{}
	c.touch()
	c.AddEvent(CartClearedEvent{CartID: c.ID, UserID: c.UserID})
}

func (c *Cart) GetTotal() CartTotal {
	var subtotal int64
	for _, item := range c.Items {
		subtotal += item.TotalPrice()
	}
	return CartTotal{
		Subtotal:  subtotal,
		ItemCount: len(c.Items),
	}
}

func (c *Cart) IsEmpty() bool {
	return len(c.Items) == 0
}

func (c *Cart) Merge(other *Cart) error {
	if c.UserID != other.UserID {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot merge carts from different users")
	}
	for _, item := range other.Items {
		if err := c.AddItem(item); err != nil {
			return err
		}
	}
	other.Status = CartStatusMerged
	other.AddEvent(CartMergedEvent{CartID: other.ID, UserID: other.UserID})
	return nil
}

func (c *Cart) touch() {
	c.UpdatedAt = time.Now()
}
