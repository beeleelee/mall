package wishlist

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type WishlistService struct {
	repo   WishlistRepository
	sf     *kernel.Snowflake
	logger kernel.Logger
}

func NewWishlistService(repo WishlistRepository, sf *kernel.Snowflake, logger kernel.Logger) *WishlistService {
	return &WishlistService{repo: repo, sf: sf, logger: logger}
}

func (s *WishlistService) getOrCreate(ctx context.Context, userID kernel.ID) (*Wishlist, error) {
	existing, err := s.repo.FindByUserID(ctx, userID)
	if err != nil && !kernel.IsNotFound(err) {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	id, err := s.sf.NextID()
	if err != nil {
		return nil, err
	}

	w, err := NewWishlist(id, userID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, w); err != nil {
		return nil, err
	}

	return w, nil
}

func (s *WishlistService) AddItem(ctx context.Context, userID, productID kernel.ID) error {
	w, err := s.getOrCreate(ctx, userID)
	if err != nil {
		return err
	}

	if err := w.AddItem(productID); err != nil {
		return err
	}

	if err := s.repo.Save(ctx, w); err != nil {
		return err
	}

	s.logger.Info(ctx, "wishlist.item_added", kernel.Field("user_id", userID.Int64()), kernel.Field("product_id", productID.Int64()))
	return nil
}

func (s *WishlistService) RemoveItem(ctx context.Context, userID, productID kernel.ID) error {
	w, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return err
	}

	if err := w.RemoveItem(productID); err != nil {
		return err
	}

	if err := s.repo.Save(ctx, w); err != nil {
		return err
	}

	s.logger.Info(ctx, "wishlist.item_removed", kernel.Field("user_id", userID.Int64()), kernel.Field("product_id", productID.Int64()))
	return nil
}

func (s *WishlistService) GetWishlist(ctx context.Context, userID kernel.ID) (*Wishlist, error) {
	w, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		if kernel.IsNotFound(err) {
			idl, _ := s.sf.NextID()
			return NewWishlist(idl, userID)
		}
		return nil, err
	}
	return w, nil
}

func (s *WishlistService) ClearWishlist(ctx context.Context, userID kernel.ID) error {
	w, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return err
	}

	w.Clear()
	if err := s.repo.Save(ctx, w); err != nil {
		return err
	}

	s.logger.Info(ctx, "wishlist.cleared", kernel.Field("user_id", userID.Int64()))
	return nil
}

func (s *WishlistService) IsInWishlist(ctx context.Context, userID, productID kernel.ID) (bool, error) {
	w, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		if kernel.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return w.Contains(productID), nil
}
