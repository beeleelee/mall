package cart

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/beeleelee/mall/domain/kernel"
)

var cartTracer = otel.Tracer("mall.domain.cart")

type CartService struct {
	repo      CartRepository
	publisher CartEventPublisher
	logger    kernel.Logger
}

func NewCartService(repo CartRepository, publisher CartEventPublisher, logger kernel.Logger) *CartService {
	return &CartService{repo: repo, publisher: publisher, logger: logger}
}

type AddItemInput struct {
	CartID    kernel.ID
	UserID    kernel.ID
	ProductID kernel.ID
	SKU       string
	Name      string
	Quantity  int
	UnitPrice int64
	ImageURL  string
}

func (s *CartService) GetOrCreateCart(ctx context.Context, cartID, userID kernel.ID) (*Cart, error) {
	if userID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}

	cart, err := s.repo.FindByUserID(ctx, userID)
	if err != nil && !kernel.IsNotFound(err) {
		return nil, err
	}

	if cart != nil {
		return cart, nil
	}

	cart, err = NewCart(cartID, userID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, cart); err != nil {
		return nil, err
	}

	s.logger.Info(ctx, "cart.created", kernel.Field("cart_id", cart.ID.String()), kernel.Field("user_id", userID.String()))

	return cart, nil
}

func (s *CartService) GetCart(ctx context.Context, userID kernel.ID) (*Cart, error) {
	cart, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return cart, nil
}

func (s *CartService) AddItem(ctx context.Context, input AddItemInput) (*Cart, error) {
	ctx, span := cartTracer.Start(ctx, "cart.add_item",
		trace.WithAttributes(
			attribute.Int64("user_id", input.UserID.Int64()),
			attribute.Int64("product_id", input.ProductID.Int64()),
			attribute.Int("quantity", input.Quantity),
		),
	)
	defer span.End()

	cart, err := s.GetOrCreateCart(ctx, input.CartID, input.UserID)
	if err != nil {
		return nil, err
	}

	item := CartItem{
		ProductID: input.ProductID,
		SKU:       input.SKU,
		Name:      input.Name,
		Quantity:  input.Quantity,
		UnitPrice: input.UnitPrice,
		ImageURL:  input.ImageURL,
	}

	if err := cart.AddItem(item); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, cart); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, cart)

	return cart, nil
}

func (s *CartService) UpdateQuantity(ctx context.Context, userID, productID kernel.ID, quantity int) (*Cart, error) {
	cart, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if err := cart.UpdateQuantity(productID, quantity); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, cart); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, cart)

	return cart, nil
}

func (s *CartService) RemoveItem(ctx context.Context, userID, productID kernel.ID) (*Cart, error) {
	cart, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if err := cart.RemoveItem(productID); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, cart); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, cart)

	return cart, nil
}

func (s *CartService) ClearCart(ctx context.Context, userID kernel.ID) (*Cart, error) {
	cart, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	cart.Clear()

	if err := s.repo.Save(ctx, cart); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, cart)

	return cart, nil
}

func (s *CartService) MergeCarts(ctx context.Context, targetID, sourceID kernel.ID) (*Cart, error) {
	ctx, span := cartTracer.Start(ctx, "cart.merge",
		trace.WithAttributes(
			attribute.Int64("target_id", targetID.Int64()),
			attribute.Int64("source_id", sourceID.Int64()),
		),
	)
	defer span.End()

	target, err := s.repo.FindByID(ctx, targetID)
	if err != nil {
		return nil, err
	}

	source, err := s.repo.FindByID(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	if err := target.Merge(source); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, target); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, source); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, target)
	s.publishEvents(ctx, source)

	return target, nil
}

func (s *CartService) publishEvents(ctx context.Context, cart *Cart) {
	for _, event := range cart.Events() {
		s.logger.Info(ctx, "cart.event", kernel.Field("event", event.EventName()), kernel.Field("cart_id", cart.ID.String()))

		if event.EventName() == "cart.updated" || event.EventName() == "cart.cleared" || event.EventName() == "cart.merged" {
			if err := s.publisher.PublishCartUpdated(ctx, cart); err != nil {
				s.logger.Error(ctx, "cart.publish failed", err, kernel.Field("cart_id", cart.ID.String()))
			}
		}
	}
	cart.ClearEvents()
}
