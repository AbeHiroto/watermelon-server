package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"xicserver/handlers"
	"xicserver/models"

	"github.com/gin-gonic/gin"
)

var logger *zap.Logger

func main() {
	logger.Info("アプリケーションが起動しました。")
	defer logger.Sync() //ロガーを適切に閉じます。

	config, err := LoadConfig("config.json")
	if err != nil {
		logger.Fatal("設定ファイルの読み込みに失敗しました", zap.Error(err))
	}

	db, err := initDB(config)
	if err != nil {
		logger.Fatal("データベースの初期化に失敗しました", zap.Error(err))
	}
	//defer db.Close() // データベース接続のクローズ（GORMv2からは不要）

	router := gin.Default()
	router.Use(gin.Recovery(), Logger(logger))
	router.POST("/create", func(c *gin.Context) {
		handlers.RoomCreate(c, db) // RoomCreate ハンドラに db を渡す
	})

	router.Run() // HTTPサーバー用。デフォルトポートは ":8080"

	// // HTTPSサーバーの起動
	// err = router.RunTLS(":443", "path/to/cert.pem", "path/to/key.pem")
	// if err != nil {
	// 	logger.Fatal("Failed to run HTTPS server: ", zap.Error(err))
	// }
}

func init() {
	var err error
	// logger というグローバル変数に、Zapロギングライブラリを使用して生成された
	//新しいプロダクションロガーが割り当てられます。
	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
}

func LoadConfig(filename string) (models.Config, error) {
	var config models.Config
	configFile, err := os.Open(filename)
	if err != nil {
		return config, err
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	return config, err
}

// リクエストのログ取得
func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		latency := time.Since(start)
		logger.Info("request",
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
		)
	}
}

func initDB(config models.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s dbname=%s password=%s sslmode=%s",
		config.DBHost, config.DBUser, config.DBName, config.DBPassword, config.DBSSLMode)

	var err error // err変数をループの外で定義
	for i := 0; i <= maxRetries; i++ {
		var gormDB *gorm.DB
		gormDB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			return gormDB, nil
		}

		logger.Error("データベース接続のリトライ", zap.Int("retry", i), zap.Error(err))
		time.Sleep(retryInterval)
	}

	return nil, fmt.Errorf("データベース接続に失敗しました: %v", err)
}

const maxRetries = 3                  // 最大再試行回数
const retryInterval = 5 * time.Second // 再試行間の待機時間
