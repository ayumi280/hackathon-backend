package model

import "time"

// タグテーブル（トレンドスコア付き）
type Tag struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"unique;not null" json:"name"`
	TrendScore int       `gorm:"default:0" json:"trend_score"`
	CreatedAt  time.Time `json:"created_at"`
}
