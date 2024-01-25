package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"xicserver/handlers"

	"github.com/gin-gonic/gin"
)

const maxRetries = 3                  // 最大再試行回数
const retryInterval = 5 * time.Second // 再試行間の待機時間

type Config struct {
	DBHost     string `json:"db_host"`
	DBUser     string `json:"db_user"`
	DBPassword string `json:"db_password"`
	DBName     string `json:"db_name"`
	DBSSLMode  string `json:"db_sslmode"`
}

var (
	logger *zap.Logger
	sqlDB  *sql.DB // グローバル変数として定義
)

func LoadConfig(filename string) (Config, error) {
	var config Config
	configFile, err := os.Open(filename)
	if err != nil {
		return config, err
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	return config, err
}

func init() {
	var err error
	// Zapのロガーを設定
	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
}

func initDB(config Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s dbname=%s password=%s sslmode=%s",
		config.DBHost, config.DBUser, config.DBName, config.DBPassword, config.DBSSLMode)

	var err error
	for i := 0; i <= maxRetries; i++ {
		var gormDB *gorm.DB
		gormDB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			sqlDB, err = gormDB.DB()
			if err == nil {
				return gormDB, nil
			}
		}

		logger.Error("データベース接続のリトライ", zap.Int("retry", i), zap.Error(err))
		time.Sleep(retryInterval)
	}

	return nil, fmt.Errorf("データベース接続に失敗しました: %v", err)

	// host := os.Getenv("DB_HOST")
	// user := os.Getenv("DB_USER")
	// dbname := os.Getenv("DB_NAME")
	// password := os.Getenv("DB_PASSWORD")
	// sslmode := os.Getenv("DB_SSLMODE")

	// dsn := "host=" + host + " user=" + user + " dbname=" + dbname + " password=" + password + " sslmode=" + sslmode

	// var err error
	// for i := 0; i <= maxRetries; i++ {
	// 	var gormDB *gorm.DB
	// 	gormDB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	// 	if err == nil {
	// 		sqlDB, err = gormDB.DB()
	// 		if err == nil {
	// 			return gormDB, nil
	// 		}
	// 	}

	// 	logger.Error("データベース接続のリトライ", zap.Int("retry", i), zap.Error(err))
	// 	time.Sleep(retryInterval)
	// }

	// return nil, fmt.Errorf("データベース接続に失敗しました: %v", err)
}

// User モデルの定義
type User struct {
	gorm.Model
	UserID             string `gorm:"unique;not null"`
	SubscriptionStatus string `gorm:"not null"`
	ValidRoomCount     int    `gorm:"not null"`
}

// GameRoom モデルの定義
type GameRoom struct {
	gorm.Model
	GameRoomID       uint
	Platform         string `gorm:"not null"`
	AccountName      string `gorm:"not null"`
	MatchType        string `gorm:"not null"`
	UnfairnessDegree int    `gorm:"not null"`
	GameState        string `gorm:"not null"`
	CreationTime     int64  `gorm:"not null"`
	LastActivityTime int64
	FinishTime       int64
	StartTime        int64
	RoomTheme        string
}

func main() {
	logger.Info("アプリケーションが起動しました。")

	defer sqlDB.Close()
	defer logger.Sync()

	config, err := LoadConfig("config.json")
	if err != nil {
		logger.Fatal("設定ファイルの読み込みに失敗しました", zap.Error(err))
	}

	// データベース接続の初期化
	_, err = initDB(config) // gormDB は現在使用しないため、変数に格納しない
	if err != nil {
		logger.Error("データベースの初期化に失敗しました", zap.Error(err))
		return
	}

	// Ginエンジンの初期化
	r := gin.Default()
	// ルーティングの設定
	r.POST("/gameroom", handlers.CreateGameRoom)

	r.Run() // デフォルトでは ":8080" でリスニングします
}
