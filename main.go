package main

import (
	//"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const maxRetries = 3                  // 最大再試行回数
const retryInterval = 5 * time.Second // 再試行間の待機時間
var logger *zap.Logger

func init() {
	var err error
	// Zapのロガーを設定
	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
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

	// 環境変数からデータベースの接続情報を取得
	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	dbname := os.Getenv("DB_NAME")
	password := os.Getenv("DB_PASSWORD")
	sslmode := os.Getenv("DB_SSLMODE")

	dsn := "host=" + host + " user=" + user + " dbname=" + dbname + " password=" + password + " sslmode=" + sslmode
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("データベースへの接続に失敗しました", zap.Error(err))
		return
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		logger.Error("SQLDBの取得に失敗しました", zap.Error(err))
		return
	}
	// db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	// if err != nil {
	// 	logger.Error("データベースへの接続に失敗しました", zap.Error(err))
	// 	return
	// }

	defer sqlDB.Close() // SQLDBを閉じる
	//defer db.Close()   // データベース接続を閉じる
	defer logger.Sync() // ロガーの終了処理
}

// package main

// import (
// 	"fmt"
// 	"os"
// 	"time"

// 	//"log"

// 	"go.uber.org/zap"
// 	"gorm.io/driver/postgres"
// 	"gorm.io/gorm"
// )

// const maxRetries = 3                  // 最大再試行回数
// const retryInterval = 5 * time.Second // 再試行間の待機時間
// var logger *zap.Logger

// func init() {
// 	var err error
// 	// Zapのロガーを設定
// 	logger, err = zap.NewProduction() // または zap.NewDevelopment() で開発用の設定を行う
// 	if err != nil {
// 		panic(err)
// 	}
// 	logger = logger.WithOptions(zap.IncreaseLevel(zap.InfoLevel))
// }

// // User モデルの定義
// type User struct {
// 	gorm.Model                // gorm.Modelを埋め込むことでID、CreatedAt、UpdatedAt、DeletedAtフィールドが自動的に追加されます
// 	UserID             string `gorm:"unique;not null"` // ユーザーID
// 	SubscriptionStatus string `gorm:"not null"`        // 課金ステータス (無課金、スタンダード、プレミアムなど)
// 	ValidRoomCount     int    `gorm:"not null"`        // 有効なルームの数
// }

// // テーブルの作成
// func AutoMigrateDB(db *gorm.DB) {
// 	err := db.AutoMigrate(&User{})
// 	if err != nil {
// 		panic("Error migrating User table: " + err.Error())
// 	} else {
// 		fmt.Println("User table created successfully")
// 	}
// 	// 他のモデルに対しても同様にAutoMigrateを呼び出すことができますx
// }

// func main() {
// 	logger.Info("アプリケーションが起動しました。")

// 	// 環境変数からデータベースの接続情報を取得
// 	host := os.Getenv("DB_HOST")
// 	user := os.Getenv("DB_USER")
// 	dbname := os.Getenv("DB_NAME")
// 	password := os.Getenv("DB_PASSWORD")
// 	sslmode := os.Getenv("DB_SSLMODE")

// 	dsn := "host=" + host + " user=" + user + " dbname=" + dbname + " password=" + password + " sslmode=" + sslmode

// 	var db *gorm.DB
// 	var err error
// 	for i := 0; i < maxRetries; i++ {
// 		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
// 		if err == nil {
// 			break
// 		}
// 		logger.Error("データベースへの接続に失敗しました", zap.Int("retry", i), zap.Error(err))
// 		time.Sleep(retryInterval)
// 	}
// 	if err != nil {
// 		logger.Fatal("データベースへの接続に最終的に失敗しました", zap.Error(err))
// 		return
// 	}

// 	// テーブル 'users' が存在するかを確認する
// 	var exists bool
// 	db.Raw("SELECT exists (SELECT 1 FROM information_schema.tables WHERE table_name = 'users')").Scan(&exists)
// 	if exists {
// 		logger.Info("Table 'users' exists.")
// 	} else {
// 		logger.Info("Table 'users' does not exist.")
// 	}

// 	AutoMigrateDB(db)

// 	defer logger.Sync()
