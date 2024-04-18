package models

// "time"

// "gorm.io/gorm"

// // SessionToken モデルの定義
// type SessionToken struct {
// 	gorm.Model
// 	TokenID    uint
// 	UserID     uint      `gorm:"not null"`
// 	Token      string    `gorm:"not null"`
// 	TokenType  string    `gorm:"not null"` // "anonymous" または "registered"
// 	ExpiresAt  time.Time `gorm:"not null"`
// 	DeviceInfo string    // デバイス情報
// }

// type TokenRequest struct {
// 	SubscriptionStatus string `json:"subscriptionStatus" binding:"required"`
// }

// type myRoomInfoRequest struct {
// 	Token string `json:"token,omitempty"` // 既存のユーザー固有のJWTトークン
// }
