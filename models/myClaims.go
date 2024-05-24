package models

import (
	"github.com/golang-jwt/jwt"
)

// MyClaims はJWTクレームの構造体定義です。
type MyClaims struct {
	UserID             uint   `json:"userid"`
	SubscriptionStatus string `json:"subscriptionStatus"`
	jwt.StandardClaims
}
