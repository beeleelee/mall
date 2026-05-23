package kernel

type AggregateRoot struct {
	Entity
	events []DomainEvent
}

func NewAggregateRoot(id ID) AggregateRoot {
	return AggregateRoot{
		Entity: NewEntity(id),
	}
}

func (a *AggregateRoot) AddEvent(event DomainEvent) {
	a.events = append(a.events, event)
}

func (a *AggregateRoot) Events() []DomainEvent {
	return a.events
}

func (a *AggregateRoot) ClearEvents() {
	a.events = nil
}
