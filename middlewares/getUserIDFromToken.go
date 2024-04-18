package middlewares

import (
	"fmt"
	"net/http"
	"strings"

	"xicserver/auth"
	"xicserver/models"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// リクエストからJWTトークンを取得し、ユーザーIDを解析して返します。
func GetUserIDFromToken(c *gin.Context, logger *zap.Logger) (uint, error) {
	// トークンをリクエストヘッダーから取得
	tokenString := c.GetHeader("Authorization")

	// Bearerトークンのプレフィックスを確認し、存在する場合は削除
	if strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	}

	// ここでtokenStringが空文字列でないことを確認
	if tokenString == "" {
		logger.Error("Token string is empty")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
		return 0, fmt.Errorf("Token is required")
	}

	// JWTトークンの解析
	token, err := jwt.ParseWithClaims(tokenString, &models.MyClaims{}, func(token *jwt.Token) (interface{}, error) {
		return auth.JwtKey, nil // ！！！ここで使用するシークレットキーは、本番環境では環境変数で設定
	})

	if err != nil {
		logger.Error("Failed to parse JWT token", zap.Error(err))
		return 0, err
	}

	// クレームの検証とユーザーIDの取得
	if claims, ok := token.Claims.(*models.MyClaims); ok && token.Valid {
		return claims.UserID, nil
	} else {
		return 0, err
	}
}
