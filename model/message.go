package model

import "time"

// メッセージテーブル（ユーザー間DM）
type Message struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	SenderID   uint      `gorm:"not null;index" json:"sender_id"`
	Sender     User      `gorm:"foreignKey:SenderID" json:"sender,omitempty"`
	ReceiverID uint      `gorm:"not null;index" json:"receiver_id"`
	Receiver   User      `gorm:"foreignKey:ReceiverID" json:"receiver,omitempty"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	IsRead     bool      `gorm:"default:false" json:"is_read"`
	CreatedAt  time.Time `json:"created_at"`
}
