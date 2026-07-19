package review

import (
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func TestNewRating(t *testing.T) {
	tests := []struct {
		name    string
		input   int
		wantErr bool
	}{
		{"valid rating 1", 1, false},
		{"valid rating 3", 3, false},
		{"valid rating 5", 5, false},
		{"too low", 0, true},
		{"too high", 6, true},
		{"negative", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRating(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRating(%d) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestNewReview(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()

	t.Run("valid review", func(t *testing.T) {
		rating, _ := NewRating(4)
		rv, err := NewReview(id, 100, 200, rating, "Great product", "Really love this item")

		if err != nil {
			t.Fatalf("NewReview() error = %v", err)
		}
		if rv.ID != id {
			t.Errorf("ID = %d, want %d", rv.ID, id)
		}
		if rv.ProductID != 100 {
			t.Errorf("ProductID = %d, want 100", rv.ProductID)
		}
		if rv.UserID != 200 {
			t.Errorf("UserID = %d, want 200", rv.UserID)
		}
		if rv.Rating != rating {
			t.Errorf("Rating = %d, want %d", rv.Rating, rating)
		}
		if rv.Title != "Great product" {
			t.Errorf("Title = %q, want %q", rv.Title, "Great product")
		}
		if rv.Content != "Really love this item" {
			t.Errorf("Content = %q, want %q", rv.Content, "Really love this item")
		}
		if rv.Status != ReviewStatusPending {
			t.Errorf("Status = %q, want %q", rv.Status, ReviewStatusPending)
		}
	})

	t.Run("empty id", func(t *testing.T) {
		rating, _ := NewRating(3)
		_, err := NewReview(0, 100, 200, rating, "title", "content")
		if err == nil {
			t.Fatal("expected error for empty id")
		}
		if !kernel.IsInvalidArgument(err) {
			t.Errorf("expected invalid argument error, got %v", err)
		}
	})

	t.Run("empty product id", func(t *testing.T) {
		rating, _ := NewRating(3)
		_, err := NewReview(id, 0, 200, rating, "title", "content")
		if err == nil {
			t.Fatal("expected error for empty product id")
		}
	})

	t.Run("empty user id", func(t *testing.T) {
		rating, _ := NewRating(3)
		_, err := NewReview(id, 100, 0, rating, "title", "content")
		if err == nil {
			t.Fatal("expected error for empty user id")
		}
	})

	t.Run("domain event emitted", func(t *testing.T) {
		rating, _ := NewRating(5)
		rv, _ := NewReview(id, 100, 200, rating, "title", "content")
		events := rv.Events()
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		evt, ok := events[0].(*ReviewCreated)
		if !ok {
			t.Fatalf("expected ReviewCreated event, got %T", events[0])
		}
		if evt.ReviewID != id {
			t.Errorf("ReviewID = %d, want %d", evt.ReviewID, id)
		}
		if evt.ProductID != 100 {
			t.Errorf("ProductID = %d, want 100", evt.ProductID)
		}
		if evt.UserID != 200 {
			t.Errorf("UserID = %d, want 200", evt.UserID)
		}
		if evt.Rating != 5 {
			t.Errorf("Rating = %d, want 5", evt.Rating)
		}
	})
}

func TestReviewApprove(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()
	rating, _ := NewRating(4)
	rv, _ := NewReview(id, 100, 200, rating, "title", "")

	t.Run("approve pending review", func(t *testing.T) {
		rv.ClearEvents()
		err := rv.Approve()
		if err != nil {
			t.Fatalf("Approve() error = %v", err)
		}
		if rv.Status != ReviewStatusApproved {
			t.Errorf("Status = %q, want %q", rv.Status, ReviewStatusApproved)
		}
		events := rv.Events()
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if _, ok := events[0].(*ReviewApproved); !ok {
			t.Fatalf("expected ReviewApproved event")
		}
	})

	t.Run("approve already approved review", func(t *testing.T) {
		err := rv.Approve()
		if err == nil {
			t.Fatal("expected error for approving already approved review")
		}
	})
}

func TestReviewReject(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()
	rating, _ := NewRating(2)
	rv, _ := NewReview(id, 100, 200, rating, "Bad", "Not good")

	t.Run("reject pending review", func(t *testing.T) {
		rv.ClearEvents()
		err := rv.Reject()
		if err != nil {
			t.Fatalf("Reject() error = %v", err)
		}
		if rv.Status != ReviewStatusRejected {
			t.Errorf("Status = %q, want %q", rv.Status, ReviewStatusRejected)
		}
		if _, ok := rv.Events()[0].(*ReviewRejected); !ok {
			t.Fatal("expected ReviewRejected event")
		}
	})

	t.Run("reject already rejected review", func(t *testing.T) {
		err := rv.Reject()
		if err == nil {
			t.Fatal("expected error for rejecting already rejected review")
		}
	})
}

func TestReviewFlag(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()
	rating, _ := NewRating(3)
	rv, _ := NewReview(id, 100, 200, rating, "OK", "")

	t.Run("flag pending review", func(t *testing.T) {
		rv.ClearEvents()
		err := rv.Flag()
		if err != nil {
			t.Fatalf("Flag() error = %v", err)
		}
		if rv.Status != ReviewStatusFlagged {
			t.Errorf("Status = %q, want %q", rv.Status, ReviewStatusFlagged)
		}
		if _, ok := rv.Events()[0].(*ReviewFlagged); !ok {
			t.Fatal("expected ReviewFlagged event")
		}
	})

	t.Run("cannot approve flagged without admin", func(t *testing.T) {
		err := rv.Approve()
		if err == nil {
			t.Fatal("expected error for approving flagged review without unflagging")
		}
	})

	t.Run("reject flagged review", func(t *testing.T) {
		id2, _ := sf.NextID()
		rv2, _ := NewReview(id2, 100, 200, rating, "OK", "")
		rv2.ClearEvents()
		_ = rv2.Approve()
		_ = rv2.Flag()
		err := rv2.Reject()
		if err != nil {
			t.Fatalf("Reject() flagged review error = %v", err)
		}
		if rv2.Status != ReviewStatusRejected {
			t.Errorf("Status = %q, want %q", rv2.Status, ReviewStatusRejected)
		}
	})
}

func TestReviewClearEvents(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()
	rating, _ := NewRating(5)
	rv, _ := NewReview(id, 100, 200, rating, "title", "")

	if len(rv.Events()) == 0 {
		t.Fatal("expected initial events")
	}

	rv.ClearEvents()
	if len(rv.Events()) != 0 {
		t.Fatal("expected cleared events")
	}
}
