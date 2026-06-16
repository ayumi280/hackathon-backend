package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"hackathon-backend/db"
	"hackathon-backend/model"
)

// Follow は指定ユーザーをフォローする
func Follow(c echo.Context) error {
	followerID := c.Get("user_id").(uint)
	followingID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	if followerID == uint(followingID) {
		return echo.NewHTTPError(http.StatusBadRequest, "自分自身をフォローできません")
	}

	// 対象ユーザーの存在確認
	var target model.User
	if result := db.DB.First(&target, followingID); result.Error != nil {
		return echo.NewHTTPError(http.StatusNotFound, "ユーザーが見つかりません")
	}

	// 既にフォロー済みかチェック
	var existing model.Follow
	result := db.DB.Where("follower_id = ? AND following_id = ?", followerID, followingID).First(&existing)
	if result.Error == nil {
		return echo.NewHTTPError(http.StatusConflict, "既にフォローしています")
	}

	follow := model.Follow{
		FollowerID:  followerID,
		FollowingID: uint(followingID),
	}
	db.DB.Create(&follow)
	return c.JSON(http.StatusCreated, map[string]bool{"following": true})
}

// Unfollow は指定ユーザーのフォローを解除する
func Unfollow(c echo.Context) error {
	followerID := c.Get("user_id").(uint)
	followingID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	result := db.DB.Where("follower_id = ? AND following_id = ?", followerID, followingID).
		Delete(&model.Follow{})
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "フォロー関係が見つかりません")
	}
	return c.JSON(http.StatusOK, map[string]bool{"following": false})
}

// GetFollowers はフォロワー一覧を返す
func GetFollowers(c echo.Context) error {
	userID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var follows []model.Follow
	db.DB.Preload("Follower").Where("following_id = ?", userID).Find(&follows)

	users := make([]model.User, 0, len(follows))
	for _, f := range follows {
		users = append(users, f.Follower)
	}
	return c.JSON(http.StatusOK, users)
}

// GetFollowing はフォロー中のユーザー一覧を返す
func GetFollowing(c echo.Context) error {
	userID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var follows []model.Follow
	db.DB.Preload("Following").Where("follower_id = ?", userID).Find(&follows)

	users := make([]model.User, 0, len(follows))
	for _, f := range follows {
		users = append(users, f.Following)
	}
	return c.JSON(http.StatusOK, users)
}
