package models

import (
	"gorm.io/gorm"
)

// GameRoom モデルの定義
type GameRoom struct {
	gorm.Model
	RoomCreator string `gorm:"not null"` // 作成者ニックネーム
	GameState   string `gorm:"not null;default:'created'"`
	UniqueToken string `gorm:"unique;not null"` // 招待URL
	FinishTime  int64
	StartTime   int64
	RoomTheme   string
}
