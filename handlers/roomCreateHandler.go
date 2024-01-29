package handlers

import (
	"net/http"
	"time"

	"xicserver/auth"
	"xicserver/models"

	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	jwt "github.com/dgrijalva/jwt-go"
)

var db *gorm.DB // GORMデータベース接続を保持するグローバル変数

var logger *zap.Logger

func init() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
}

func RoomCreate(c *gin.Context) {
	var request models.LoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error("Room create request bind error", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if request.Token != "" {
		// トークンが提供された場合、そのトークンをパースして検証
		claims := &models.MyClaims{}
		token, err := jwt.ParseWithClaims(request.Token, claims, func(token *jwt.Token) (interface{}, error) {
			return auth.JwtKey, nil
		})

		if err != nil || !token.Valid {
			logger.Error("Token validation error", zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "認証失敗"})
			return
		}

		// トークンの有効期限チェック
		needUpdate := time.Unix(claims.ExpiresAt, 0).Sub(time.Now()) < time.Hour
		if needUpdate {
			// 新しいトークンを生成
			newToken, err := GenerateToken(claims.SubscriptionStatus)
			if err != nil {
				logger.Error("Token generation error", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "トークン生成に失敗しました"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"token": newToken})
			return
		}

		// トークンが有効な場合、認証成功
		c.JSON(http.StatusOK, gin.H{"message": "認証成功"})
		return
	}

	// トークンがない場合、新しいトークンを生成
	token, err := GenerateToken(request.SubscriptionStatus)
	if err != nil {
		logger.Error("Token generation error", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "トークン生成に失敗しました"})
		return
	}

	// トークンをクライアントに送信
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func GenerateToken(subscriptionStatus string) (string, error) {
	var expirationTime time.Time

	// generateUserIDから返されるユーザーIDとエラーを受け取る
	userID, err := generateUserID(subscriptionStatus)
	if err != nil {
		logger.Error("トークン生成中にエラー発生", zap.Error(err))
		return "", err
	}

	if subscriptionStatus == "paid" {
		expirationTime = time.Now().Add(72 * time.Hour) // 例: 72時間
	} else {
		expirationTime = time.Now().Add(72 * time.Hour) // 例: 72時間
	}
	//JWTトークン生成時に内包するデータ
	claims := &models.MyClaims{
		UserID:             userID,
		SubscriptionStatus: subscriptionStatus,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(auth.JwtKey)

	return tokenString, err
}

// GORMによるオートインクリメントユーザーIDを生成する関数
func generateUserID(subscriptionStatus string) (uint, error) {
	// データベースに新しいUserインスタンスを作成
	user := models.User{
		SubscriptionStatus: subscriptionStatus, // 課金ステータスを設定
	}
	if err := db.Create(&user).Error; err != nil {
		logger.Error("ユーザーID生成中にエラー発生", zap.Error(err))
		return 0, err // エラー発生時
	}
	return user.ID, nil // UserインスタンスのIDを返す
}

// // トークンからユーザーIDを抽出
// func extractUserIDFromToken(tokenString string) (*models.MyClaims, error) {
// 	claims := &models.MyClaims{}
// 	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
// 		return auth.JwtKey, nil
// 	})
// 	if err != nil || !token.Valid {
// 		return nil, err
// 	}
// 	return claims, nil
// }
