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

	jwt "github.com/dgrijalva/jwt-go"
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

// トークン生成用のシークレットキー（実際のアプリケーションでは安全に管理する必要があります）
var jwtKey = []byte("my_secret_key")

// JWTクレームの構造体定義
type MyClaims struct {
	UserID string `json:"userid"`
	jwt.StandardClaims
}

// JWTトークンを生成する関数
func generateToken(userID string) (string, error) {
	// 有効期限の設定
	expirationTime := time.Now().Add(1 * time.Hour)

	// クレームの設定
	claims := &MyClaims{
		UserID: userID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	// トークンの生成
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)

	return tokenString, err
}

// JWTトークンを検証する関数
func validateToken(tokenString string) (*MyClaims, error) {
	claims := &MyClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("Invalid token")
	}

	return claims, nil
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

// SessionToken モデルの定義
type SessionToken struct {
	gorm.Model
	TokenID    uint
	UserID     uint      `gorm:"not null"`
	Token      string    `gorm:"not null"`
	TokenType  string    `gorm:"not null"` // "anonymous" または "registered"
	ExpiresAt  time.Time `gorm:"not null"`
	DeviceInfo string    // デバイス情報
}

func main() {
	logger.Info("アプリケーションが起動しました。")

	r := gin.New()
	r.Use(gin.Recovery())                     // パニックからの回復を行う標準ミドルウェア
	r.Use(LoggerMiddleware(logger))           // ロギングミドルウェアの登録
	r.Use(middlewares.AuthMiddleware(logger)) // 認証ミドルウェアの適用

	authGroup := r.Group("/").Use(middlewares.AuthMiddleware(logger))
	{
		authGroup.POST("/gameroom", handlers.CreateGameRoom)
		// その他の保護されたルート
	}

	config, err := LoadConfig("config.json")
	if err != nil {
		logger.Fatal("設定ファイルの読み込みに失敗しました", zap.Error(err))
	}

	// データベース接続の初期化
	_, err = initDB(config)
	if err != nil {
		logger.Error("データベースの初期化に失敗しました", zap.Error(err))
		return
	}

	// ルーティングの設定
	r.POST("/gameroom", handlers.CreateGameRoom)
	r.GET("/someEndpoint", func(c *gin.Context) {
		// エンドポイントの処理
	})

	// トークンの生成
	tokenString, err := generateToken("12345")
	if err != nil {
		panic(err)
	}
	fmt.Println("Generated Token:", tokenString)

	// トークンの検証
	claims, err := validateToken(tokenString)
	if err != nil {
		panic(err)
	}
	fmt.Println("Token Valid, UserID:", claims.UserID)

	defer sqlDB.Close() //アプリケーション終了時にデータベース接続と、
	defer logger.Sync() //ロガーを適切に閉じます。

	r.Run() // デフォルトでは ":8080" でリスニングします
}
