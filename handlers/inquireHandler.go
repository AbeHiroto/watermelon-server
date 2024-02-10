package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"xicserver/models"

	"go.uber.org/zap"
	"gorm.io/gorm"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

// ReplyRequest はリプライリクエストのボディを表す構造体です。
type ReplyRequest struct {
	Status string `json:"status"` // "accept"または"reject"
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

	requestId := c.Param("requestId")

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
	if replyRequest.Status != "accept" && replyRequest.Status != "reject" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status value"})
		return
	}
	if err := db.Model(&challenger).Update("status", replyRequest.Status).Error; err != nil {
		logger.Error("Failed to update challenger status", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update challenger status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reply successfully processed"})
}

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
	if err := db.Where("id = ? AND user_id = ?", requestID, userID).First(&challenger).Error; err != nil {
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

	// 成功レスポンス
	c.JSON(http.StatusOK, gin.H{"message": "申請が無効化されました"})
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

	// 正常に処理が完了したことをクライアントに通知
	c.JSON(http.StatusOK, gin.H{
		"message": "ルームが正常に削除されました",
	})
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

// getUserIDFromToken はリクエストからJWTトークンを取得し、ユーザーIDを解析して返します。
func GetUserIDFromToken(c *gin.Context, logger *zap.Logger) (uint, error) {
	// トークンをリクエストヘッダーから取得
	tokenString := c.GetHeader("Authorization")

	// Bearerトークンのプレフィックスを確認し、存在する場合は削除
	if strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	}

	// ここでtokenStringが空文字列でないことを確認
	if tokenString == "" {
		logger.Error("Token string is empty")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
		return 0, fmt.Errorf("Token is required")
	}

	// JWTトークンの解析
	token, err := jwt.ParseWithClaims(tokenString, &models.MyClaims{}, func(token *jwt.Token) (interface{}, error) {
		// ！！！ここで使用するシークレットキーは、本番環境では環境変数で設定
		return []byte("your_secret_key"), nil
	})

	if err != nil {
		logger.Error("Failed to parse JWT token", zap.Error(err))
		return 0, err
	}

	// クレームの検証とユーザーIDの取得
	if claims, ok := token.Claims.(*models.MyClaims); ok && token.Valid {
		return claims.UserID, nil
	} else {
		return 0, err
	}
}
