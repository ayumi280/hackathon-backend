package main

import (
	"context"
	"log"
	"os"

	"github.com/labstack/echo/v4"

	"hackathon-backend/config"
	"hackathon-backend/db"
	"hackathon-backend/router"
)

func main() {
	ctx := context.Background()

	// 環境に応じてSecret Manager(.envフォールバック)からシークレットを読み込む
	config.Load(ctx)

	// DB接続・マイグレーション
	db.Init()

	e := echo.New()
	router.Setup(e)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("サーバー起動: :%s", port)
	if err := e.Start(":" + port); err != nil {
		log.Fatalf("サーバー起動失敗: %v", err)
	}
}
