package order

import (
	"context"

	checkout "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
)

type OrderService struct {
	repo      OrderRepository
	publisher OrderEventPublisher
	logger    kernel.Logger
}

func NewOrderService(repo OrderRepository, publisher OrderEventPublisher, logger kernel.Logger) *OrderService {
	return &OrderService{repo: repo, publisher: publisher, logger: logger}
}

func (s *OrderService) CreateOrder(ctx context.Context, session *checkout.CheckoutSession) (*Order, error) {
	id := session.ID
	order, err := NewOrderFromCheckout(id, session)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, order); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, order)
	s.logger.Info(ctx, "order.created", kernel.Field("order_id", order.ID.String()), kernel.Field("user_id", order.UserID.String()))
	return order, nil
}

func (s *OrderService) GetOrder(ctx context.Context, id kernel.ID) (*Order, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *OrderService) GetOrdersByUser(ctx context.Context, userID kernel.ID) ([]*Order, error) {
	return s.repo.FindByUserID(ctx, userID)
}

func (s *OrderService) StartProcessing(ctx context.Context, id kernel.ID) (*Order, error) {
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := order.StartProcessing(); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, order); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, order)
	return order, nil
}

func (s *OrderService) Ship(ctx context.Context, id kernel.ID, trackingNumber, carrier string) (*Order, error) {
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := order.Ship(trackingNumber, carrier); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, order); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, order)
	return order, nil
}

func (s *OrderService) MarkDelivered(ctx context.Context, id kernel.ID) (*Order, error) {
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := order.MarkDelivered(); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, order); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, order)
	return order, nil
}

func (s *OrderService) ReturnOrder(ctx context.Context, id kernel.ID) (*Order, error) {
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := order.Return(); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, order); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, order)
	return order, nil
}

func (s *OrderService) Cancel(ctx context.Context, id kernel.ID) (*Order, error) {
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := order.Cancel(); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, order); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, order)
	return order, nil
}

func (s *OrderService) publishEvents(ctx context.Context, order *Order) {
	for range order.Events() {
		if err := s.publisher.PublishOrderEvent(ctx, order); err != nil {
			s.logger.Error(ctx, "order.publish failed", err, kernel.Field("order_id", order.ID.String()))
		}
	}
	order.ClearEvents()
}
