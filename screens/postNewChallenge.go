package screens

import (
	"net/http"
	"strings"

	"xicserver/middlewares"
	"xicserver/models"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ChallengerRequest は入室申請リクエストのボディを表す構造体です。
type ChallengerRequest struct {
	Nickname           string `json:"nickname"`           // 入室申請者のニックネーム
	SubscriptionStatus string `json:"subscriptionStatus"` // 課金ステータス
}

// ChallengerHandler は対戦申請を処理するハンドラです。
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

	// TokenAuthentication関数でJWTの有効性を確認、無効であれば更新されたトークンを送付する
	userID, newToken, tokenValid, err := middlewares.TokenAuthentication(c, db, logger, request.SubscriptionStatus)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token processing failed", "newToken": newToken})
		return
	}
	if !tokenValid {
		c.JSON(http.StatusOK, gin.H{"status": "token_invalid", "newToken": newToken})
		return
	}

	// トークンが有効だった場合はここで対戦申請を作成
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

		// 新しい対戦申請を作成
		newChallenger := models.Challenger{
			UserID:             userID,
			GameRoomID:         gameRoom.ID,
			ChallengerNickname: request.Nickname,
			Status:             "pending", // デフォルト値は"pending"
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
