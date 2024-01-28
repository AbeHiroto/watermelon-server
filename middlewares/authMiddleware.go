package middlewares

import (
	"fmt"
	"net/http"
	"time"

	"xicserver/models"
	// 必要なパッケージをインポート
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
	// データベース操作用のパッケージ
)

// グローバル変数としてデータベース接続を保持
var db *gorm.DB

// この関数を使用してデータベース接続を設定
func SetDatabase(database *gorm.DB) {
	db = database
}

var jwtKey = []byte("your_secret_key")

// JWTクレームの構造体定義
type MyClaims struct {
	UserID             string `json:"userid"`
	SubscriptionStatus string `json:"subscriptionStatus"`
	jwt.StandardClaims
}

// JWTトークンを生成する関数
func GenerateToken(user models.User) (string, error) {
	var expirationTime time.Time

	// 課金ユーザーは長い有効期限を設定
	if user.SubscriptionStatus == "paid" {
		expirationTime = time.Now().Add(72 * time.Hour) // 例: 72時間
	} else {
		// それ以外のユーザー（無料など）は短い有効期限
		expirationTime = time.Now().Add(24 * time.Hour) // 例: 24時間
	}

	// クレームの設定
	claims := &MyClaims{
		UserID:             user.UserID,
		SubscriptionStatus: user.SubscriptionStatus,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	// トークンの生成
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)

	return tokenString, err
}

// // トークン検証とユーザーID検証を行うミドルウェア
// func AuthMiddleware(logger *zap.Logger) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		token := c.GetHeader("Authorization")

// 		// ユーザーIDをトークンから抽出
// 		claims, err := extractUserIDFromToken(token)
// 		if err != nil {
// 			logger.Warn("トークンからユーザーIDの取得に失敗", zap.Error(err))
// 			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
// 			return
// 		}

// 		// ユーザーIDが取得できなかった場合の処理
// 		if claims == nil || claims.UserID == "" {
// 			logger.Warn("ユーザーIDが取得できなかった", zap.String("token", token))
// 			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
// 			return
// 		}

// 		// ユーザーIDが有効かどうかをチェック
// 		if isValidUserID(logger, db, claims.UserID) {
// 			logger.Info("認証成功", zap.String("token", token), zap.String("userID", claims.UserID))
// 			c.Set("UserID", claims.UserID) // ユーザーIDをコンテキストにセット
// 			c.Next()
// 		} else {
// 			logger.Warn("認証失敗", zap.String("token", token), zap.String("userID", claims.UserID))
// 			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
// 		}
// 	}
// }

// // トークンからユーザーIDを抽出する関数
// func extractUserIDFromToken(tokenString string) (*MyClaims, error) {
// 	claims := &MyClaims{}
// 	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
// 		return jwtKey, nil
// 	})
// 	if err != nil || !token.Valid {
// 		return nil, err
// 	}
// 	return claims, nil
// }

// トークン検証とユーザーID検証を行うミドルウェア
func AuthMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		userID := c.GetHeader("UserID")

		refreshTokenIfNeeded(c, token)

		if isValidToken(logger, db, token) && isValidUserID(logger, db, userID) {
			logger.Info("認証成功", zap.String("token", token), zap.String("userID", userID))
			c.Next()
		} else {
			logger.Warn("認証失敗", zap.String("token", token), zap.String("userID", userID))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		}
	}
}

// トークンが有効かどうかをチェックする関数
func isValidToken(logger *zap.Logger, db *gorm.DB, tokenString string) bool {
	claims := &MyClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil || !token.Valid {
		logger.Warn("トークンのパースに失敗", zap.Error(err))
		return false
	}

	var sessionToken models.SessionToken
	if err := db.Where("token = ? AND expires_at > ?", tokenString, time.Now()).First(&sessionToken).Error; err != nil {
		logger.Warn("トークンがデータベースに存在しない", zap.Error(err))
		return false
	}

	logger.Info("トークンが有効", zap.String("token", tokenString))
	return true
}

// トークンの有効期限が近づいているかどうかをチェック
func ValidateToken(tokenString string) (*MyClaims, bool, error) {
	claims := &MyClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil {
		return nil, false, err
	}

	if !token.Valid {
		return nil, false, fmt.Errorf("Invalid token")
	}

	// トークンの有効期限が1時間未満の場合、更新が必要
	needUpdate := time.Unix(claims.ExpiresAt, 0).Sub(time.Now()) < time.Hour

	return claims, needUpdate, nil
}

func refreshTokenIfNeeded(c *gin.Context, tokenString string) {
	claims, needUpdate, err := ValidateToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if needUpdate {
		var user models.User
		if err := db.Where("user_id = ?", claims.UserID).First(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
			return
		}

		// 新しいトークンを生成
		newToken, err := GenerateToken(user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		// 新しいトークンをレスポンスに追加
		c.Header("Authorization", newToken)
	}
}

// ユーザーIDが有効かどうかをチェックする関数
func isValidUserID(logger *zap.Logger, db *gorm.DB, userID string) bool {
	var user models.User
	if err := db.Where("user_id = ?", userID).First(&user).Error; err != nil {
		logger.Warn("ユーザーIDがデータベースに存在しない", zap.Error(err))
		return false
	}

	logger.Info("ユーザーIDが有効", zap.String("userID", userID))
	return true
}
