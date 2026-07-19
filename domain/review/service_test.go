package review

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

type fakeReviewRepo struct {
	reviews  map[kernel.ID]*Review
	byProd   map[kernel.ID][]*Review
	byUser   map[kernel.ID][]*Review
	nextID   kernel.ID
}

func newFakeRepo() *fakeReviewRepo {
	return &fakeReviewRepo{
		reviews: make(map[kernel.ID]*Review),
		byProd:  make(map[kernel.ID][]*Review),
		byUser:  make(map[kernel.ID][]*Review),
	}
}

func (f *fakeReviewRepo) Save(_ context.Context, r *Review) error {
	f.reviews[r.ID] = r
	f.byProd[r.ProductID] = append(f.byProd[r.ProductID], r)
	f.byUser[r.UserID] = append(f.byUser[r.UserID], r)
	return nil
}

func (f *fakeReviewRepo) FindByID(_ context.Context, id kernel.ID) (*Review, error) {
	r, ok := f.reviews[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "review not found")
	}
	return r, nil
}

func (f *fakeReviewRepo) FindByProduct(_ context.Context, productID kernel.ID, opts ReviewQueryOptions) (*ReviewListResult, error) {
	reviews := f.byProd[productID]
	var filtered []*Review
	for _, r := range reviews {
		if opts.Status == "" || r.Status == opts.Status {
			filtered = append(filtered, r)
		}
	}
	return &ReviewListResult{Reviews: filtered, Total: len(filtered)}, nil
}

func (f *fakeReviewRepo) FindByUser(_ context.Context, userID kernel.ID, opts ReviewQueryOptions) (*ReviewListResult, error) {
	reviews := f.byUser[userID]
	var filtered []*Review
	for _, r := range reviews {
		if opts.Status == "" || r.Status == opts.Status {
			filtered = append(filtered, r)
		}
	}
	return &ReviewListResult{Reviews: filtered, Total: len(filtered)}, nil
}

func (f *fakeReviewRepo) FindByProductAndUser(_ context.Context, productID, userID kernel.ID) (*Review, error) {
	for _, r := range f.reviews {
		if r.ProductID == productID && r.UserID == userID {
			return r, nil
		}
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "review not found")
}

func (f *fakeReviewRepo) FindAll(_ context.Context, opts ReviewQueryOptions) (*ReviewListResult, error) {
	var filtered []*Review
	for _, r := range f.reviews {
		if opts.Status == "" || r.Status == opts.Status {
			filtered = append(filtered, r)
		}
	}
	return &ReviewListResult{Reviews: filtered, Total: len(filtered)}, nil
}

func (f *fakeReviewRepo) Delete(_ context.Context, id kernel.ID) error {
	delete(f.reviews, id)
	return nil
}

func (f *fakeReviewRepo) GetAverageRating(_ context.Context, productID kernel.ID) (float64, error) {
	var sum int
	var count int
	for _, r := range f.reviews {
		if r.ProductID == productID && r.Status == ReviewStatusApproved {
			sum += int(r.Rating)
			count++
		}
	}
	if count == 0 {
		return 0, nil
	}
	return float64(sum) / float64(count), nil
}

func TestCreateReview(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	svc := NewReviewService(newFakeRepo(), &mockLogger{})

	t.Run("create review", func(t *testing.T) {
		id, _ := sf.NextID()
		rv, err := svc.CreateReview(context.Background(), id, 100, 200, 4, "Great", "Love it")
		if err != nil {
			t.Fatalf("CreateReview() error = %v", err)
		}
		if rv.Rating != 4 {
			t.Errorf("Rating = %d, want 4", rv.Rating)
		}
		if rv.Status != ReviewStatusPending {
			t.Errorf("Status = %q, want pending", rv.Status)
		}
	})

	t.Run("duplicate review", func(t *testing.T) {
		id, _ := sf.NextID()
		_, err := svc.CreateReview(context.Background(), id, 100, 200, 5, "Still great", "Still love it")
		if err == nil {
			t.Fatal("expected error for duplicate review")
		}
		if !kernel.IsAlreadyExists(err) {
			t.Errorf("expected already exists error, got %v", err)
		}
	})

	t.Run("invalid rating", func(t *testing.T) {
		id, _ := sf.NextID()
		_, err := svc.CreateReview(context.Background(), id, 200, 300, 6, "Bad", "")
		if err == nil {
			t.Fatal("expected error for invalid rating")
		}
	})
}

func TestApproveReview(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	svc := NewReviewService(newFakeRepo(), &mockLogger{})

	id, _ := sf.NextID()
	_, _ = svc.CreateReview(context.Background(), id, 100, 200, 3, "OK", "")

	t.Run("approve", func(t *testing.T) {
		rv, err := svc.ApproveReview(context.Background(), id)
		if err != nil {
			t.Fatalf("ApproveReview() error = %v", err)
		}
		if rv.Status != ReviewStatusApproved {
			t.Errorf("Status = %q, want approved", rv.Status)
		}
	})

	t.Run("approve non-existent", func(t *testing.T) {
		_, err := svc.ApproveReview(context.Background(), 99999)
		if err == nil {
			t.Fatal("expected error for non-existent review")
		}
	})
}

func TestRejectReview(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	svc := NewReviewService(newFakeRepo(), &mockLogger{})

	id, _ := sf.NextID()
	_, _ = svc.CreateReview(context.Background(), id, 100, 200, 2, "Bad", "Not good")

	t.Run("reject", func(t *testing.T) {
		rv, err := svc.RejectReview(context.Background(), id)
		if err != nil {
			t.Fatalf("RejectReview() error = %v", err)
		}
		if rv.Status != ReviewStatusRejected {
			t.Errorf("Status = %q, want rejected", rv.Status)
		}
	})
}

func TestGetAverageRating(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	svc := NewReviewService(newFakeRepo(), &mockLogger{})

	ids := make([]kernel.ID, 3)
	for i := range ids {
		id, _ := sf.NextID()
		ids[i] = id
		svc.CreateReview(context.Background(), id, 100, 200+kernel.ID(i), i+3, "", "")
		svc.ApproveReview(context.Background(), id)
	}

	avg, err := svc.GetAverageRating(context.Background(), 100)
	if err != nil {
		t.Fatalf("GetAverageRating() error = %v", err)
	}
	expected := (3.0 + 4.0 + 5.0) / 3.0
	if avg != expected {
		t.Errorf("avg = %f, want %f", avg, expected)
	}
}

func TestGetReviewsByProduct(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	svc := NewReviewService(newFakeRepo(), &mockLogger{})

	for i := 0; i < 5; i++ {
		id, _ := sf.NextID()
		svc.CreateReview(context.Background(), id, 100, 200+kernel.ID(i), 4, "Great", "")
	}

	result, err := svc.GetReviewsByProduct(context.Background(), 100, ReviewQueryOptions{Limit: 10})
	if err != nil {
		t.Fatalf("GetReviewsByProduct() error = %v", err)
	}
	if result.Total != 5 {
		t.Errorf("Total = %d, want 5", result.Total)
	}
	if len(result.Reviews) != 5 {
		t.Errorf("got %d reviews, want 5", len(result.Reviews))
	}
}

func TestDeleteReview(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	svc := NewReviewService(newFakeRepo(), &mockLogger{})

	id, _ := sf.NextID()
	svc.CreateReview(context.Background(), id, 100, 200, 4, "Great", "")

	t.Run("delete existing", func(t *testing.T) {
		err := svc.DeleteReview(context.Background(), id)
		if err != nil {
			t.Fatalf("DeleteReview() error = %v", err)
		}
	})

	t.Run("delete non-existent", func(t *testing.T) {
		err := svc.DeleteReview(context.Background(), 99999)
		if err == nil {
			t.Fatal("expected error for non-existent review")
		}
	})
}
