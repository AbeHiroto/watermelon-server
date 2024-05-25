package screens

import (
	"net/http"

	"xicserver/middlewares"
	"xicserver/models"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
)

// ルームを作成したユーザーのホーム画面に表示される情報を取得するハンドラー
func MyRoomInfo(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	// JWTトークンからユーザーIDを取得
	userID, err := middlewares.GetUserIDFromToken(c, logger)
	if err != nil {
		logger.Error("Failed to get user ID from token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{
			"status": "token_validation_error",
			"error":  "認証に失敗しました",
		})
		return
	}

	// ユーザーが作成した、特定のステータス（"disabled"や"finished"）を除外したルーム情報を取得
	var rooms []models.GameRoom
	if err := db.Where("user_id = ? AND game_state NOT IN ('disabled', 'finished')", userID).Find(&rooms).Error; err != nil {
		logger.Error("Failed to find rooms owned by the user", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"status": "not_your_rooms_error",
			"error":  "ユーザーが所有するルームが見つかりません",
		})
		return
	}

	// 各ルームに紐づく入室申請情報も同時に取得
	var roomsData []map[string]interface{}
	for _, room := range rooms {
		var challengers []struct {
			ID                 uint   `json:"visitorId"`
			ChallengerNickname string `json:"challengerNickname"`
			Status             string `json:"status"`
		}
		db.Model(&models.Challenger{}).Select("id", "challenger_nickname", "status").
			Where("game_room_id = ? AND status = ?", room.ID, "pending").Scan(&challengers)

		roomData := map[string]interface{}{
			"roomID":      room.ID,
			"roomTheme":   room.RoomTheme,
			"gameState":   room.GameState,
			"uniqueToken": room.UniqueToken,
			"createdAt":   room.CreatedAt,
			"challengers": challengers,
		}
		roomsData = append(roomsData, roomData)
	}

	// 全てのルームと申請情報をクライアントに返す
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"rooms":  roomsData,
	})
}

// ReplyRequest はリプライリクエストのボディを表す構造体です。
type ReplyRequest struct {
	Status string `json:"status"` // "accepted"または"rejected"
}

// ReplyHandler は入室申請に対するリプライ（承認または拒否）を処理します。
func ReplyHandler(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	var replyRequest ReplyRequest
	if err := c.ShouldBindJSON(&replyRequest); err != nil {
		logger.Error("Failed to bind reply request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// ユーザーIDをトークンから取得
	userID, err := middlewares.GetUserIDFromToken(c, logger)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	// ユーザーIDに基づいて、該当する入室申請を取得
	var challengers []models.Challenger
	if err := db.Joins("JOIN game_rooms ON game_rooms.id = challengers.game_room_id").
		Where("game_rooms.user_id = ? AND challengers.status = 'pending'", userID).
		Find(&challengers).Error; err != nil {
		logger.Error("Failed to find challengers for user", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "No pending challengers found or unauthorized"})
		return
	}

	// 申請の承認または拒否
	for _, challenger := range challengers {
		if replyRequest.Status != "accepted" && replyRequest.Status != "rejected" {
			continue // 不正なステータス値は無視
		}
		if err := db.Model(&challenger).Update("status", replyRequest.Status).Error; err != nil {
			logger.Error("Failed to update challenger status", zap.Error(err))
			continue // ステータス更新失敗はログに記録し続行
		}

		// "reject"の場合、申請者のHasRequestをfalseに更新
		if replyRequest.Status == "rejected" {
			if err := db.Model(&models.User{}).Where("id = ?", challenger.UserID).Update("has_request", false).Error; err != nil {
				logger.Error("Failed to update user's HasRequest status", zap.Error(err))
				// このエラーは入室申請の状態更新には影響しないため、ユーザーには成功のレスポンスを返しますが、内部ログには記録します。
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "All replies processed successfully"})
}

// RoomDeleteHandler handles the request for deleting a room.
func DeleteMyRoom(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	// JWTトークンからユーザーIDを取得
	userID, err := middlewares.GetUserIDFromToken(c, logger)
	if err != nil {
		logger.Error("Failed to get user ID from token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "認証に失敗しました",
		})
		return
	}

	// ユーザーIDに基づいてゲームルームを検索し、ゲーム状態を"disabled"に更新
	result := db.Model(&models.GameRoom{}).
		Where("user_id = ?", userID).
		Update("game_state", "disabled")

	if result.Error != nil {
		logger.Error("Failed to delete the room", zap.Error(result.Error))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "ルームの削除に失敗しました",
		})
		return
	}

	if result.RowsAffected == 0 {
		// 対象のルームが見つからない、またはユーザーが所有していない場合
		c.JSON(http.StatusNotFound, gin.H{
			"error": "ルームが見つからないか、所有していません",
		})
		return
	}

	// ユーザーのHasRoomをfalseに更新
	if err := db.Model(&models.User{}).Where("id = ?", userID).Update("has_room", false).Error; err != nil {
		logger.Error("Failed to update user's HasRoom status", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user's room status"})
		return
	}

	// GameStateが"disabled"に更新されたルームに対する全ての入室申請のStatusを"disabled"に更新
	err = db.Model(&models.Challenger{}).
		Where("game_room_id IN (SELECT id FROM game_rooms WHERE user_id = ?)", userID).
		Update("status", "disabled").Error

	if err != nil {
		logger.Error("Failed to update status of challengers", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update challengers' status"})
		return
	}

	// すべてのChallengerのUserIDを取得し、それらのHasRequestをfalseに更新
	var challengerUserIDs []uint
	db.Model(&models.Challenger{}).
		Where("game_room_id IN (SELECT id FROM game_rooms WHERE user_id = ?)", userID).
		Pluck("user_id", &challengerUserIDs)

	if len(challengerUserIDs) > 0 {
		if err := db.Model(&models.User{}).Where("id IN (?)", challengerUserIDs).Update("has_request", false).Error; err != nil {
			logger.Error("Failed to update challengers' HasRequest status", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update challengers' request status"})
			return
		}
	}

	// 正常に処理が完了したことをクライアントに通知
	c.JSON(http.StatusOK, gin.H{"message": "ルームが正常に削除されました"})
}
