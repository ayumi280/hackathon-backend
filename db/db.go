package db

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"hackathon-backend/model"
)

var DB *gorm.DB

// Init はMySQLに接続しオートマイグレーションを実行する
func Init() {
	user := os.Getenv("MYSQL_USER")
	pwd := os.Getenv("MYSQL_PWD")
	host := os.Getenv("MYSQL_HOST")
	database := os.Getenv("MYSQL_DATABASE")

	dsn := fmt.Sprintf(
		"%s:%s@%s/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, pwd, host, database,
	)

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("DB接続失敗: %v", err)
	}

	// 全テーブルをオートマイグレーション
	err = DB.AutoMigrate(
		&model.User{},
		&model.Category{},
		&model.Item{},
		&model.ItemImage{},
		&model.Tag{},
		&model.Like{},
		&model.Offer{},
		&model.Transaction{},
		&model.Review{},
		&model.Follow{},
		&model.Message{},
	)
	if err != nil {
		log.Fatalf("マイグレーション失敗: %v", err)
	}

	log.Println("DB接続・マイグレーション完了")
}
