package wishlist

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type WishlistItem struct {
	ProductID kernel.ID `json:"product_id"`
	AddedAt   time.Time `json:"added_at"`
}

type Wishlist struct {
	kernel.AggregateRoot
	UserID kernel.ID
	Items  []WishlistItem
}

func NewWishlist(id, userID kernel.ID) (*Wishlist, error) {
	if id == 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "wishlist id must not be empty")
	}
	if userID == 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user id must not be empty")
	}

	w := &Wishlist{
		AggregateRoot: kernel.NewAggregateRoot(id),
		UserID:        userID,
		Items:         []WishlistItem{},
	}

	w.AddEvent(&WishlistCreated{WishlistID: id, UserID: userID})
	return w, nil
}

func (w *Wishlist) AddItem(productID kernel.ID) error {
	if productID == 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "product id must not be empty")
	}

	for _, item := range w.Items {
		if item.ProductID == productID {
			return kernel.NewDomainError(kernel.ErrAlreadyExists, "product already in wishlist")
		}
	}

	w.Items = append(w.Items, WishlistItem{
		ProductID: productID,
		AddedAt:   time.Now(),
	})
	w.UpdatedAt = time.Now()
	w.AddEvent(&WishlistItemAdded{WishlistID: w.ID, UserID: w.UserID, ProductID: productID})
	return nil
}

func (w *Wishlist) RemoveItem(productID kernel.ID) error {
	if productID == 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "product id must not be empty")
	}

	for i, item := range w.Items {
		if item.ProductID == productID {
			w.Items = append(w.Items[:i], w.Items[i+1:]...)
			w.UpdatedAt = time.Now()
			w.AddEvent(&WishlistItemRemoved{WishlistID: w.ID, UserID: w.UserID, ProductID: productID})
			return nil
		}
	}

	return kernel.NewDomainError(kernel.ErrNotFound, "product not in wishlist")
}

func (w *Wishlist) Clear() {
	w.Items = []WishlistItem{}
	w.UpdatedAt = time.Now()
	w.AddEvent(&WishlistCleared{WishlistID: w.ID, UserID: w.UserID})
}

func (w *Wishlist) Contains(productID kernel.ID) bool {
	for _, item := range w.Items {
		if item.ProductID == productID {
			return true
		}
	}
	return false
}

func (w *Wishlist) ItemCount() int {
	return len(w.Items)
}

type WishlistCreated struct {
	WishlistID kernel.ID
	UserID     kernel.ID
}

func (e *WishlistCreated) EventName() string      { return "wishlist.created" }
func (e *WishlistCreated) OccurredAt() time.Time  { return time.Now() }
func (e *WishlistCreated) AggregateID() kernel.ID { return e.WishlistID }

type WishlistItemAdded struct {
	WishlistID kernel.ID
	UserID     kernel.ID
	ProductID  kernel.ID
}

func (e *WishlistItemAdded) EventName() string      { return "wishlist.item_added" }
func (e *WishlistItemAdded) OccurredAt() time.Time  { return time.Now() }
func (e *WishlistItemAdded) AggregateID() kernel.ID { return e.WishlistID }

type WishlistItemRemoved struct {
	WishlistID kernel.ID
	UserID     kernel.ID
	ProductID  kernel.ID
}

func (e *WishlistItemRemoved) EventName() string      { return "wishlist.item_removed" }
func (e *WishlistItemRemoved) OccurredAt() time.Time  { return time.Now() }
func (e *WishlistItemRemoved) AggregateID() kernel.ID { return e.WishlistID }

type WishlistCleared struct {
	WishlistID kernel.ID
	UserID     kernel.ID
}

func (e *WishlistCleared) EventName() string      { return "wishlist.cleared" }
func (e *WishlistCleared) OccurredAt() time.Time  { return time.Now() }
func (e *WishlistCleared) AggregateID() kernel.ID { return e.WishlistID }
