package handlers

import (
	"net/http"

	"xicserver/models"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
)

// DisableMyRequest ハンドラー
func DisableMyRequest(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	// JWTトークンからユーザーIDを取得
	userID, err := GetUserIDFromToken(c, logger)
	if err != nil {
		logger.Error("Failed to get user ID from token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "認証に失敗しました"})
		return
	}

	// URLパラメータから入室申請IDを取得
	requestID := c.Param("requestID")

	// 指定された入室申請が存在し、かつユーザーが申請者であることを確認
	var challenger models.Challenger
	if err := db.Where("challengers.id = ? AND challengers.user_id = ?", requestID, userID).First(&challenger).Error; err != nil {
		logger.Error("Request not found or not owned by the user", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "申請が見つからないか、あなたのものではありません"})
		return
	}

	// Status属性を"disabled"に更新
	if err := db.Model(&challenger).Update("status", "disabled").Error; err != nil {
		logger.Error("Failed to disable the request", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "申請の無効化に失敗しました"})
		return
	}

	// ここでユーザーのValidRequestCountをデクリメント
	if err := db.Model(&models.User{}).Where("id = ?", userID).Update("valid_request_count", gorm.Expr("valid_request_count - ?", 1)).Error; err != nil {
		logger.Error("Failed to decrement ValidRequestCount", zap.Error(err))
		// このエラーは入室申請の無効化には影響しないため、ユーザーには成功のレスポンスを返しますが、内部ログには記録します。
	}

	// 成功レスポンス
	c.JSON(http.StatusOK, gin.H{"message": "申請が無効化されました"})
}

// MyRequestHandler handles the request for viewing the status of an application to a room.
func MyRequestHandler(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	// JWTトークンからユーザーIDを取得
	userID, err := GetUserIDFromToken(c, logger)
	if err != nil {
		logger.Error("Failed to get user ID from token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "認証に失敗しました"})
		return
	}

	// URLパラメータから入室申請IDを取得
	requestID := c.Param("requestID")

	// 入室申請情報と関連するルーム情報を取得
	var requestInfo struct {
		ChallengerNickname string `gorm:"column:challenger_nickname"`
		RoomCreator        string `gorm:"column:room_creator"`
		RoomTheme          string `gorm:"column:room_theme"`
		CreatedAt          string `gorm:"column:created_at"`
	}
	err = db.Table("challengers").
		Select("challengers.challenger_nickname, game_rooms.room_creator, game_rooms.room_theme, challengers.created_at").
		Joins("join game_rooms on game_rooms.id = challengers.game_room_id").
		Where("challengers.id = ? AND challengers.user_id = ?", requestID, userID).
		Scan(&requestInfo).Error

	if err != nil {
		logger.Error("Failed to retrieve request information", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "申請情報の取得に失敗しました"})
		return
	}

	// 取得した情報をレスポンスとして返す
	c.JSON(http.StatusOK, gin.H{
		"challengerNickname": requestInfo.ChallengerNickname,
		"roomCreator":        requestInfo.RoomCreator,
		"roomTheme":          requestInfo.RoomTheme,
		"createdAt":          requestInfo.CreatedAt,
	})
}
