package router

import (
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	"hackathon-backend/handler"
	mw "hackathon-backend/middleware"
)

// Setup はEchoのルーティングを設定する
func Setup(e *echo.Echo) {
	// グローバルミドルウェア
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())

	// ローカル開発用: アップロード画像を静的配信
	e.Static("/uploads", "uploads")
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Content-Type", "Authorization"},
	}))

	api := e.Group("/api")

	// 認証不要ルート
	auth := api.Group("/auth")
	auth.POST("/register", handler.Register)
	auth.POST("/login", handler.Login)

	// 公開ルート（認証任意：いいね状態などを返す）
	public := api.Group("")
	public.Use(mw.OptionalJWTAuth)
	public.GET("/items", handler.GetItems)
	public.GET("/items/:id", handler.GetItem)
	public.GET("/users/:id", handler.GetUser)
	public.GET("/tags/trend", handler.GetTrendTags)
	public.GET("/categories", handler.GetCategories)

	// 認証必須ルート
	private := api.Group("")
	private.Use(mw.JWTAuth)

	// 自分の情報
	private.GET("/me", handler.Me)
	private.PUT("/me", handler.UpdateProfile)

	// 画像アップロード
	private.POST("/upload", handler.UploadImage)

	// AIアシスト（商品説明生成・質問応答）
	private.POST("/ai/assist", handler.AIAssist)
	private.POST("/ai/qa", handler.AIQnA)

	// 商品
	private.POST("/items", handler.CreateItem)
	private.DELETE("/items/:id", handler.DeleteItem)
	private.POST("/items/:id/like", handler.ToggleLike)
	private.POST("/items/:id/buy", handler.BuyItem)
	private.GET("/items/:id/offers", handler.GetItemOffers)
	private.POST("/items/:id/offers", handler.CreateOffer)

	// オファー返答
	private.POST("/offers/:id/respond", handler.RespondOffer)

	// フォロー
	private.POST("/users/:id/follow", handler.Follow)
	private.DELETE("/users/:id/follow", handler.Unfollow)
	private.GET("/users/:id/followers", handler.GetFollowers)
	private.GET("/users/:id/following", handler.GetFollowing)

	// 取引・レビュー
	private.GET("/transactions", handler.GetMyTransactions)
	private.POST("/transactions/:id/complete", handler.CompleteTransaction)
	private.POST("/transactions/:id/review", handler.CreateReview)

	// DM（ダイレクトメッセージ）
	private.GET("/messages", handler.GetConversations)
	private.GET("/messages/:id", handler.GetMessages)
	private.POST("/messages/:id", handler.SendMessage)
}
