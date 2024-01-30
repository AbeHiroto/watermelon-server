package models

//"gorm.io/gorm"

type TokenRequest struct {
	SubscriptionStatus string `json:"subscriptionStatus" binding:"required"`
}
