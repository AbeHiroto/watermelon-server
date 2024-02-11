package handlers

import (
	"net/http"
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
	claims := &models.MyClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// JWT署名検証のためのキーを返す（実際のキーに置き換えてください）
		return auth.JwtKey, nil
	})

	if err != nil {
		logger.Error("Failed to parse JWT token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	userID := claims.UserID
	var response gin.H

	// ユーザーが作成したゲームルーム情報を取得
	var gameRooms []models.GameRoom
	db.Where("user_id = ?", userID).Find(&gameRooms)

	// ユーザーの入室申請情報を取得
	var requests []models.Challenger
	db.Where("user_id = ?", userID).Find(&requests)

	// レスポンスの内容を決定
	if len(gameRooms) == 0 && len(requests) == 0 {
		response = gin.H{"message": "Create room or join rooms"}
	} else {
		response = gin.H{
			"gameRooms": gameRooms,
			"requests":  requests,
		}
	}

	c.JSON(http.StatusOK, response)
}

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
