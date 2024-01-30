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
	var err error
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error("Room create request bind error", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var userID uint
	var newToken string
	var tokenValid = false // トークンの有効性を判定するためのフラグ

	// トークンが提供されている場合
	if request.Token != "" {
		claims := &models.MyClaims{}
		token, err := jwt.ParseWithClaims(request.Token, claims, func(token *jwt.Token) (interface{}, error) {
			return auth.JwtKey, nil
		})

		if err != nil || !token.Valid {
			logger.Error("Token validation error", zap.Error(err))
			newToken, userID, err = GenerateToken(request.SubscriptionStatus, 0)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "トークン生成に失敗しました"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"token": newToken})
			return
		} else {
			userID = claims.UserID
			// トークンの有効期限が1時間未満の場合は新しいトークンを生成
			if time.Unix(claims.ExpiresAt, 0).Sub(time.Now()) < time.Hour {
				newToken, _, err = GenerateToken(claims.SubscriptionStatus, userID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "トークン生成に失敗しました"})
					return
				}
				// 新しいトークンをクライアントに返す
				c.JSON(http.StatusOK, gin.H{"token": newToken})
				return // ここで処理を終了し、ゲームルーム作成をスキップ
			} else {
				tokenValid = true // トークンが有効かつ有効期限が1時間以上ある場合
			}
		}

	} else {
		newToken, userID, err = GenerateToken(request.SubscriptionStatus, 0)
		if err != nil {
			logger.Error("Token generation error", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "トークン生成に失敗しました"})
			return
		}
	}

	// トークンが有効な場合のみゲームルームを作成
	if tokenValid {
		newGameRoom := models.GameRoom{
			// フィールドの設定...
		}
		if err := db.Create(&newGameRoom).Error; err != nil {
			logger.Error("Failed to create a new game room", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ゲームルーム作成に失敗しました"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": newToken, "gameRoomID": newGameRoom.ID})
	} else {
		// トークンが無効な場合、新しいトークンを返す
		c.JSON(http.StatusOK, gin.H{"token": newToken})
	}
}

func GenerateToken(subscriptionStatus string, existingUserID uint) (string, uint, error) {
	var expirationTime time.Time
	var userID uint
	var err error

	if existingUserID > 0 {
		// 既存のユーザーIDを再利用
		userID = existingUserID
	} else {
		// 新しいユーザーIDを生成
		userID, err = generateUserID(subscriptionStatus)
		if err != nil {
			logger.Error("トークン生成中にエラー発生", zap.Error(err))
			return "", 0, err
		}
	}

	// トークンの有効期限を設定
	if subscriptionStatus == "paid" {
		expirationTime = time.Now().Add(72 * time.Hour) // 例: 72時間
	} else {
		expirationTime = time.Now().Add(72 * time.Hour) // 例: 72時間
	}

	// JWTトークン生成時に内包するデータ
	claims := &models.MyClaims{
		UserID:             userID,
		SubscriptionStatus: subscriptionStatus,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(auth.JwtKey)

	return tokenString, userID, err
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
