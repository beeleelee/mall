package wishlist

import (
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func TestNewWishlist(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()

	t.Run("valid wishlist", func(t *testing.T) {
		w, err := NewWishlist(id, 100)
		if err != nil {
			t.Fatalf("NewWishlist() error = %v", err)
		}
		if w.ID != id {
			t.Errorf("ID = %d, want %d", w.ID, id)
		}
		if w.UserID != 100 {
			t.Errorf("UserID = %d, want 100", w.UserID)
		}
		if w.ItemCount() != 0 {
			t.Errorf("expected empty wishlist, got %d items", w.ItemCount())
		}
	})

	t.Run("empty id", func(t *testing.T) {
		_, err := NewWishlist(0, 100)
		if err == nil {
			t.Fatal("expected error for empty id")
		}
	})

	t.Run("empty user id", func(t *testing.T) {
		_, err := NewWishlist(id, 0)
		if err == nil {
			t.Fatal("expected error for empty user id")
		}
	})

	t.Run("domain event emitted", func(t *testing.T) {
		id2, _ := sf.NextID()
		w, _ := NewWishlist(id2, 200)
		events := w.Events()
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if _, ok := events[0].(*WishlistCreated); !ok {
			t.Fatalf("expected WishlistCreated event, got %T", events[0])
		}
	})
}

func TestAddItem(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()
	w, _ := NewWishlist(id, 100)

	t.Run("add item", func(t *testing.T) {
		w.ClearEvents()
		err := w.AddItem(500)
		if err != nil {
			t.Fatalf("AddItem() error = %v", err)
		}
		if w.ItemCount() != 1 {
			t.Errorf("ItemCount = %d, want 1", w.ItemCount())
		}
		if !w.Contains(500) {
			t.Error("expected wishlist to contain product 500")
		}
		events := w.Events()
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if _, ok := events[0].(*WishlistItemAdded); !ok {
			t.Fatalf("expected WishlistItemAdded event")
		}
	})

	t.Run("add duplicate", func(t *testing.T) {
		err := w.AddItem(500)
		if err == nil {
			t.Fatal("expected error for duplicate item")
		}
		if !kernel.IsAlreadyExists(err) {
			t.Errorf("expected already exists error, got %v", err)
		}
	})

	t.Run("add another item", func(t *testing.T) {
		err := w.AddItem(600)
		if err != nil {
			t.Fatalf("AddItem() error = %v", err)
		}
		if w.ItemCount() != 2 {
			t.Errorf("ItemCount = %d, want 2", w.ItemCount())
		}
	})

	t.Run("empty product id", func(t *testing.T) {
		err := w.AddItem(0)
		if err == nil {
			t.Fatal("expected error for empty product id")
		}
	})
}

func TestRemoveItem(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()
	w, _ := NewWishlist(id, 100)
	_ = w.AddItem(500)
	_ = w.AddItem(600)

	t.Run("remove existing", func(t *testing.T) {
		w.ClearEvents()
		err := w.RemoveItem(500)
		if err != nil {
			t.Fatalf("RemoveItem() error = %v", err)
		}
		if w.ItemCount() != 1 {
			t.Errorf("ItemCount = %d, want 1", w.ItemCount())
		}
		if w.Contains(500) {
			t.Error("expected wishlist to not contain product 500")
		}
		events := w.Events()
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if _, ok := events[0].(*WishlistItemRemoved); !ok {
			t.Fatalf("expected WishlistItemRemoved event")
		}
	})

	t.Run("remove non-existent", func(t *testing.T) {
		err := w.RemoveItem(999)
		if err == nil {
			t.Fatal("expected error for non-existent item")
		}
		if !kernel.IsNotFound(err) {
			t.Errorf("expected not found error, got %v", err)
		}
	})

	t.Run("empty product id", func(t *testing.T) {
		err := w.RemoveItem(0)
		if err == nil {
			t.Fatal("expected error for empty product id")
		}
	})
}

func TestClear(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()
	w, _ := NewWishlist(id, 100)
	_ = w.AddItem(500)
	_ = w.AddItem(600)

	w.ClearEvents()
	w.Clear()

	if w.ItemCount() != 0 {
		t.Errorf("ItemCount = %d, want 0", w.ItemCount())
	}
	events := w.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if _, ok := events[0].(*WishlistCleared); !ok {
		t.Fatalf("expected WishlistCleared event")
	}
}

func TestContains(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()
	w, _ := NewWishlist(id, 100)
	_ = w.AddItem(500)

	if !w.Contains(500) {
		t.Error("expected Contains(500) = true")
	}
	if w.Contains(999) {
		t.Error("expected Contains(999) = false")
	}
	if w.Contains(0) {
		t.Error("expected Contains(0) = false")
	}
}
