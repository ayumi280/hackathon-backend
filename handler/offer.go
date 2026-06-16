package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"hackathon-backend/db"
	"hackathon-backend/model"
)

// CreateOffer は購入者がオファー（値下げ交渉）を送る
func CreateOffer(c echo.Context) error {
	buyerID := c.Get("user_id").(uint)
	itemID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	// 自分の出品商品にはオファー不可
	var item model.Item
	if result := db.DB.First(&item, itemID); result.Error != nil {
		return echo.NewHTTPError(http.StatusNotFound, "商品が見つかりません")
	}
	if item.SellerID == buyerID {
		return echo.NewHTTPError(http.StatusBadRequest, "自分の商品にオファーはできません")
	}
	if item.Status != model.ItemStatusSelling {
		return echo.NewHTTPError(http.StatusBadRequest, "この商品は販売中ではありません")
	}

	var req struct {
		OfferedPrice int    `json:"offered_price"`
		Message      string `json:"message"`
	}
	if err := c.Bind(&req); err != nil || req.OfferedPrice <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "オファー価格を入力してください")
	}

	offer := model.Offer{
		ItemID:       uint(itemID),
		BuyerID:      buyerID,
		OfferedPrice: req.OfferedPrice,
		Message:      req.Message,
		Status:       model.OfferStatusPending,
	}
	db.DB.Create(&offer)
	db.DB.Preload("Buyer").Preload("Item").First(&offer, offer.ID)

	return c.JSON(http.StatusCreated, offer)
}

// RespondOffer は出品者がオファーに承諾/拒否/カウンター提案する
func RespondOffer(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	offerID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var offer model.Offer
	if result := db.DB.Preload("Item").First(&offer, offerID); result.Error != nil {
		return echo.NewHTTPError(http.StatusNotFound, "オファーが見つかりません")
	}
	if offer.Item.SellerID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "権限がありません")
	}
	if offer.Status != model.OfferStatusPending {
		return echo.NewHTTPError(http.StatusBadRequest, "既に処理済みのオファーです")
	}

	var req struct {
		Action       string `json:"action"`        // "accept" / "reject" / "counter"
		CounterPrice int    `json:"counter_price"` // counterの場合に必要
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエスト形式が不正です")
	}

	switch req.Action {
	case "accept":
		offer.Status = model.OfferStatusAccepted
		// 取引を作成し商品ステータスを更新
		tx := model.Transaction{
			ItemID:     offer.ItemID,
			BuyerID:    offer.BuyerID,
			SellerID:   userID,
			FinalPrice: offer.OfferedPrice,
			Status:     model.TransactionStatusPending,
		}
		db.DB.Create(&tx)
		db.DB.Model(&offer.Item).Update("status", model.ItemStatusTrading)

	case "reject":
		offer.Status = model.OfferStatusRejected

	case "counter":
		if req.CounterPrice <= 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "カウンター価格を入力してください")
		}
		offer.Status = model.OfferStatusCountered
		offer.CounterPrice = &req.CounterPrice

	default:
		return echo.NewHTTPError(http.StatusBadRequest, "actionはaccept/reject/counterのいずれかです")
	}

	db.DB.Save(&offer)
	return c.JSON(http.StatusOK, offer)
}

// GetItemOffers は商品へのオファー一覧を返す（出品者のみ）
func GetItemOffers(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	itemID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var item model.Item
	if result := db.DB.First(&item, itemID); result.Error != nil {
		return echo.NewHTTPError(http.StatusNotFound, "商品が見つかりません")
	}
	if item.SellerID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "権限がありません")
	}

	var offers []model.Offer
	db.DB.Preload("Buyer").Where("item_id = ?", itemID).Order("created_at DESC").Find(&offers)
	return c.JSON(http.StatusOK, offers)
}

// BuyItem は即時購入（オファーなし）の取引を作成する
func BuyItem(c echo.Context) error {
	buyerID := c.Get("user_id").(uint)
	itemID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var item model.Item
	if result := db.DB.First(&item, itemID); result.Error != nil {
		return echo.NewHTTPError(http.StatusNotFound, "商品が見つかりません")
	}
	if item.SellerID == buyerID {
		return echo.NewHTTPError(http.StatusBadRequest, "自分の商品は購入できません")
	}
	if item.Status != model.ItemStatusSelling {
		return echo.NewHTTPError(http.StatusBadRequest, "この商品は購入できません")
	}

	tx := model.Transaction{
		ItemID:     uint(itemID),
		BuyerID:    buyerID,
		SellerID:   item.SellerID,
		FinalPrice: item.Price,
		Status:     model.TransactionStatusPending,
	}
	db.DB.Create(&tx)
	db.DB.Model(&item).Update("status", model.ItemStatusTrading)

	return c.JSON(http.StatusCreated, tx)
}
