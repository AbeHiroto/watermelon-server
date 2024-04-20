package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"xicserver/middlewares"
	"xicserver/models"

	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RoomCreateRequest struct {
	SubscriptionStatus string `json:"subscriptionStatus,omitempty"` // 課金ステータス
	Nickname           string `json:"nickname"`                     // ニックネーム
	RoomTheme          string `json:"roomTheme"`                    // ルームのテーマ
}

func RoomCreate(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	var request RoomCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error("Room create request bind error", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "request_binding_error",
			"message": err.Error(),
		})
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

	// 一意の招待URLを生成し、重複がないことを確認する部分
	var uniqueToken string
	for {
		bytes := make([]byte, 8) // 64ビットの乱数を生成
		_, err := rand.Read(bytes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate unique token"})
			return
		}
		uniqueToken = hex.EncodeToString(bytes) // 16進数の文字列に変換

		// 生成されたトークンがデータベース内で既に使用されていないかを確認
		var exists bool
		db.Model(&models.GameRoom{}).Select("count(*) > 0").Where("unique_token = ?", uniqueToken).Find(&exists)
		if !exists {
			break // 重複がなければループを抜ける
		}
		// 重複があればループを続け、新しいトークンを生成
	}

	// トークンが有効な場合のみゲームルームを作成
	if tokenValid {
		// ユーザーが既にゲームルームを持っているか確認
		var user models.User
		if err := db.First(&user, userID).Error; err != nil {
			logger.Error("Failed to fetch user", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "User fetch failed"})
			return
		}
		if user.HasRoom {
			// すでにゲームルームを持っている場合はエラーを返す
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "room_limit",
				"error":  "User already has an active room",
			})
			return
		}

		// ユーザーがゲームルームを持っていなければ新たに作成
		newGameRoom := models.GameRoom{
			UserID:      userID,
			RoomCreator: request.Nickname,
			GameState:   "created",
			UniqueToken: uniqueToken,
			RoomTheme:   request.RoomTheme,
		}
		if err := db.Create(&newGameRoom).Error; err != nil {
			logger.Error("Failed to create a new game room", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ゲームルーム作成に失敗しました"})
			return
		}

		// ゲームルームの作成に成功したので、ユーザーのHasRoomフィールドを更新
		user.HasRoom = true
		if err := db.Save(&user).Error; err != nil {
			logger.Error("Failed to update user's room status", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user room status"})
			return
		}

		// 成功レスポンスを返す
		c.JSON(http.StatusOK, gin.H{
			"status":      "success",
			"gameRoomID":  newGameRoom.ID,
			"uniqueToken": newGameRoom.UniqueToken,
		})
	} else {
		// トークンが無効な場合、新しいトークンを返す
		c.JSON(http.StatusOK, gin.H{
			"status": "no_token",
			"token":  newToken,
		})
	}
}
