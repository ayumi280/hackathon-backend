package model

import "time"

// フォローテーブル（フォロワー/フォロー中）
type Follow struct {
	FollowerID  uint      `gorm:"primaryKey" json:"follower_id"`
	FollowingID uint      `gorm:"primaryKey" json:"following_id"`
	Follower    User      `gorm:"foreignKey:FollowerID" json:"follower,omitempty"`
	Following   User      `gorm:"foreignKey:FollowingID" json:"following,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
