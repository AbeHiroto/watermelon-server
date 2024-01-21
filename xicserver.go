package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// User モデルの定義
type User struct {
	gorm.Model                // gorm.Modelを埋め込むことでID、CreatedAt、UpdatedAt、DeletedAtフィールドが自動的に追加されます
	UserID             string `gorm:"unique;not null"` // ユーザーID
	SubscriptionStatus string `gorm:"not null"`        // 課金ステータス (無課金、スタンダード、プレミアムなど)
	ValidRoomCount     int    `gorm:"not null"`        // 有効なルームの数
}

// テーブルの作成
func AutoMigrateDB(db *gorm.DB) {
	err := db.AutoMigrate(&User{})
	if err != nil {
		panic("Error migrating User table: " + err.Error())
	} else {
		fmt.Println("User table created successfully")
	}
	// 他のモデルに対しても同様にAutoMigrateを呼び出すことができます
}

func main() {
	// 環境変数からデータベースの接続情報を取得
	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	dbname := os.Getenv("DB_NAME")
	password := os.Getenv("DB_PASSWORD")
	sslmode := os.Getenv("DB_SSLMODE")

	dsn := "host=" + host + " user=" + user + " dbname=" + dbname + " password=" + password + " sslmode=" + sslmode
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// テーブル 'users' が存在するかを確認する
	var exists bool
	db.Raw("SELECT exists (SELECT 1 FROM information_schema.tables WHERE table_name = 'users')").Scan(&exists)
	if exists {
		log.Println("Table 'users' exists.")
	} else {
		log.Println("Table 'users' does not exist.")
	}

	AutoMigrateDB(db)

	// その他のデータベース操作...
}
