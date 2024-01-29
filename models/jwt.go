package models

import (
	"github.com/dgrijalva/jwt-go"
)

// MyClaims はJWTクレームの構造体定義です。
type MyClaims struct {
	UserID             uint   `json:"userid"`
	SubscriptionStatus string `json:"subscriptionStatus"`
	jwt.StandardClaims
}
