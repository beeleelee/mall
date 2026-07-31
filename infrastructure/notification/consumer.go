package notification

import (
	"context"
	"encoding/json"
	"log"
	"strconv"

	"github.com/nats-io/nats.go/jetstream"

	domainidentity "github.com/beeleelee/mall/domain/identity"
	"github.com/beeleelee/mall/domain/kernel"
	domainnotification "github.com/beeleelee/mall/domain/notification"
	domainsubscription "github.com/beeleelee/mall/domain/subscription"
	"github.com/beeleelee/mall/infrastructure/tracing"
)

type orderEvent struct {
	OrderID int64  `json:"order_id"`
	UserID  int64  `json:"user_id"`
	Status  string `json:"status"`
	Total   int64  `json:"grand_total"`
}

type subscriptionEvent struct {
	SubscriptionID int64  `json:"subscription_id"`
	UserID         int64  `json:"user_id"`
	PlanID         int64  `json:"plan_id"`
	Status         string `json:"status"`
}

type NotificationConsumer struct {
	js       jetstream.JetStream
	notifSvc *domainnotification.NotificationService
	userRepo domainidentity.UserRepository
	sf       *kernel.Snowflake
}

func NewNotificationConsumer(js jetstream.JetStream, notifSvc *domainnotification.NotificationService, userRepo domainidentity.UserRepository, sf *kernel.Snowflake) *NotificationConsumer {
	return &NotificationConsumer{js: js, notifSvc: notifSvc, userRepo: userRepo, sf: sf}
}

func (c *NotificationConsumer) Start(ctx context.Context) error {
	if err := c.startOrderConsumer(ctx); err != nil {
		return err
	}
	return c.startSubscriptionConsumer(ctx)
}

func (c *NotificationConsumer) startOrderConsumer(ctx context.Context) error {
	cons, err := c.js.CreateOrUpdateConsumer(context.Background(), "orders", jetstream.ConsumerConfig{
		Name:          "email-notifications",
		FilterSubject: "order.>",
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		return err
	}

	cons.Consume(func(msg jetstream.Msg) {
		ctx := tracing.ExtractFromJetStream(msg)

		var evt orderEvent
		if err := json.Unmarshal(msg.Data(), &evt); err != nil {
			log.Printf("notification consumer: failed to unmarshal order event: %v", err)
			msg.Ack()
			return
		}

		user, err := c.userRepo.FindByID(ctx, kernel.ID(evt.UserID))
		if err != nil {
			log.Printf("notification consumer: user %d not found: %v", evt.UserID, err)
			msg.Ack()
			return
		}

		to := domainnotification.EmailAddress(user.Email)

		switch evt.Status {
		case "confirmed":
			c.notifSvc.SendOrderConfirmationPref(ctx, user.ID, to, user.Name, evt.OrderID, evt.Total)
			c.notifyInApp(ctx, user.ID, domainnotification.NotificationTypeOrder, "Order Confirmed", "Your order #"+strconv.FormatInt(evt.OrderID, 10)+" has been confirmed.")
		case "shipped":
			c.notifSvc.SendShippingUpdatePref(ctx, user.ID, to, user.Name, evt.OrderID, "shipped")
			c.notifyInApp(ctx, user.ID, domainnotification.NotificationTypeShipping, "Order Shipped", "Your order #"+strconv.FormatInt(evt.OrderID, 10)+" has been shipped.")
		case "delivered":
			c.notifSvc.SendShippingUpdatePref(ctx, user.ID, to, user.Name, evt.OrderID, "delivered")
			c.notifyInApp(ctx, user.ID, domainnotification.NotificationTypeShipping, "Order Delivered", "Your order #"+strconv.FormatInt(evt.OrderID, 10)+" has been delivered.")
		}
		msg.Ack()
	})
	return nil
}

func (c *NotificationConsumer) startSubscriptionConsumer(ctx context.Context) error {
	cons, err := c.js.CreateOrUpdateConsumer(context.Background(), "subscriptions", jetstream.ConsumerConfig{
		Name:          "subscription-notifications",
		FilterSubject: "subscription.>",
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		return err
	}

	cons.Consume(func(msg jetstream.Msg) {
		ctx := tracing.ExtractFromJetStream(msg)

		var evt subscriptionEvent
		if err := json.Unmarshal(msg.Data(), &evt); err != nil {
			log.Printf("notification consumer: failed to unmarshal subscription event: %v", err)
			msg.Ack()
			return
		}

		user, err := c.userRepo.FindByID(ctx, kernel.ID(evt.UserID))
		if err != nil {
			log.Printf("notification consumer: user %d not found: %v", evt.UserID, err)
			msg.Ack()
			return
		}

		to := domainnotification.EmailAddress(user.Email)

		switch domainsubscription.SubscriptionStatus(evt.Status) {
		case domainsubscription.SubscriptionStatusActive:
			c.notifyInApp(ctx, user.ID, domainnotification.NotificationTypeSubscription, "Subscription Active", "Your subscription #"+strconv.FormatInt(evt.SubscriptionID, 10)+" is now active.")
		case domainsubscription.SubscriptionStatusPastDue:
			c.notifSvc.SendPaymentFailed(ctx, user.ID, to, user.Name, evt.SubscriptionID)
			c.notifyInApp(ctx, user.ID, domainnotification.NotificationTypeSubscription, "Payment Failed", "We could not charge your subscription #"+strconv.FormatInt(evt.SubscriptionID, 10)+".")
		case domainsubscription.SubscriptionStatusExpired:
			c.notifSvc.SendSubscriptionExpired(ctx, user.ID, to, user.Name, evt.SubscriptionID)
			c.notifyInApp(ctx, user.ID, domainnotification.NotificationTypeSubscription, "Subscription Expired", "Your subscription #"+strconv.FormatInt(evt.SubscriptionID, 10)+" has expired.")
		}
		msg.Ack()
	})
	return nil
}

func (c *NotificationConsumer) notifyInApp(ctx context.Context, userID kernel.ID, ntype domainnotification.NotificationType, title, body string) {
	id, err := c.sf.NextID()
	if err != nil {
		log.Printf("notification consumer: generate id failed: %v", err)
		return
	}
	if err := c.notifSvc.NotifyInApp(ctx, id, userID, ntype, title, body); err != nil {
		log.Printf("notification consumer: in-app notify failed: %v", err)
	}
}
