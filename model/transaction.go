package model

import "time"

// 取引ステータス
type TransactionStatus string

const (
	TransactionStatusPending   TransactionStatus = "pending"
	TransactionStatusShipping  TransactionStatus = "shipping"
	TransactionStatusCompleted TransactionStatus = "completed"
	TransactionStatusCanceled  TransactionStatus = "canceled"
)

// 取引テーブル
type Transaction struct {
	ID         uint              `gorm:"primaryKey" json:"id"`
	ItemID     uint              `gorm:"not null" json:"item_id"`
	Item       Item              `gorm:"foreignKey:ItemID" json:"item,omitempty"`
	BuyerID    uint              `gorm:"not null" json:"buyer_id"`
	Buyer      User              `gorm:"foreignKey:BuyerID" json:"buyer,omitempty"`
	SellerID   uint              `gorm:"not null" json:"seller_id"`
	Seller     User              `gorm:"foreignKey:SellerID" json:"seller,omitempty"`
	FinalPrice int               `gorm:"not null" json:"final_price"`
	Status     TransactionStatus `gorm:"type:varchar(20);default:'pending'" json:"status"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// レビューテーブル
type Review struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	TransactionID uint      `gorm:"not null" json:"transaction_id"`
	ReviewerID    uint      `gorm:"not null" json:"reviewer_id"`
	Reviewer      User      `gorm:"foreignKey:ReviewerID" json:"reviewer,omitempty"`
	RevieweeID    uint      `gorm:"not null" json:"reviewee_id"`
	Rating        int       `gorm:"not null" json:"rating"`
	Comment       string    `json:"comment"`
	CreatedAt     time.Time `json:"created_at"`
}
