package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"hackathon-backend/db"
	"hackathon-backend/model"
)

// GetTrendTags はトレンドスコア順にタグ一覧を返す
func GetTrendTags(c echo.Context) error {
	var tags []model.Tag
	db.DB.Order("trend_score DESC").Limit(20).Find(&tags)
	return c.JSON(http.StatusOK, tags)
}

// GetCategories はカテゴリ一覧を返す
func GetCategories(c echo.Context) error {
	var categories []model.Category
	db.DB.Find(&categories)
	return c.JSON(http.StatusOK, categories)
}
