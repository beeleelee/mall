package review

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type ReviewService struct {
	repo   ReviewRepository
	logger kernel.Logger
}

func NewReviewService(repo ReviewRepository, logger kernel.Logger) *ReviewService {
	return &ReviewService{repo: repo, logger: logger}
}

func (s *ReviewService) CreateReview(ctx context.Context, id, productID, userID kernel.ID, rating int, title, content string) (*Review, error) {
	r, err := NewRating(rating)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.FindByProductAndUser(ctx, productID, userID)
	if err != nil && !kernel.IsNotFound(err) {
		return nil, err
	}
	if existing != nil {
		return nil, kernel.NewDomainError(kernel.ErrAlreadyExists, "user already reviewed this product")
	}

	rv, err := NewReview(id, productID, userID, r, title, content)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, rv); err != nil {
		return nil, err
	}

	s.logger.Info(ctx, "review.created", kernel.Field("review_id", id.Int64()), kernel.Field("product_id", productID.Int64()), kernel.Field("user_id", userID.Int64()))
	return rv, nil
}

func (s *ReviewService) GetReview(ctx context.Context, id kernel.ID) (*Review, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *ReviewService) GetReviewsByProduct(ctx context.Context, productID kernel.ID, opts ReviewQueryOptions) (*ReviewListResult, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 20
	}
	return s.repo.FindByProduct(ctx, productID, opts)
}

func (s *ReviewService) GetReviewsByUser(ctx context.Context, userID kernel.ID, opts ReviewQueryOptions) (*ReviewListResult, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 20
	}
	return s.repo.FindByUser(ctx, userID, opts)
}

func (s *ReviewService) GetAllReviews(ctx context.Context, opts ReviewQueryOptions) (*ReviewListResult, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 20
	}
	return s.repo.FindAll(ctx, opts)
}

func (s *ReviewService) ApproveReview(ctx context.Context, id kernel.ID) (*Review, error) {
	rv, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := rv.Approve(); err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, rv); err != nil {
		return nil, err
	}
	s.logger.Info(ctx, "review.approved", kernel.Field("review_id", id.Int64()))
	return rv, nil
}

func (s *ReviewService) RejectReview(ctx context.Context, id kernel.ID) (*Review, error) {
	rv, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := rv.Reject(); err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, rv); err != nil {
		return nil, err
	}
	s.logger.Info(ctx, "review.rejected", kernel.Field("review_id", id.Int64()))
	return rv, nil
}

func (s *ReviewService) DeleteReview(ctx context.Context, id kernel.ID) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

func (s *ReviewService) GetAverageRating(ctx context.Context, productID kernel.ID) (float64, error) {
	return s.repo.GetAverageRating(ctx, productID)
}
