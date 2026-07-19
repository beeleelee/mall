package wishlist

import (
	"context"
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

type mockLogger struct{}

func (m *mockLogger) Debug(_ context.Context, _ string, _ ...kernel.LogField) {}
func (m *mockLogger) Info(_ context.Context, _ string, _ ...kernel.LogField)  {}
func (m *mockLogger) Warn(_ context.Context, _ string, _ ...kernel.LogField)  {}
func (m *mockLogger) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}

type fakeWishlistRepo struct {
	wishlists map[kernel.ID]*Wishlist
	byUser    map[kernel.ID]*Wishlist
}

func newFakeRepo() *fakeWishlistRepo {
	return &fakeWishlistRepo{
		wishlists: make(map[kernel.ID]*Wishlist),
		byUser:    make(map[kernel.ID]*Wishlist),
	}
}

func (f *fakeWishlistRepo) Save(_ context.Context, w *Wishlist) error {
	f.wishlists[w.ID] = w
	f.byUser[w.UserID] = w
	return nil
}

func (f *fakeWishlistRepo) FindByUserID(_ context.Context, userID kernel.ID) (*Wishlist, error) {
	w, ok := f.byUser[userID]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "wishlist not found")
	}
	return w, nil
}

func (f *fakeWishlistRepo) Delete(_ context.Context, id kernel.ID) error {
	delete(f.wishlists, id)
	return nil
}

func TestServiceAddItem(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	svc := NewWishlistService(newFakeRepo(), sf, &mockLogger{})

	t.Run("add item creates wishlist", func(t *testing.T) {
		err := svc.AddItem(context.Background(), 100, 500)
		if err != nil {
			t.Fatalf("AddItem() error = %v", err)
		}

		w, err := svc.GetWishlist(context.Background(), 100)
		if err != nil {
			t.Fatalf("GetWishlist() error = %v", err)
		}
		if w.ItemCount() != 1 {
			t.Errorf("ItemCount = %d, want 1", w.ItemCount())
		}
		if !w.Contains(500) {
			t.Error("expected wishlist to contain product 500")
		}
	})

	t.Run("add duplicate item", func(t *testing.T) {
		err := svc.AddItem(context.Background(), 100, 500)
		if err == nil {
			t.Fatal("expected error for duplicate item")
		}
	})

	t.Run("add different item", func(t *testing.T) {
		err := svc.AddItem(context.Background(), 100, 600)
		if err != nil {
			t.Fatalf("AddItem() error = %v", err)
		}
		w, _ := svc.GetWishlist(context.Background(), 100)
		if w.ItemCount() != 2 {
			t.Errorf("ItemCount = %d, want 2", w.ItemCount())
		}
	})
}

func TestServiceRemoveItem(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	svc := NewWishlistService(newFakeRepo(), sf, &mockLogger{})

	_ = svc.AddItem(context.Background(), 200, 500)

	t.Run("remove existing", func(t *testing.T) {
		err := svc.RemoveItem(context.Background(), 200, 500)
		if err != nil {
			t.Fatalf("RemoveItem() error = %v", err)
		}
		w, _ := svc.GetWishlist(context.Background(), 200)
		if w.ItemCount() != 0 {
			t.Errorf("ItemCount = %d, want 0", w.ItemCount())
		}
	})

	t.Run("remove non-existent", func(t *testing.T) {
		err := svc.RemoveItem(context.Background(), 200, 999)
		if err == nil {
			t.Fatal("expected error for non-existent item")
		}
	})

	t.Run("remove from non-existent wishlist", func(t *testing.T) {
		err := svc.RemoveItem(context.Background(), 999, 500)
		if err == nil {
			t.Fatal("expected error for non-existent wishlist")
		}
	})
}

func TestServiceGetWishlist(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	svc := NewWishlistService(newFakeRepo(), sf, &mockLogger{})

	t.Run("get non-existent returns empty", func(t *testing.T) {
		w, err := svc.GetWishlist(context.Background(), 300)
		if err != nil {
			t.Fatalf("GetWishlist() error = %v", err)
		}
		if w.ItemCount() != 0 {
			t.Errorf("expected empty wishlist, got %d items", w.ItemCount())
		}
	})

	t.Run("get existing wishlist", func(t *testing.T) {
		_ = svc.AddItem(context.Background(), 300, 500)
		w, err := svc.GetWishlist(context.Background(), 300)
		if err != nil {
			t.Fatalf("GetWishlist() error = %v", err)
		}
		if w.ItemCount() != 1 {
			t.Errorf("ItemCount = %d, want 1", w.ItemCount())
		}
	})
}

func TestServiceClear(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	svc := NewWishlistService(newFakeRepo(), sf, &mockLogger{})

	_ = svc.AddItem(context.Background(), 400, 500)
	_ = svc.AddItem(context.Background(), 400, 600)

	t.Run("clear existing wishlist", func(t *testing.T) {
		err := svc.ClearWishlist(context.Background(), 400)
		if err != nil {
			t.Fatalf("ClearWishlist() error = %v", err)
		}
		w, _ := svc.GetWishlist(context.Background(), 400)
		if w.ItemCount() != 0 {
			t.Errorf("ItemCount = %d, want 0", w.ItemCount())
		}
	})

	t.Run("clear non-existent", func(t *testing.T) {
		err := svc.ClearWishlist(context.Background(), 999)
		if err == nil {
			t.Fatal("expected error for non-existent wishlist")
		}
	})
}

func TestServiceIsInWishlist(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	svc := NewWishlistService(newFakeRepo(), sf, &mockLogger{})

	_ = svc.AddItem(context.Background(), 500, 777)

	t.Run("product in wishlist", func(t *testing.T) {
		found, err := svc.IsInWishlist(context.Background(), 500, 777)
		if err != nil {
			t.Fatalf("IsInWishlist() error = %v", err)
		}
		if !found {
			t.Error("expected product to be in wishlist")
		}
	})

	t.Run("product not in wishlist", func(t *testing.T) {
		found, err := svc.IsInWishlist(context.Background(), 500, 999)
		if err != nil {
			t.Fatalf("IsInWishlist() error = %v", err)
		}
		if found {
			t.Error("expected product not to be in wishlist")
		}
	})

	t.Run("user without wishlist", func(t *testing.T) {
		found, err := svc.IsInWishlist(context.Background(), 999, 777)
		if err != nil {
			t.Fatalf("IsInWishlist() error = %v", err)
		}
		if found {
			t.Error("expected false for user without wishlist")
		}
	})
}
