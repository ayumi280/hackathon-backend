package model

import "time"

// 商品ステータス
type ItemStatus string

const (
	ItemStatusSelling ItemStatus = "selling"
	ItemStatusTrading ItemStatus = "trading"
	ItemStatusSold    ItemStatus = "sold"
)

// カテゴリテーブル
type Category struct {
	ID       uint    `gorm:"primaryKey" json:"id"`
	Name     string  `gorm:"not null" json:"name"`
	ParentID *uint   `json:"parent_id"`
}

// 商品テーブル
type Item struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	SellerID    uint       `gorm:"not null" json:"seller_id"`
	Seller      User       `gorm:"foreignKey:SellerID" json:"seller,omitempty"`
	Title       string     `gorm:"not null" json:"title"`
	Description string     `gorm:"type:text" json:"description"`
	Price       int        `gorm:"not null" json:"price"`
	Status      ItemStatus `gorm:"type:varchar(20);default:'selling'" json:"status"`
	CategoryID  *uint      `json:"category_id"`
	Category    Category   `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	Images      []ItemImage `gorm:"foreignKey:ItemID" json:"images,omitempty"`
	Tags        []Tag      `gorm:"many2many:item_tags;" json:"tags,omitempty"`
	LikeCount   int        `gorm:"-" json:"like_count"`
	IsLiked     bool       `gorm:"-" json:"is_liked"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// 商品画像テーブル
type ItemImage struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	ItemID       uint   `gorm:"not null" json:"item_id"`
	URL          string `gorm:"not null" json:"url"`
	DisplayOrder int    `gorm:"default:0" json:"display_order"`
}

// いいねテーブル
type Like struct {
	UserID    uint      `gorm:"primaryKey" json:"user_id"`
	ItemID    uint      `gorm:"primaryKey" json:"item_id"`
	CreatedAt time.Time `json:"created_at"`
}
