// package main

// import (
// 	"fmt"
// 	"os"

// 	"go.uber.org/zap"
// 	"gorm.io/driver/postgres"
// 	"gorm.io/gorm"
// )

// // User モデルの定義
// type User struct {
// 	gorm.Model
// 	SubscriptionStatus string `gorm:"not null"`
// 	ValidRoomCount     int    `gorm:"not null"`
// }

// // GameRoom モデルの定義
// type GameRoom struct {
// 	gorm.Model
// 	Platform         string `gorm:"not null"`
// 	AccountName      string `gorm:"not null"`
// 	MatchType        string `gorm:"not null"`
// 	UnfairnessDegree int    `gorm:"not null"`
// 	GameState        string `gorm:"not null"`
// 	CreationTime     int64  `gorm:"not null"`
// 	LastActivityTime int64
// 	FinishTime       int64
// 	StartTime        int64
// 	RoomTheme        string
// }

// var logger *zap.Logger

// func init() {
// 	// Zapのロガー設定
// 	var err error
// 	logger, err = zap.NewProduction()
// 	if err != nil {
// 		panic(err)
// 	}
// }

// // マイグレーションを実行する関数
// func AutoMigrateDB(db *gorm.DB) {
// 	err := db.AutoMigrate(&User{}, &GameRoom{})
// 	if err != nil {
// 		logger.Error("Error migrating tables", zap.Error(err))
// 	} else {
// 		logger.Info("User and GameRoom tables created successfully")
// 	}
// }

// func main() {
// 	// 環境変数からデータベースの接続情報を取得
// 	host := os.Getenv("DB_HOST")
// 	user := os.Getenv("DB_USER")
// 	dbname := os.Getenv("DB_NAME")
// 	password := os.Getenv("DB_PASSWORD")
// 	sslmode := os.Getenv("DB_SSLMODE")

// 	dsn := fmt.Sprintf("host=%s user=%s dbname=%s password=%s sslmode=%s", host, user, dbname, password, sslmode)
// 	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
// 	if err != nil {
// 		logger.Error("Failed to connect to database", zap.Error(err))
// 		return
// 	}

// 	// データベース接続の取得
// 	sqlDB, err := gormDB.DB()
// 	if err != nil {
// 		logger.Error("Failed to get SQLDB", zap.Error(err))
// 		return
// 	}
// 	defer sqlDB.Close()

// 	// マイグレーション実行
// 	AutoMigrateDB(gormDB)
// }
