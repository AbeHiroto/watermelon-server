package models

import (
	"gorm.io/gorm"
)

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
