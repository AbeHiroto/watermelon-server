package models

import (
	"gorm.io/gorm"
)

// User モデルの定義
type User struct {
	gorm.Model
	SubscriptionStatus string `gorm:"not null"`
	ValidRoomCount     int    `gorm:"not null"`
}
