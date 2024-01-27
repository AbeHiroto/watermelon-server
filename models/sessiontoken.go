package models

import (
	"time"

	"gorm.io/gorm"
)

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
