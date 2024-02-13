package handlers

import (
	//"fmt"
	"net/http"
	//"strings"

	//"xicserver/auth"
	"xicserver/models"

	"go.uber.org/zap"
	"gorm.io/gorm"

	//jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

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
	userID, err := GetUserIDFromToken(c, logger)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	requestId := c.Param("requestID")

	// ルーム作成者としての認証を確認
	var challenger models.Challenger
	if err := db.Joins("JOIN game_rooms ON game_rooms.id = challengers.game_room_id").
		Where("challengers.id = ? AND game_rooms.user_id = ?", requestId, userID).
		First(&challenger).Error; err != nil {
		logger.Error("Failed to find challenger or unauthorized", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Challenger not found or unauthorized"})
		return
	}

	// Status属性を更新
	if replyRequest.Status != "accepted" && replyRequest.Status != "rejected" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status value"})
		return
	}
	if err := db.Model(&challenger).Update("status", replyRequest.Status).Error; err != nil {
		logger.Error("Failed to update challenger status", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update challenger status"})
		return
	}

	// "reject"の場合、ValidRequestCountをデクリメント
	if replyRequest.Status == "rejected" {
		if err := db.Model(&models.User{}).Where("id = ?", challenger.UserID).Update("valid_request_count", gorm.Expr("valid_request_count - ?", 1)).Error; err != nil {
			logger.Error("Failed to decrement ValidRequestCount", zap.Error(err))
			// このエラーは入室申請の状態更新には影響しないため、ユーザーには成功のレスポンスを返しますが、内部ログには記録します。
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reply successfully processed"})
}

// RoomDeleteHandler handles the request for deleting a room.
func RoomDeleteHandler(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	// JWTトークンからユーザーIDを取得
	userID, err := GetUserIDFromToken(c, logger)
	if err != nil {
		logger.Error("Failed to get user ID from token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "認証に失敗しました",
		})
		return
	}

	// URLパラメータからルームIDを取得
	roomID := c.Param("roomID")

	// ルームの所有者かどうかを確認し、条件を満たす場合はGameStateを"disabled"に更新
	result := db.Model(&models.GameRoom{}).
		Where("id = ? AND user_id = ?", roomID, userID).
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

	// GameStateを"disabled"に更新後にユーザーのValidRoomCountをデクリメント
	if result.RowsAffected > 0 {
		var user models.User
		if err := db.First(&user, userID).Error; err != nil {
			logger.Error("Failed to fetch user for updating room count", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user room count"})
			return
		}

		if user.ValidRoomCount > 0 {
			user.ValidRoomCount -= 1
			if err := db.Save(&user).Error; err != nil {
				logger.Error("Failed to decrement user's valid room count", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decrement room count"})
				return
			}
		}
	}

	// GameStateが"disabled"に更新されたルームに対する全ての入室申請のStatusを"disabled"に更新
	if result.RowsAffected > 0 {
		err := db.Model(&models.Challenger{}).
			Where("game_room_id = ?", roomID).
			Update("status", "disabled").Error

		if err != nil {
			logger.Error("Failed to update status of challengers", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update challengers' status"})
			return
		}
	}

	// 正常に処理が完了したことをクライアントに通知
	c.JSON(http.StatusOK, gin.H{
		"message": "ルームが正常に削除されました",
	})
}

// 作成者が自分のルームをタップして表示される情報を取得するハンドラー
func MyRoomInfoHandler(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	// JWTトークンからユーザーIDを取得
	userID, err := GetUserIDFromToken(c, logger)
	if err != nil {
		logger.Error("Failed to get user ID from token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{
			"status": "token_validation_error",
			"error":  "認証に失敗しました",
		})
		return
	}

	// ユーザーが作成したルームのIDを取得（例：URLパラメータから取得）
	roomID := c.Param("roomID")

	// ルームの所有者かどうかを確認し、RoomTheme、GameStateとcreated_atも取得
	var room models.GameRoom
	if err := db.Select("id", "user_id", "room_theme", "game_state", "created_at").Where("id = ? AND user_id = ?", roomID, userID).First(&room).Error; err != nil {
		logger.Error("Room not found or not owned by the user", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"status": "not_your_room_error",
			"error":  "ルームが見つからないか、ユーザーが所有していません",
		})
		return
	}

	// ルームIDに基づいて、該当する入室申請情報を取得
	var challengers []struct {
		ID                 uint   `json:"visitorId"`
		ChallengerNickname string `json:"challengerNickname"`
		Status             string `json:"status"`
	}
	if err := db.Model(&models.Challenger{}).Select("id", "challenger_nickname", "status").
		Where("game_room_id = ? AND status = ?", roomID, "pending").Scan(&challengers).Error; err != nil {
		logger.Error("Failed to retrieve challengers list", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "get_info_error",
			"error":  "入室申請一覧の取得に失敗しました",
		})
		return
	}

	// 申請者がいない場合の処理
	if len(challengers) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"status":  "no_challengers",
			"message": "申請者はいません。",
		})
		return
	}

	// 申請一覧とルームのテーマ、作成日時をレスポンスとして返す
	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"roomTheme":   room.RoomTheme,
		"gameState":   room.GameState,
		"created_at":  room.CreatedAt,
		"challengers": challengers,
	})
}
