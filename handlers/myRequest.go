package handlers

import (
	"net/http"

	"xicserver/middlewares"
	"xicserver/models"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
)

// DisableMyRequest ハンドラー
func DisableMyRequest(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	// JWTトークンからユーザーIDを取得
	userID, err := middlewares.GetUserIDFromToken(c, logger)
	if err != nil {
		logger.Error("Failed to get user ID from token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "認証に失敗しました"})
		return
	}

	// ユーザーIDに基づいてユーザーの全ての"pending"状態の入室申請を"disabled"に更新
	result := db.Model(&models.Challenger{}).
		Where("user_id = ? AND status = 'pending'", userID).
		Update("status", "disabled")

	if result.Error != nil {
		logger.Error("Failed to disable the requests", zap.Error(result.Error))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "申請の無効化に失敗しました"})
		return
	}

	if result.RowsAffected == 0 {
		// 対象の申請が見つからない、またはすでに無効化されている場合
		c.JSON(http.StatusNotFound, gin.H{"error": "有効な申請が見つかりません"})
		return
	}

	// 申請が無効化されたため、ユーザーのHasRequestをfalseに更新
	if err := db.Model(&models.User{}).Where("id = ?", userID).Update("has_request", false).Error; err != nil {
		logger.Error("Failed to update user's HasRequest", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ユーザーの申請状態更新に失敗しました"})
		return
	}

	// 成功レスポンス
	c.JSON(http.StatusOK, gin.H{"message": "全ての申請が無効化されました"})
}

// MyRequestHandler handles the request for viewing the status of an application to a room.
func MyRequestHandler(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	// JWTトークンからユーザーIDを取得
	userID, err := middlewares.GetUserIDFromToken(c, logger)
	if err != nil {
		logger.Error("Failed to get user ID from token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "認証に失敗しました"})
		return
	}

	// 入室申請情報と関連するルーム情報を取得
	var requests []struct {
		ChallengerNickname string `json:"challengerNickname"`
		RoomCreator        string `json:"roomCreator"`
		RoomTheme          string `json:"roomTheme"`
		Status             string `json:"status"`
		CreatedAt          string `json:"createdAt"`
	}
	err = db.Table("challengers").
		Select("challengers.challenger_nickname, game_rooms.room_creator, game_rooms.room_theme, challengers.status, challengers.created_at").
		Joins("join game_rooms on game_rooms.id = challengers.game_room_id").
		Where("challengers.user_id = ?", userID).
		Scan(&requests).Error

	if err != nil {
		logger.Error("Failed to retrieve request information", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "申請情報の取得に失敗しました"})
		return
	}

	// 取得した情報をレスポンスとして返す
	c.JSON(http.StatusOK, gin.H{
		"requests": requests,
	})
}
