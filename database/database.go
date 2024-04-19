package database

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"xicserver/models"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// LoadConfig loads the configuration from config.json
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

func InitPostgreSQL(config models.Config, logger *zap.Logger) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s dbname=%s password=%s sslmode=%s",
		config.DBHost, config.DBUser, config.DBName, config.DBPassword, config.DBSSLMode)

	const maxRetries = 3
	const retryInterval = 5 * time.Second
	var err error
	for i := 0; i <= maxRetries; i++ {
		gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			return gormDB, nil
		}
		logger.Error("データベース接続のリトライ", zap.Int("retry", i), zap.Error(err))
		time.Sleep(retryInterval)
	}
	return nil, fmt.Errorf("データベース接続に失敗しました: %v", err)
}

func InitRedis(logger *zap.Logger) (*redis.Client, error) {
	// 環境変数からRedis接続情報を取得
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // デフォルト値
	}

	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB := os.Getenv("REDIS_DB")
	db, err := strconv.Atoi(redisDB)
	if err != nil {
		logger.Info("Invalid REDIS_DB value, using default DB 0")
		db = 0 // デフォルトDB
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       db,
	})

	// Redisへの接続テスト（オプショナル）
	if _, err = rdb.Ping(context.Background()).Result(); err != nil {
		logger.Error("Failed to connect to Redis", zap.Error(err))
		return nil, err
	}

	logger.Info("Connected to Redis")
	return rdb, nil
}
