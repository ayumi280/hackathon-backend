package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/storage"
	"github.com/labstack/echo/v4"
	"google.golang.org/api/option"

	"hackathon-backend/db"
	"hackathon-backend/model"
)

// GetItems はフィード（新着・フォロー中・タグ別）を取得する
func GetItems(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	limit := 20
	offset := (page - 1) * limit

	query := db.DB.Model(&model.Item{}).
		Preload("Images").
		Preload("Seller").
		Preload("Tags").
		Where("status = 'selling'")

	// タグフィルター
	if tag := c.QueryParam("tag"); tag != "" {
		query = query.Joins("JOIN item_tags ON item_tags.item_id = items.id").
			Joins("JOIN tags ON tags.id = item_tags.tag_id").
			Where("tags.name = ?", tag)
	}

	// カテゴリフィルター
	if catID := c.QueryParam("category_id"); catID != "" {
		query = query.Where("category_id = ?", catID)
	}

	// フォロー中フィード
	if c.QueryParam("feed") == "following" {
		userID := c.Get("user_id").(uint)
		var followingIDs []uint
		db.DB.Model(&model.Follow{}).Where("follower_id = ?", userID).
			Pluck("following_id", &followingIDs)
		query = query.Where("seller_id IN ?", followingIDs)
	}

	// キーワード検索
	if keyword := c.QueryParam("q"); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR description LIKE ?", like, like)
	}

	// ソート順
	switch c.QueryParam("sort") {
	case "trend":
		// いいね数の多い順（trend）
		query = query.Joins("LEFT JOIN (SELECT item_id, COUNT(*) as lc FROM likes GROUP BY item_id) l ON l.item_id = items.id").
			Order("COALESCE(l.lc, 0) DESC")
	case "price_asc":
		query = query.Order("price ASC")
	case "price_desc":
		query = query.Order("price DESC")
	default:
		query = query.Order("items.created_at DESC")
	}

	var items []model.Item
	query.Offset(offset).Limit(limit).Find(&items)

	// いいね数とログインユーザーのいいね状態を付加
	if loginUserID, ok := c.Get("user_id").(uint); ok {
		for i := range items {
			var cnt int64
			db.DB.Model(&model.Like{}).Where("item_id = ?", items[i].ID).Count(&cnt)
			items[i].LikeCount = int(cnt)
			var liked int64
			db.DB.Model(&model.Like{}).Where("item_id = ? AND user_id = ?", items[i].ID, loginUserID).Count(&liked)
			items[i].IsLiked = liked > 0
		}
	}

	return c.JSON(http.StatusOK, items)
}

// GetItem は商品詳細を取得する
func GetItem(c echo.Context) error {
	itemID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "IDが不正です")
	}

	var item model.Item
	result := db.DB.Preload("Images").Preload("Seller").Preload("Tags").
		Preload("Category").First(&item, itemID)
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusNotFound, "商品が見つかりません")
	}

	// いいね数
	var cnt int64
	db.DB.Model(&model.Like{}).Where("item_id = ?", itemID).Count(&cnt)
	item.LikeCount = int(cnt)

	if loginUserID, ok := c.Get("user_id").(uint); ok {
		var liked int64
		db.DB.Model(&model.Like{}).Where("item_id = ? AND user_id = ?", itemID, loginUserID).Count(&liked)
		item.IsLiked = liked > 0
	}

	return c.JSON(http.StatusOK, item)
}

// CreateItem は商品を出品する
func CreateItem(c echo.Context) error {
	userID := c.Get("user_id").(uint)

	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Price       int      `json:"price"`
		CategoryID  *uint    `json:"category_id"`
		Tags        []string `json:"tags"`
		ImageURLs   []string `json:"image_urls"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエスト形式が不正です")
	}
	if req.Title == "" || req.Price <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "タイトルと価格は必須です")
	}

	item := model.Item{
		SellerID:    userID,
		Title:       req.Title,
		Description: req.Description,
		Price:       req.Price,
		CategoryID:  req.CategoryID,
		Status:      model.ItemStatusSelling,
	}

	// タグを取得または作成
	var tags []model.Tag
	for _, tagName := range req.Tags {
		var tag model.Tag
		db.DB.Where(model.Tag{Name: tagName}).FirstOrCreate(&tag)
		// タグのトレンドスコアを加算
		db.DB.Model(&tag).UpdateColumn("trend_score", tag.TrendScore+1)
		tags = append(tags, tag)
	}
	item.Tags = tags

	if result := db.DB.Create(&item); result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "出品に失敗しました")
	}

	// 画像URLを保存
	for i, url := range req.ImageURLs {
		img := model.ItemImage{ItemID: item.ID, URL: url, DisplayOrder: i}
		db.DB.Create(&img)
	}

	db.DB.Preload("Images").Preload("Tags").First(&item, item.ID)
	return c.JSON(http.StatusCreated, item)
}

// DeleteItem は自分の商品を削除する
func DeleteItem(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	itemID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var item model.Item
	if result := db.DB.First(&item, itemID); result.Error != nil {
		return echo.NewHTTPError(http.StatusNotFound, "商品が見つかりません")
	}
	if item.SellerID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "権限がありません")
	}

	db.DB.Delete(&item)
	return c.JSON(http.StatusOK, map[string]string{"message": "削除しました"})
}

// ToggleLike はいいねを付けたり外したりする
func ToggleLike(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	itemID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var like model.Like
	result := db.DB.Where("user_id = ? AND item_id = ?", userID, itemID).First(&like)
	if result.Error != nil {
		// いいね追加
		db.DB.Create(&model.Like{UserID: userID, ItemID: uint(itemID)})
		return c.JSON(http.StatusOK, map[string]bool{"liked": true})
	}
	// いいね解除
	db.DB.Delete(&like)
	return c.JSON(http.StatusOK, map[string]bool{"liked": false})
}

// UploadImage は画像をGCSにアップロードしURLを返す
func UploadImage(c echo.Context) error {
	userID := c.Get("user_id").(uint)

	file, err := c.FormFile("image")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "画像ファイルが必要です")
	}

	src, err := file.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "ファイルオープン失敗")
	}
	defer src.Close()

	bucket := os.Getenv("GCS_BUCKET")
	if bucket == "" {
		return echo.NewHTTPError(http.StatusInternalServerError, "ストレージ設定エラー")
	}

	ctx := context.Background()

	// ローカル開発（K_SERVICE未設定）かつ認証情報がない場合はプレースホルダーURLを返す
	isLocal := os.Getenv("K_SERVICE") == ""
	keyPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if isLocal && keyPath == "" {
		placeholderURL := fmt.Sprintf("https://placehold.co/400x400?text=%s", file.Filename)
		return c.JSON(http.StatusOK, map[string]string{"url": placeholderURL})
	}

	var client *storage.Client
	if keyPath != "" {
		client, err = storage.NewClient(ctx, option.WithCredentialsFile(keyPath))
	} else {
		client, err = storage.NewClient(ctx)
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "ストレージ接続失敗")
	}
	defer client.Close()

	objectName := fmt.Sprintf("items/%d/%d_%s", userID, time.Now().UnixNano(), file.Filename)
	wc := client.Bucket(bucket).Object(objectName).NewWriter(ctx)
	wc.ContentType = file.Header.Get("Content-Type")

	buf := make([]byte, file.Size)
	if _, err = src.Read(buf); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "ファイル読み込み失敗")
	}
	if _, err = wc.Write(buf); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "アップロード失敗")
	}
	if err = wc.Close(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "アップロード完了失敗")
	}

	publicURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucket, objectName)
	return c.JSON(http.StatusOK, map[string]string{"url": publicURL})
}
