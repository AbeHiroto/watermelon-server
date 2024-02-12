package models

import (
	"gorm.io/gorm"
)

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
