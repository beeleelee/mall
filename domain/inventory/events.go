package inventory

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type StockUpdated struct {
	ProductID kernel.ID
	Quantity  int
}

func (e *StockUpdated) EventName() string      { return "inventory.stock.updated" }
func (e *StockUpdated) OccurredAt() time.Time  { return time.Now() }
func (e *StockUpdated) AggregateID() kernel.ID { return e.ProductID }

type StockReserved struct {
	ProductID kernel.ID
	Quantity  int
}

func (e *StockReserved) EventName() string      { return "inventory.stock.reserved" }
func (e *StockReserved) OccurredAt() time.Time  { return time.Now() }
func (e *StockReserved) AggregateID() kernel.ID { return e.ProductID }

type StockReservationReleased struct {
	ProductID kernel.ID
	Quantity  int
}

func (e *StockReservationReleased) EventName() string      { return "inventory.stock.reservation_released" }
func (e *StockReservationReleased) OccurredAt() time.Time  { return time.Now() }
func (e *StockReservationReleased) AggregateID() kernel.ID { return e.ProductID }

type StockReservationConfirmed struct {
	ProductID kernel.ID
	Quantity  int
}

func (e *StockReservationConfirmed) EventName() string {
	return "inventory.stock.reservation_confirmed"
}
func (e *StockReservationConfirmed) OccurredAt() time.Time  { return time.Now() }
func (e *StockReservationConfirmed) AggregateID() kernel.ID { return e.ProductID }

type StockLow struct {
	ProductID kernel.ID
	Quantity  int
}

func (e *StockLow) EventName() string      { return "inventory.stock.low" }
func (e *StockLow) OccurredAt() time.Time  { return time.Now() }
func (e *StockLow) AggregateID() kernel.ID { return e.ProductID }

type StockOutOfStock struct {
	ProductID kernel.ID
}

func (e *StockOutOfStock) EventName() string      { return "inventory.stock.out_of_stock" }
func (e *StockOutOfStock) OccurredAt() time.Time  { return time.Now() }
func (e *StockOutOfStock) AggregateID() kernel.ID { return e.ProductID }

type StockRestocked struct {
	ProductID kernel.ID
	Quantity  int
}

func (e *StockRestocked) EventName() string      { return "inventory.stock.restocked" }
func (e *StockRestocked) OccurredAt() time.Time  { return time.Now() }
func (e *StockRestocked) AggregateID() kernel.ID { return e.ProductID }
