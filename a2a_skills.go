package main

import (
	"context"
	"fmt"
	"strings"

	appSubscription "github.com/beeleelee/mall/application/subscription"
	domainA2A "github.com/beeleelee/mall/domain/a2a"
	domainDiscount "github.com/beeleelee/mall/domain/discount"
	domainFulfillment "github.com/beeleelee/mall/domain/fulfillment"
	domainInventory "github.com/beeleelee/mall/domain/inventory"
	"github.com/beeleelee/mall/domain/kernel"
	domainNotification "github.com/beeleelee/mall/domain/notification"
	domainPayment "github.com/beeleelee/mall/domain/payment"
	domainReview "github.com/beeleelee/mall/domain/review"
	domainWishlist "github.com/beeleelee/mall/domain/wishlist"
)

func skillText(msg domainA2A.Message) string {
	for _, p := range msg.Parts {
		if p.Type == domainA2A.PartTypeText {
			return p.Text
		}
	}
	return ""
}

func skillData(msg domainA2A.Message) (map[string]any, bool) {
	for _, p := range msg.Parts {
		if p.Type == domainA2A.PartTypeData {
			if m, ok := p.Data.(map[string]any); ok {
				return m, true
			}
		}
	}
	return nil, false
}

func numArg(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	}
	return 0
}

func strArg(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// discountSkillHandler validates a discount code mentioned in the message.
type discountSkillHandler struct {
	svc *domainDiscount.DiscountService
}

func (h *discountSkillHandler) Handle(ctx context.Context, task *domainA2A.Task, msg domainA2A.Message) error {
	code := ""
	if data, ok := skillData(msg); ok {
		code = strArg(data, "code")
	}
	if code == "" {
		text := skillText(msg)
		for _, tok := range strings.Fields(text) {
			t := strings.TrimSpace(tok)
			if len(t) >= 4 && t == strings.ToUpper(t) {
				code = t
				break
			}
		}
	}
	if code == "" {
		task.AddArtifact(domainA2A.Artifact{
			ID:    "discount-result",
			Name:  "Discount Response",
			Parts: []domainA2A.Part{domainA2A.TextPart("no discount code found in message")},
		})
		return nil
	}

	dc, valid := h.svc.ValidateCode(ctx, code, 0)
	if !valid || dc == nil {
		task.AddArtifact(domainA2A.Artifact{
			ID:    "discount-result",
			Name:  "Discount Response",
			Parts: []domainA2A.Part{domainA2A.TextPart(fmt.Sprintf("discount code %q is not valid", code))},
		})
		return nil
	}
	task.AddArtifact(domainA2A.Artifact{
		ID:    "discount-result",
		Name:  "Discount Response",
		Parts: []domainA2A.Part{domainA2A.TextPart(fmt.Sprintf("discount code %q is valid (%s %d)", dc.Code, dc.Type, dc.Value))},
	})
	return nil
}

// inventorySkillHandler reports stock levels for a product or low-stock list.
type inventorySkillHandler struct {
	svc *domainInventory.InventoryService
}

func (h *inventorySkillHandler) Handle(ctx context.Context, task *domainA2A.Task, msg domainA2A.Message) error {
	if data, ok := skillData(msg); ok {
		if pid := numArg(data, "product_id"); pid > 0 {
			item, err := h.svc.GetStock(ctx, kernel.ID(pid))
			if err != nil {
				return err
			}
			task.AddArtifact(domainA2A.Artifact{
				ID:    "inventory-result",
				Name:  "Inventory Response",
				Parts: []domainA2A.Part{domainA2A.TextPart(fmt.Sprintf("product %d has %d in stock", item.ProductID.Int64(), item.QuantityAvailable))},
			})
			return nil
		}
	}

	items, err := h.svc.ListLowStock(ctx, 10)
	if err != nil {
		return err
	}
	task.AddArtifact(domainA2A.Artifact{
		ID:    "inventory-result",
		Name:  "Inventory Response",
		Parts: []domainA2A.Part{domainA2A.TextPart(fmt.Sprintf("found %d low-stock products", len(items)))},
	})
	return nil
}

// paymentSkillHandler lists the user's mandates.
type paymentSkillHandler struct {
	svc *domainPayment.PaymentService
}

func (h *paymentSkillHandler) Handle(ctx context.Context, task *domainA2A.Task, msg domainA2A.Message) error {
	mandates, err := h.svc.ListUserMandates(ctx, task.UserID)
	if err != nil {
		return err
	}
	task.AddArtifact(domainA2A.Artifact{
		ID:    "payment-result",
		Name:  "Payment Response",
		Parts: []domainA2A.Part{domainA2A.TextPart(fmt.Sprintf("found %d mandates for user %d", len(mandates), task.UserID.Int64()))},
	})
	return nil
}

// fulfillmentSkillHandler calculates shipping rates for a destination.
type fulfillmentSkillHandler struct {
	svc domainFulfillment.RateCalculator
}

func (h *fulfillmentSkillHandler) Handle(ctx context.Context, task *domainA2A.Task, msg domainA2A.Message) error {
	country := "US"
	if data, ok := skillData(msg); ok {
		if c := strArg(data, "country"); c != "" {
			country = c
		}
	}

	result, err := h.svc.CalculateRates(ctx, domainFulfillment.RateInput{
		DestinationCountry: country,
	})
	if err != nil {
		return err
	}
	names := make([]string, 0, len(result.Options))
	for _, o := range result.Options {
		names = append(names, fmt.Sprintf("%s(%d)", o.Name, o.Cost))
	}
	task.AddArtifact(domainA2A.Artifact{
		ID:    "fulfillment-result",
		Name:  "Fulfillment Response",
		Parts: []domainA2A.Part{domainA2A.TextPart(fmt.Sprintf("shipping to %s: %s", country, strings.Join(names, ", ")))},
	})
	return nil
}

// reviewSkillHandler lists the user's reviews.
type reviewSkillHandler struct {
	svc *domainReview.ReviewService
}

func (h *reviewSkillHandler) Handle(ctx context.Context, task *domainA2A.Task, msg domainA2A.Message) error {
	result, err := h.svc.GetReviewsByUser(ctx, task.UserID, domainReview.ReviewQueryOptions{Limit: 10})
	if err != nil {
		return err
	}
	task.AddArtifact(domainA2A.Artifact{
		ID:    "review-result",
		Name:  "Review Response",
		Parts: []domainA2A.Part{domainA2A.TextPart(fmt.Sprintf("found %d reviews for user %d", result.Total, task.UserID.Int64()))},
	})
	return nil
}

// wishlistSkillHandler returns the user's wishlist.
type wishlistSkillHandler struct {
	svc *domainWishlist.WishlistService
}

func (h *wishlistSkillHandler) Handle(ctx context.Context, task *domainA2A.Task, msg domainA2A.Message) error {
	wl, err := h.svc.GetWishlist(ctx, task.UserID)
	if err != nil {
		return err
	}
	task.AddArtifact(domainA2A.Artifact{
		ID:    "wishlist-result",
		Name:  "Wishlist Response",
		Parts: []domainA2A.Part{domainA2A.TextPart(fmt.Sprintf("wishlist for user %d has %d items", task.UserID.Int64(), len(wl.Items)))},
	})
	return nil
}

// subscriptionSkillHandler lists the user's subscriptions.
type subscriptionSkillHandler struct {
	svc *appSubscription.SubscriptionAppService
}

func (h *subscriptionSkillHandler) Handle(ctx context.Context, task *domainA2A.Task, msg domainA2A.Message) error {
	subs, err := h.svc.ListUserSubscriptions(ctx, task.UserID.Int64())
	if err != nil {
		return err
	}
	task.AddArtifact(domainA2A.Artifact{
		ID:    "subscription-result",
		Name:  "Subscription Response",
		Parts: []domainA2A.Part{domainA2A.TextPart(fmt.Sprintf("found %d subscriptions for user %d", len(subs), task.UserID.Int64()))},
	})
	return nil
}

// notificationSkillHandler lists the user's notifications.
type notificationSkillHandler struct {
	svc *domainNotification.NotificationService
}

func (h *notificationSkillHandler) Handle(ctx context.Context, task *domainA2A.Task, msg domainA2A.Message) error {
	notifs, err := h.svc.ListByUser(ctx, task.UserID, 10)
	if err != nil {
		return err
	}
	unread := 0
	for _, n := range notifs {
		if !n.Read {
			unread++
		}
	}
	task.AddArtifact(domainA2A.Artifact{
		ID:    "notification-result",
		Name:  "Notification Response",
		Parts: []domainA2A.Part{domainA2A.TextPart(fmt.Sprintf("found %d notifications (%d unread) for user %d", len(notifs), unread, task.UserID.Int64()))},
	})
	return nil
}
