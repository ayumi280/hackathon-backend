package model

import "time"

// ユーザーテーブル
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Email        string    `gorm:"unique;not null" json:"email"`
	PasswordHash string    `gorm:"not null" json:"-"`
	Username     string    `gorm:"unique;not null" json:"username"`
	AvatarURL    string    `json:"avatar_url"`
	Bio          string    `json:"bio"`
	Rating       float64   `gorm:"default:0" json:"rating"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
