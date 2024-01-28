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
	"xicserver/middlewares"

	//"xicserver/models"

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
}

func LoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// リクエスト処理前のログ記録
		start := time.Now()
		path := c.Request.URL.Path

		// 次の処理へ
		c.Next()

		// リクエスト処理後のログ記録
		latency := time.Since(start)
		logger.Info("request",
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
		)
	}
}

func main() {
	logger.Info("アプリケーションが起動しました。")

	config, err := LoadConfig("config.json")
	if err != nil {
		logger.Fatal("設定ファイルの読み込みに失敗しました", zap.Error(err))
	}

	// データベース接続の初期化
	db, err := initDB(config)
	if err != nil {
		logger.Error("データベースの初期化に失敗しました", zap.Error(err))
		return
	}

	router := gin.Default()
	router.Use(gin.Recovery())                     // パニックからの回復を行う標準ミドルウェア
	router.Use(LoggerMiddleware(logger))           // ロギングミドルウェアの登録
	router.Use(middlewares.AuthMiddleware(logger)) // 認証ミドルウェアの適用

	// ユーザー登録とトークン生成のルートを設定
	router.POST("/auth/register", handlers.RegisterUser(db))
	router.POST("/auth/token", handlers.GenerateToken(db))

	authGroup := router.Group("/").Use(middlewares.AuthMiddleware(logger))
	{
		authGroup.POST("/gameroom", handlers.CreateGameRoom)
		// その他の保護されたルート
	}

	// ルーティングの設定
	router.POST("/gameroom", handlers.CreateGameRoom)
	router.GET("/someEndpoint", func(c *gin.Context) { // エンドポイントの処理
	})

	defer sqlDB.Close() //アプリケーション終了時にデータベース接続と、
	defer logger.Sync() //ロガーを適切に閉じます。

	router.Run() // デフォルトでは ":8080" でリスニングします
}
