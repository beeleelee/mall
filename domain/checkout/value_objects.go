package checkout

import "github.com/beeleelee/mall/domain/kernel"

type CheckoutStatus string

const (
	CheckoutStatusIncomplete         CheckoutStatus = "incomplete"
	CheckoutStatusReadyForComplete   CheckoutStatus = "ready_for_complete"
	CheckoutStatusRequiresEscalation CheckoutStatus = "requires_escalation"
	CheckoutStatusCompleted          CheckoutStatus = "completed"
	CheckoutStatusCancelled          CheckoutStatus = "cancelled"
)

type CartSnapshotItem struct {
	ProductID kernel.ID `json:"product_id"`
	SKU       string    `json:"sku"`
	Name      string    `json:"name"`
	Quantity  int       `json:"quantity"`
	UnitPrice int64     `json:"unit_price"`
	ImageURL  string    `json:"image_url,omitempty"`
}

func (i CartSnapshotItem) TotalPrice() int64 {
	return i.UnitPrice * int64(i.Quantity)
}

type CartSnapshot struct {
	Items []CartSnapshotItem `json:"items"`
	Total int64              `json:"total"`
}

func NewCartSnapshot(items []CartSnapshotItem) CartSnapshot {
	var total int64
	for _, item := range items {
		total += item.TotalPrice()
	}
	return CartSnapshot{Items: items, Total: total}
}

type Address struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

func (a Address) IsValid() bool {
	return a.Line1 != "" && a.City != "" && a.State != "" && a.PostalCode != "" && a.Country != ""
}

type ShippingOption struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Cost      int64  `json:"cost"`
	Estimated string `json:"estimated,omitempty"`
}
