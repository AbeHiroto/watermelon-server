package models

//"gorm.io/gorm"

// User モデルの定義
type TokenRequest struct {
	SubscriptionStatus string `json:"subscriptionStatus" binding:"required"`
}
