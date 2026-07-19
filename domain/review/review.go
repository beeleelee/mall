package review

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type Rating int

func NewRating(r int) (Rating, error) {
	if r < 1 || r > 5 {
		return 0, kernel.NewDomainError(kernel.ErrInvalidArgument, "rating must be between 1 and 5")
	}
	return Rating(r), nil
}

type ReviewStatus string

const (
	ReviewStatusPending  ReviewStatus = "pending"
	ReviewStatusApproved ReviewStatus = "approved"
	ReviewStatusRejected ReviewStatus = "rejected"
	ReviewStatusFlagged  ReviewStatus = "flagged"
)

type Review struct {
	kernel.AggregateRoot
	ProductID kernel.ID
	UserID    kernel.ID
	Rating    Rating
	Title     string
	Content   string
	Status    ReviewStatus
}

func NewReview(id kernel.ID, productID, userID kernel.ID, rating Rating, title, content string) (*Review, error) {
	if id == 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "review id must not be empty")
	}
	if productID == 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "product id must not be empty")
	}
	if userID == 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user id must not be empty")
	}

	rv := &Review{
		AggregateRoot: kernel.NewAggregateRoot(id),
		ProductID:     productID,
		UserID:        userID,
		Rating:        rating,
		Title:         title,
		Content:       content,
		Status:        ReviewStatusPending,
	}

	rv.AddEvent(&ReviewCreated{
		ReviewID:  id,
		ProductID: productID,
		UserID:    userID,
		Rating:    int(rating),
	})

	return rv, nil
}

func (r *Review) Approve() error {
	if r.Status != ReviewStatusPending {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only approve a pending review")
	}
	r.Status = ReviewStatusApproved
	r.UpdatedAt = time.Now()
	r.AddEvent(&ReviewApproved{
		ReviewID:  r.ID,
		ProductID: r.ProductID,
	})
	return nil
}

func (r *Review) Reject() error {
	if r.Status != ReviewStatusPending && r.Status != ReviewStatusFlagged {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only reject a pending or flagged review")
	}
	r.Status = ReviewStatusRejected
	r.UpdatedAt = time.Now()
	r.AddEvent(&ReviewRejected{
		ReviewID:  r.ID,
		ProductID: r.ProductID,
	})
	return nil
}

func (r *Review) Flag() error {
	if r.Status != ReviewStatusApproved && r.Status != ReviewStatusPending {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only flag an approved or pending review")
	}
	r.Status = ReviewStatusFlagged
	r.UpdatedAt = time.Now()
	r.AddEvent(&ReviewFlagged{
		ReviewID:  r.ID,
		ProductID: r.ProductID,
	})
	return nil
}

type ReviewCreated struct {
	ReviewID  kernel.ID
	ProductID kernel.ID
	UserID    kernel.ID
	Rating    int
}

func (e *ReviewCreated) EventName() string      { return "review.created" }
func (e *ReviewCreated) OccurredAt() time.Time  { return time.Now() }
func (e *ReviewCreated) AggregateID() kernel.ID { return e.ReviewID }

type ReviewApproved struct {
	ReviewID  kernel.ID
	ProductID kernel.ID
}

func (e *ReviewApproved) EventName() string      { return "review.approved" }
func (e *ReviewApproved) OccurredAt() time.Time  { return time.Now() }
func (e *ReviewApproved) AggregateID() kernel.ID { return e.ReviewID }

type ReviewRejected struct {
	ReviewID  kernel.ID
	ProductID kernel.ID
}

func (e *ReviewRejected) EventName() string      { return "review.rejected" }
func (e *ReviewRejected) OccurredAt() time.Time  { return time.Now() }
func (e *ReviewRejected) AggregateID() kernel.ID { return e.ReviewID }

type ReviewFlagged struct {
	ReviewID  kernel.ID
	ProductID kernel.ID
}

func (e *ReviewFlagged) EventName() string      { return "review.flagged" }
func (e *ReviewFlagged) OccurredAt() time.Time  { return time.Now() }
func (e *ReviewFlagged) AggregateID() kernel.ID { return e.ReviewID }
