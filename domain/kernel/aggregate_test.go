package kernel

import (
	"testing"
	"time"
)

type testEvent struct {
	name      string
	occurred  time.Time
	aggregate ID
}

func (e testEvent) EventName() string    { return e.name }
func (e testEvent) OccurredAt() time.Time { return e.occurred }
func (e testEvent) AggregateID() ID       { return e.aggregate }

func TestAggregateRoot_Events(t *testing.T) {
	agg := NewAggregateRoot(1)

	if len(agg.Events()) != 0 {
		t.Error("new aggregate should have no events")
	}

	agg.AddEvent(testEvent{name: "test.event", occurred: time.Now(), aggregate: 1})
	agg.AddEvent(testEvent{name: "test.event2", occurred: time.Now(), aggregate: 1})

	if len(agg.Events()) != 2 {
		t.Errorf("expected 2 events, got %d", len(agg.Events()))
	}

	agg.ClearEvents()
	if len(agg.Events()) != 0 {
		t.Error("events should be cleared")
	}
}
