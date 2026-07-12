package order

import (
	"context"

	"github.com/beeleelee/mall/domain/inventory"
	"github.com/beeleelee/mall/domain/kernel"
	"github.com/beeleelee/mall/domain/payment"
)

type RefundService struct {
	refundRepo    RefundRepository
	paymentSvc    *payment.PaymentService
	inventorySvc  *inventory.InventoryService
	orderSvc      *OrderService
	logger        kernel.Logger
}

func NewRefundService(refundRepo RefundRepository, paymentSvc *payment.PaymentService, inventorySvc *inventory.InventoryService, orderSvc *OrderService, logger kernel.Logger) *RefundService {
	return &RefundService{
		refundRepo:   refundRepo,
		paymentSvc:   paymentSvc,
		inventorySvc: inventorySvc,
		orderSvc:     orderSvc,
		logger:       logger,
	}
}

func (s *RefundService) ProcessRefund(ctx context.Context, refundID kernel.ID, order *Order, mandateID kernel.ID, reason string) (*Refund, error) {
	s.logger.Info(ctx, "refund.initiate", kernel.Field("order_id", order.ID.String()), kernel.Field("amount", order.GrandTotal))

	refund, err := NewRefund(refundID, order.ID, mandateID, order.GrandTotal, reason)
	if err != nil {
		return nil, err
	}

	if mandateID > 0 {
		reverseMandate, revErr := s.paymentSvc.RefundMandate(ctx, mandateID, order.GrandTotal)
		if revErr != nil {
			refund.MarkFailed(revErr.Error())
			_ = s.refundRepo.Save(ctx, refund)
			return nil, revErr
		}
		s.logger.Info(ctx, "refund.payment_reversed", kernel.Field("mandate_id", reverseMandate.ID.String()))
	}

	for _, item := range order.Items {
		if _, err := s.inventorySvc.Restock(ctx, item.ProductID, item.Quantity); err != nil {
			s.logger.Error(ctx, "refund.restock_failed", err, kernel.Field("product_id", item.ProductID.String()))
		}
	}

	if err := refund.MarkProcessed(); err != nil {
		return nil, err
	}

	if err := s.refundRepo.Save(ctx, refund); err != nil {
		return nil, err
	}

	s.logger.Info(ctx, "refund.processed", kernel.Field("refund_id", refund.ID.String()), kernel.Field("order_id", order.ID.String()))
	return refund, nil
}
