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
	Nickname           string `json:"nickname"`           // 入室申請者のニックネーム
	SubscriptionStatus string `json:"subscriptionStatus"` // 課金ステータス
}

// ChallengerHandler は入室申請を処理するハンドラです。
func ChallengerHandler(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	uniqueToken := c.Param("uniqueToken") // URLからUniqueTokenを取得

	// UniqueTokenを使用してGameRoomを検索
	var gameRoom models.GameRoom
	if err := db.Where("unique_token = ?", uniqueToken).First(&gameRoom).Error; err != nil {
		logger.Error("GameRoom not found with unique token", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "GameRoom not found"})
		return
	}

	// リクエストからニックネームを取得（その他のユーザー情報もここで取得可能）
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
		var user models.User
		if err := db.First(&user, userID).Error; err != nil {
			logger.Error("Failed to fetch user", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user data"})
			return
		}
		if user.HasRequest {
			c.JSON(http.StatusBadRequest, gin.H{"error": "You already have an active request"})
			return
		}

		// 新しい入室申請を作成
		newChallenger := models.Challenger{
			UserID:             userID,
			GameRoomID:         gameRoom.ID,
			ChallengerNickname: request.Nickname, // ニックネームを設定
			Status:             "pending",        // デフォルトは"pending"
		}
		if err := db.Create(&newChallenger).Error; err != nil {
			logger.Error("Failed to create a new challenger", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create a new request"})
			return
		}

		// 入室申請作成後にユーザーのHasRequestをtrueに更新
		if err := db.Model(&models.User{}).Where("id = ?", userID).Update("has_request", true).Error; err != nil {
			logger.Error("Failed to update user's has_request", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update request status"})
			return
		}

		// リクエストが成功した場合のレスポンス
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
