package main

import (
	"fmt"
	"os"

	//"encoding/json"
	//"io/ioutil"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// // データベース設定
// type Config struct {
// 	DBHost     string `json:"db_host"`
// 	DBUser     string `json:"db_user"`
// 	DBPassword string `json:"db_password"`
// 	DBName     string `json:"db_name"`
// 	DBSSLMode  string `json:"db_sslmode"`
// }

// User モデルの定義
type User struct {
	gorm.Model
	SubscriptionStatus string `gorm:"not null"`
	ValidRoomCount     int    `gorm:"not null;default:0"`
	ValidRequestCount  int    `gorm:"default:0"`
}

// GameRoom モデルの定義
type GameRoom struct {
	gorm.Model
	UserID           uint   `gorm:"not null"`
	RoomCreator      string `gorm:"not null"` // 作成者ニックネーム
	GameState        string `gorm:"not null;default:'created'"`
	UniqueToken      string `gorm:"unique;not null"` // 招待URL
	FinishTime       int64
	StartTime        int64
	RoomTheme        string
	ChallengersCount int          `gorm:"default:0"`             // 申請者数
	Challengers      []Challenger `gorm:"foreignKey:GameRoomID"` // 結びつく入室申請を取得
}

// 挑戦者は別テーブルで管理（複数の挑戦者に対応）
type Challenger struct {
	gorm.Model
	UserID             uint
	GameRoomID         uint     `gorm:"index"` // GameRoomテーブルのIDを参照
	ChallengerNickname string   // 挑戦希望者のニックネーム
	Status             string   `gorm:"index;default:'pending'"` // 申請状態を表す
	GameRoom           GameRoom `gorm:"foreignKey:GameRoomID"`   // GameRoomへの参照
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
	err := db.AutoMigrate(&User{}, &GameRoom{}, &Challenger{})
	if err != nil {
		logger.Error("Error migrating tables", zap.Error(err))
	} else {
		logger.Info("User and GameRoom tables created successfully")
	}

	// 複合インデックスの作成
	err = db.Exec("CREATE INDEX IF NOT EXISTS idx_user_id_game_state ON game_rooms (user_id, game_state)").Error
	if err != nil {
		logger.Error("Error creating index idx_user_id_game_state", zap.Error(err))
	}
}

func main() {
	// // config.jsonからデータベースの設定を読み込み
	// config := Config{}
	// configFile, err := ioutil.ReadFile("config.json")
	// if err != nil {
	// 	logger.Fatal("Error reading config file", zap.Error(err))
	// 	return
	// }
	// err = json.Unmarshal(configFile, &config)
	// if err != nil {
	// 	logger.Fatal("Error parsing config file", zap.Error(err))
	// 	return
	// }

	// 環境変数からデータベースの接続情報を取得
	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	dbname := os.Getenv("DB_NAME")
	password := os.Getenv("DB_PASSWORD")
	sslmode := os.Getenv("DB_SSLMODE")

	dsn := fmt.Sprintf("host=%s user=%s dbname=%s password=%s sslmode=%s", host, user, dbname, password, sslmode)
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
