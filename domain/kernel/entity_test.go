package kernel

import (
	"testing"
	"time"
)

func TestEntity_Equals(t *testing.T) {
	id1 := ID(1)
	id2 := ID(2)

	e1 := NewEntity(id1)
	time.Sleep(time.Nanosecond)
	e2 := NewEntity(id1)
	e3 := NewEntity(id2)

	if !e1.Equals(e2) {
		t.Error("entities with same ID should be equal")
	}
	if e1.Equals(e3) {
		t.Error("entities with different IDs should not be equal")
	}
}
