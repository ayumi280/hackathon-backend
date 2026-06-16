package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"hackathon-backend/db"
	"hackathon-backend/model"
)

// GetUser はユーザープロフィールを取得する
func GetUser(c echo.Context) error {
	targetID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "IDが不正です")
	}

	var user model.User
	if result := db.DB.First(&user, targetID); result.Error != nil {
		return echo.NewHTTPError(http.StatusNotFound, "ユーザーが見つかりません")
	}

	// フォロワー数・フォロー数を取得
	var followerCount, followingCount int64
	db.DB.Model(&model.Follow{}).Where("following_id = ?", targetID).Count(&followerCount)
	db.DB.Model(&model.Follow{}).Where("follower_id = ?", targetID).Count(&followingCount)

	// ログイン中ユーザーがフォローしているか確認
	isFollowing := false
	if loginUserID, ok := c.Get("user_id").(uint); ok && loginUserID != 0 {
		var cnt int64
		db.DB.Model(&model.Follow{}).
			Where("follower_id = ? AND following_id = ?", loginUserID, targetID).
			Count(&cnt)
		isFollowing = cnt > 0
	}

	// 出品中の商品を取得
	var items []model.Item
	db.DB.Preload("Images").Where("seller_id = ? AND status = 'selling'", targetID).
		Order("created_at DESC").Limit(20).Find(&items)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"user":           user,
		"follower_count": followerCount,
		"following_count": followingCount,
		"is_following":   isFollowing,
		"items":          items,
	})
}

// UpdateProfile はログイン中ユーザーのプロフィールを更新する
func UpdateProfile(c echo.Context) error {
	userID := c.Get("user_id").(uint)

	var req struct {
		Username  string `json:"username"`
		Bio       string `json:"bio"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエスト形式が不正です")
	}

	var user model.User
	if result := db.DB.First(&user, userID); result.Error != nil {
		return echo.NewHTTPError(http.StatusNotFound, "ユーザーが見つかりません")
	}

	if req.Username != "" {
		user.Username = req.Username
	}
	user.Bio = req.Bio
	if req.AvatarURL != "" {
		user.AvatarURL = req.AvatarURL
	}

	db.DB.Save(&user)
	return c.JSON(http.StatusOK, user)
}
