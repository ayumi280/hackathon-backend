package model

import "time"

// オファーステータス（値下げ交渉フロー）
type OfferStatus string

const (
	OfferStatusPending   OfferStatus = "pending"
	OfferStatusAccepted  OfferStatus = "accepted"
	OfferStatusRejected  OfferStatus = "rejected"
	OfferStatusCountered OfferStatus = "countered"
)

// オファーテーブル
type Offer struct {
	ID           uint        `gorm:"primaryKey" json:"id"`
	ItemID       uint        `gorm:"not null" json:"item_id"`
	Item         Item        `gorm:"foreignKey:ItemID" json:"item,omitempty"`
	BuyerID      uint        `gorm:"not null" json:"buyer_id"`
	Buyer        User        `gorm:"foreignKey:BuyerID" json:"buyer,omitempty"`
	OfferedPrice int         `gorm:"not null" json:"offered_price"`
	Status       OfferStatus `gorm:"type:varchar(20);default:'pending'" json:"status"`
	CounterPrice *int        `json:"counter_price"`
	Message      string      `json:"message"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}
