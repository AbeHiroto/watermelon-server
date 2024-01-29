// package middlewares

// import (
// 	"fmt"
// 	"net/http"
// 	"time"

// 	"xicserver/auth"
// 	"xicserver/handlers"
// 	"xicserver/models"

// 	// 必要なパッケージをインポート
// 	jwt "github.com/dgrijalva/jwt-go"
// 	"github.com/gin-gonic/gin"
// 	"go.uber.org/zap"
// 	"gorm.io/gorm"
// 	// データベース操作用のパッケージ
// )

// // グローバル変数としてデータベース接続を保持
// var db *gorm.DB

// // この関数を使用してデータベース接続を設定
// func SetDatabase(database *gorm.DB) {
// 	db = database
// }

// // トークン検証を行うミドルウェア
// func AuthMiddleware(logger *zap.Logger) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		token := c.GetHeader("Authorization")

// 		refreshTokenIfNeeded(c, token)

// 		if auth.IsValidToken(logger, db, token) {
// 			logger.Info("認証成功", zap.String("token", token))
// 			c.Next()
// 		} else {
// 			logger.Warn("認証失敗", zap.String("token", token))
// 			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
// 		}
// 	}
// }

// // トークンの有効期限が近づいているかどうかをチェック
// func ValidateToken(tokenString string) (*models.MyClaims, bool, error) {
// 	claims := &models.MyClaims{}

// 	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
// 		return auth.JwtKey, nil
// 	})

// 	if err != nil {
// 		return nil, false, err
// 	}

// 	if !token.Valid {
// 		return nil, false, fmt.Errorf("Invalid token")
// 	}

// 	// トークンの有効期限が1時間未満の場合、更新が必要
// 	needUpdate := time.Unix(claims.ExpiresAt, 0).Sub(time.Now()) < time.Hour

// 	return claims, needUpdate, nil
// }

// func refreshTokenIfNeeded(c *gin.Context, tokenString string) {
// 	claims, needUpdate, err := ValidateToken(tokenString)
// 	if err != nil {
// 		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
// 		return
// 	}

// 	if needUpdate {
// 		var user models.User
// 		if err := db.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
// 			return
// 		}

// 		// 新しいトークンを生成
// 		newToken, err := handlers.GenerateToken(user.SubscriptionStatus)
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
// 			return
// 		}

// 		// 新しいトークンをレスポンスに追加
// 		c.Header("Authorization", newToken)
// 	}
// }
