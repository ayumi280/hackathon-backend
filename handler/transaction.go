package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"hackathon-backend/db"
	"hackathon-backend/model"
)

// GetMyTransactions はログインユーザーの取引一覧（購入・販売）を返す
func GetMyTransactions(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	role := c.QueryParam("role") // "buyer" or "seller"

	var txs []model.Transaction
	query := db.DB.Preload("Item.Images").Preload("Buyer").Preload("Seller")

	switch role {
	case "buyer":
		query = query.Where("buyer_id = ?", userID)
	case "seller":
		query = query.Where("seller_id = ?", userID)
	default:
		query = query.Where("buyer_id = ? OR seller_id = ?", userID, userID)
	}

	query.Order("created_at DESC").Find(&txs)
	return c.JSON(http.StatusOK, txs)
}

// CompleteTransaction は取引を完了状態にする（受取評価）
func CompleteTransaction(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	txID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var tx model.Transaction
	if result := db.DB.First(&tx, txID); result.Error != nil {
		return echo.NewHTTPError(http.StatusNotFound, "取引が見つかりません")
	}
	// 購入者のみ受取完了できる
	if tx.BuyerID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "権限がありません")
	}
	if tx.Status != model.TransactionStatusPending && tx.Status != model.TransactionStatusShipping {
		return echo.NewHTTPError(http.StatusBadRequest, "この取引は完了できません")
	}

	db.DB.Model(&tx).Update("status", model.TransactionStatusCompleted)
	// 商品ステータスを売り切れに更新
	db.DB.Model(&model.Item{}).Where("id = ?", tx.ItemID).Update("status", model.ItemStatusSold)

	return c.JSON(http.StatusOK, tx)
}

// CreateReview は取引完了後にレビューを投稿する
func CreateReview(c echo.Context) error {
	reviewerID := c.Get("user_id").(uint)
	txID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var tx model.Transaction
	if result := db.DB.First(&tx, txID); result.Error != nil {
		return echo.NewHTTPError(http.StatusNotFound, "取引が見つかりません")
	}
	if tx.Status != model.TransactionStatusCompleted {
		return echo.NewHTTPError(http.StatusBadRequest, "取引完了後にレビューできます")
	}
	// 購入者または出品者のみ
	if tx.BuyerID != reviewerID && tx.SellerID != reviewerID {
		return echo.NewHTTPError(http.StatusForbidden, "権限がありません")
	}

	// レビュー対象（自分以外）を決定
	revieweeID := tx.SellerID
	if reviewerID == tx.SellerID {
		revieweeID = tx.BuyerID
	}

	var req struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if err := c.Bind(&req); err != nil || req.Rating < 1 || req.Rating > 5 {
		return echo.NewHTTPError(http.StatusBadRequest, "評価（1〜5）を入力してください")
	}

	// 重複レビュー防止
	var existing model.Review
	if result := db.DB.Where("transaction_id = ? AND reviewer_id = ?", txID, reviewerID).First(&existing); result.Error == nil {
		return echo.NewHTTPError(http.StatusConflict, "既にレビュー済みです")
	}

	review := model.Review{
		TransactionID: uint(txID),
		ReviewerID:    reviewerID,
		RevieweeID:    revieweeID,
		Rating:        req.Rating,
		Comment:       req.Comment,
	}
	db.DB.Create(&review)

	// レビュー対象ユーザーの平均評価を更新
	var avgRating float64
	db.DB.Model(&model.Review{}).Where("reviewee_id = ?", revieweeID).
		Select("AVG(rating)").Scan(&avgRating)
	db.DB.Model(&model.User{}).Where("id = ?", revieweeID).Update("rating", avgRating)

	return c.JSON(http.StatusCreated, review)
}
