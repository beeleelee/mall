package kernel

import "time"

type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
	AggregateID() ID
}
