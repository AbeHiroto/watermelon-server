package middlewares

import (
	"strings"
	"time"
	"xicserver/auth"
	"xicserver/models"

	"github.com/gin-gonic/gin"
	jwt "github.com/golang-jwt/jwt"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// リクエストからJWTトークンを検証し、ユーザーIDと新トークンを返します。
func TokenAuthentication(c *gin.Context, db *gorm.DB, logger *zap.Logger, subscriptionStatus string) (uint, string, bool, error) {
	tokenString := c.GetHeader("Authorization")
	if strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	}

	if tokenString == "" {
		// トークンが提供されていない場合、新しいトークンを生成
		newToken, userID, err := GenerateToken(db, subscriptionStatus, 0)
		if err != nil {
			logger.Error("Token generation error", zap.Error(err))
			return 0, "", false, err
		}
		return userID, newToken, false, nil
	}

	claims := &models.MyClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return auth.JwtKey, nil
	})
	if err != nil || !token.Valid {
		// トークンが無効な場合は新しいトークンを生成
		newToken, userID, err := GenerateToken(db, claims.SubscriptionStatus, 0)
		if err != nil {
			logger.Error("Token generation error", zap.Error(err))
			return 0, newToken, false, err
		}
		return userID, newToken, false, nil
	}

	// トークンの有効期限が1時間未満の場合は新しいトークンを生成
	if time.Unix(claims.ExpiresAt, 0).Sub(time.Now()) < time.Hour {
		newToken, _, err := GenerateToken(db, claims.SubscriptionStatus, claims.UserID)
		if err != nil {
			logger.Error("Token generation error", zap.Error(err))
			return claims.UserID, "", false, err
		}
		return claims.UserID, newToken, true, nil
	}

	return claims.UserID, "", true, nil // トークンが有効で有効期限が1時間以上残っている場合
}
