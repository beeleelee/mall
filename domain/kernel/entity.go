package kernel

import "time"

type Entity struct {
	ID        ID
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewEntity(id ID) Entity {
	return Entity{
		ID:        id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (e Entity) Equals(other Entity) bool {
	return e.ID == other.ID
}
