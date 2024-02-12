package handlers

import (
	"net/http"
	"strings"
	"time"
	"xicserver/auth"
	"xicserver/models"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// HomeHandler はホーム画面の情報を提供するハンドラです。
func HomeHandler(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	// トークンをヘッダーから取得し、ユーザーIDを解析
	tokenString := c.GetHeader("Authorization")
	if strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	}

	claims := &models.MyClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return auth.JwtKey, nil
	})

	if err != nil {
		logger.Error("Failed to parse JWT token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	userID := claims.UserID
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		logger.Error("Failed to retrieve user", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// ユーザーが作成したルームの有無と入室申請の有無
	hasRoom := user.ValidRoomCount > 0
	hasRequest := user.ValidRequestCount > 0

	// ユーザーの入室申請のステータスが"accepted"であるかどうかをチェック
	var acceptedRequestCount int64
	err = db.Model(&models.Challenger{}).Where("user_id = ? AND status = 'accepted'", userID).Count(&acceptedRequestCount).Error
	replyStatus := "none"
	if err == nil && acceptedRequestCount > 0 {
		replyStatus = "accepted"
	}

	response := gin.H{
		"hasRoom":     hasRoom,
		"hasRequest":  hasRequest,
		"replyStatus": replyStatus,
	}

	c.JSON(http.StatusOK, response)
}

// 	userID := claims.UserID
// 	var user models.User
// 	if err := db.First(&user, userID).Error; err != nil {
// 		logger.Error("Failed to retrieve user", zap.Error(err))
// 		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
// 		return
// 	}

// 	var hasRoom bool = user.ValidRoomCount > 0
// 	var hasRequest bool = user.ValidRequestCount > 0
// 	var challengersCount int64
// 	var gameRoomIDs []uint
// 	var roomCount int64

// 	// 24時間以内に更新されたルームのみをカウント
// 	// GameStateが "created" またはその他アクティブな状態にあるルームのみをカウント
// 	db.Model(&models.GameRoom{}).
// 		Where("user_id = ? AND game_state IN ('created', 'active') AND updated_at > ?", userID, time.Now().Add(-24*time.Hour)).
// 		Count(&roomCount)
// 	hasRoom = roomCount > 0

// 	// ユーザーが所有するゲームルームに対する"pending"状態の入室申請の数をカウント
// 	// GameStateが "created" またはその他アクティブな状態にあるルームのみをカウントし、"disabled"は除外
// 	err = db.Model(&models.Challenger{}).
// 		Joins("join game_rooms on game_rooms.id = challengers.game_room_id").
// 		Where("game_rooms.user_id = ? AND challengers.status = ? AND game_rooms.game_state NOT IN ('disabled')", userID, "pending").
// 		Count(&challengersCount).Error

// 	if err != nil {
// 		logger.Error("Failed to count pending challengers", zap.Error(err))
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count pending challengers"})
// 		return
// 	}

// 	db.Model(&models.GameRoom{}).Where("user_id = ?", userID).Pluck("id", &gameRoomIDs)

// 	// ユーザーが作成したルームに対する入室申請の総数を取得
// 	if len(gameRoomIDs) > 0 {
// 		db.Model(&models.Challenger{}).Where("game_room_id IN ?", gameRoomIDs).Count(&challengersCount)
// 	}

// 	var acceptedRequestCount int64
// 	// ユーザーが作成した入室申請の中で一つでも"accepted"があるかチェック
// 	err = db.Model(&models.Challenger{}).Where("user_id = ? AND status = 'accepted'", userID).Count(&acceptedRequestCount).Error
// 	replyStatus := "none"
// 	if acceptedRequestCount > 0 {
// 		replyStatus = "accepted"
// 	}

// 	response := gin.H{
// 		"hasRoom":          hasRoom,
// 		"challengersCount": challengersCount,
// 		"hasRequest":       hasRequest,
// 		"replyStatus":      replyStatus,
// 	}

// 	c.JSON(http.StatusOK, response)
// }

// ListHandler はリスト画面用のハンドラです。
func ListHandler(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	// トークンからユーザーIDを取得
	userID, err := GetUserIDFromToken(c, logger)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	var rooms []struct {
		CreatedAt        time.Time `json:"createdAt"`
		ChallengersCount int       `json:"challengersCount"`
		RoomTheme        string    `json:"roomTheme"`
	}
	// ユーザーが作成したゲームルームの情報を取得
	if err := db.Model(&models.GameRoom{}).
		Where("user_id = ?", userID).
		Select("created_at", "challengers_count", "room_theme").
		Find(&rooms).Error; err != nil {
		logger.Error("Failed to retrieve rooms list", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve rooms list"})
		return
	}

	var requests []struct {
		RoomCreator string `json:"roomCreator"`
	}
	// ユーザーが送信した入室申請の情報を取得
	if err := db.Model(&models.Challenger{}).
		Joins("join game_rooms on game_rooms.id = challengers.game_room_id").
		Where("challengers.user_id = ?", userID).
		Select("game_rooms.room_creator").
		Find(&requests).Error; err != nil {
		logger.Error("Failed to retrieve requests list", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve requests list"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rooms":    rooms,
		"requests": requests,
	})
}
