package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// データベース設定
type Config struct {
	DBHost     string `json:"db_host"`
	DBUser     string `json:"db_user"`
	DBPassword string `json:"db_password"`
	DBName     string `json:"db_name"`
	DBSSLMode  string `json:"db_sslmode"`
}

// User モデルの定義
type User struct {
	gorm.Model
	SubscriptionStatus string `gorm:"not null"`
	ValidRoomCount     int    `gorm:"not null"`
}

// GameRoom モデルの定義
type GameRoom struct {
	gorm.Model
	Platform         string `gorm:"not null"`
	AccountName      string `gorm:"not null"`
	MatchType        string `gorm:"not null"`
	UnfairnessDegree int    `gorm:"not null"`
	GameState        string `gorm:"not null"`
	FinishTime       int64
	StartTime        int64
	RoomTheme        string
}

var logger *zap.Logger

func init() {
	// Zapのロガー設定
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
}

// マイグレーションを実行する関数
func AutoMigrateDB(db *gorm.DB) {
	err := db.AutoMigrate(&User{}, &GameRoom{})
	if err != nil {
		logger.Error("Error migrating tables", zap.Error(err))
	} else {
		logger.Info("User and GameRoom tables created successfully")
	}
}

func main() {
	// config.jsonからデータベースの設定を読み込み
	config := Config{}
	configFile, err := ioutil.ReadFile("config.json")
	if err != nil {
		logger.Fatal("Error reading config file", zap.Error(err))
		return
	}
	err = json.Unmarshal(configFile, &config)
	if err != nil {
		logger.Fatal("Error parsing config file", zap.Error(err))
		return
	}

	dsn := fmt.Sprintf("host=%s user=%s dbname=%s password=%s sslmode=%s", config.DBHost, config.DBUser, config.DBName, config.DBPassword, config.DBSSLMode)
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("Failed to connect to database", zap.Error(err))
		return
	}

	// データベース接続の取得
	sqlDB, err := gormDB.DB()
	if err != nil {
		logger.Error("Failed to get SQLDB", zap.Error(err))
		return
	}
	defer sqlDB.Close()

	// マイグレーション実行
	AutoMigrateDB(gormDB)
}
