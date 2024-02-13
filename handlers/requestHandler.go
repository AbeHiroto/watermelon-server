package handlers

import (
	"net/http"
	"strings"
	"time"
	"xicserver/auth"
	"xicserver/middlewares"
	"xicserver/models"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ChallengerRequest は入室申請リクエストのボディを表す構造体です。
type ChallengerRequest struct {
	GameRoomID         uint   `json:"gameRoomId"`         // 申請するゲームルームのID
	Nickname           string `json:"nickname"`           // 入室申請者のニックネーム
	SubscriptionStatus string `json:"subscriptionStatus"` // 課金ステータス
}

// ChallengerHandler は入室申請を処理するハンドラです。
func ChallengerHandler(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	var request ChallengerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error("Request binding error", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request binding error"})
		return
	}

	// トークンをヘッダーから取得
	tokenString := c.GetHeader("Authorization")
	// Bearerトークンのプレフィックスを確認し、存在する場合は削除
	if strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	}

	var userID uint
	var newToken string
	var tokenValid bool = false // トークンの有効性を判定するフラグ

	if tokenString != "" {
		claims := &models.MyClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return auth.JwtKey, nil
		})

		if err != nil || !token.Valid {
			logger.Error("Token validation error", zap.Error(err))
			newToken, userID, err = middlewares.GenerateToken(db, claims.SubscriptionStatus, 0) // 課金ステータスはリクエストから取得またはデータベースを確認
			if err != nil {
				logger.Error("Token generation error", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate new token"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"newToken": newToken})
			return
		} else {
			userID = claims.UserID
			if time.Unix(claims.ExpiresAt, 0).Sub(time.Now()) < time.Hour {
				newToken, _, err = middlewares.GenerateToken(db, claims.SubscriptionStatus, userID)
				if err != nil {
					logger.Error("Token generation error", zap.Error(err))
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate new token"})
					return
				}
				c.JSON(http.StatusOK, gin.H{"newToken": newToken})
				return
			} else {
				tokenValid = true
			}
		}
	} else {
		// トークンが提供されていない場合、新しいトークンを生成
		newToken, _, err := middlewares.GenerateToken(db, request.SubscriptionStatus, 0)
		if err != nil {
			logger.Error("Token generation error", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate new token"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"newToken": newToken})
		return
	}

	if tokenValid {
		// アクティブな入室申請の数を確認
		var count int64
		db.Model(&models.Challenger{}).
			Where("user_id = ? AND status = 'pending'", userID).
			Count(&count)

		maxRequestCount := 5 // ユーザーごとの入室申請上限数
		if count >= int64(maxRequestCount) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Reached the limit of active requests"})
			return
		}

		// 新しい入室申請を作成
		newChallenger := models.Challenger{
			UserID:             userID,
			GameRoomID:         request.GameRoomID,
			ChallengerNickname: request.Nickname, // ニックネームを設定
			Status:             "pending",        // デフォルトは"pending"
		}
		if err := db.Create(&newChallenger).Error; err != nil {
			logger.Error("Failed to create a new challenger", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create a new request"})
			return
		}

		// 入室申請作成後にユーザーのValidRequestCountをインクリメント
		if err := db.Model(&models.User{}).Where("id = ?", userID).Update("valid_request_count", gorm.Expr("valid_request_count + ?", 1)).Error; err != nil {
			logger.Error("Failed to increment ValidRequestCount", zap.Error(err))
			// このエラーは入室申請の作成には影響しないため、ユーザーには成功のレスポンスを返しますが、内部ログには記録します。
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":   "Request successfully created",
			"requestId": newChallenger.ID,
		})
	} else {
		// トークンが無効な場合、新しいトークンを返す
		c.JSON(http.StatusOK, gin.H{
			"status": "no_token",
			"token":  newToken,
		})
	}
}
