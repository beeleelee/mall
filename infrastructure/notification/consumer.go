package notification

import (
	"context"
	"encoding/json"
	"log"

	"github.com/nats-io/nats.go/jetstream"

	domainidentity "github.com/beeleelee/mall/domain/identity"
	domainnotification "github.com/beeleelee/mall/domain/notification"
	"github.com/beeleelee/mall/domain/kernel"
	"github.com/beeleelee/mall/infrastructure/tracing"
)

type orderEvent struct {
	OrderID int64  `json:"order_id"`
	UserID  int64  `json:"user_id"`
	Status  string `json:"status"`
}

func StartEmailConsumer(js jetstream.JetStream, notifSvc *domainnotification.NotificationService, userRepo domainidentity.UserRepository) {
	go func() {
		cons, err := js.CreateOrUpdateConsumer(context.Background(), "orders", jetstream.ConsumerConfig{
			Name:          "email-notifications",
			FilterSubject: "order.>",
			AckPolicy:     jetstream.AckExplicitPolicy,
		})
		if err != nil {
			log.Fatalf("create email consumer: %v", err)
		}

		cons.Consume(func(msg jetstream.Msg) {
			ctx := tracing.ExtractFromJetStream(msg)

			var evt orderEvent
			if err := json.Unmarshal(msg.Data(), &evt); err != nil {
				log.Printf("email consumer: failed to unmarshal event: %v", err)
				msg.Ack()
				return
			}

			user, err := userRepo.FindByID(ctx, kernel.ID(evt.UserID))
			if err != nil {
				log.Printf("email consumer: user %d not found: %v", evt.UserID, err)
				msg.Ack()
				return
			}

			to := domainnotification.EmailAddress(user.Email)

			switch evt.Status {
			case "confirmed":
				if err := notifSvc.SendOrderConfirmation(ctx, to, user.Name, evt.OrderID, 0); err != nil {
					log.Printf("email consumer: send confirmation failed: %v", err)
				}
			case "shipped":
				if err := notifSvc.SendShippingUpdate(ctx, to, user.Name, evt.OrderID, "shipped"); err != nil {
					log.Printf("email consumer: send shipping update failed: %v", err)
				}
			case "delivered":
				if err := notifSvc.SendShippingUpdate(ctx, to, user.Name, evt.OrderID, "delivered"); err != nil {
					log.Printf("email consumer: send shipping update failed: %v", err)
				}
			}
			msg.Ack()
		})
	}()
}
