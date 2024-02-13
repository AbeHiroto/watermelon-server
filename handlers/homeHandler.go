package handlers

import (
	"fmt"
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
		return auth.JwtKey, nil // ！！！ここで使用するシークレットキーは、本番環境では環境変数で設定
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

// HomeHandler はホーム画面の情報を提供するハンドラです。
func HomeHandler(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	// トークンをヘッダーから取得し、ユーザーIDを解析
	tokenString := c.GetHeader("Authorization")
	if strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	}

	var userToken bool = true // トークンがある場合をデフォルトとする
	// トークンが存在しない場合
	if tokenString == "" {
		userToken = false // トークンがないことを示す
		c.JSON(http.StatusOK, gin.H{
			"hasToken":    userToken,
			"hasRoom":     false,
			"hasRequest":  false,
			"replyStatus": "none",
		})
		return
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

	var acceptedRequestCount int64
	// ユーザーの入室申請のステータスが"accepted"であるかどうかをチェック
	err = db.Model(&models.Challenger{}).Where("user_id = ? AND status = 'accepted'", userID).Count(&acceptedRequestCount).Error
	replyStatus := "none"
	if err == nil && acceptedRequestCount > 0 {
		replyStatus = "accepted"
	}

	response := gin.H{
		"hasToken":    userToken,
		"hasRoom":     hasRoom,
		"hasRequest":  hasRequest,
		"replyStatus": replyStatus,
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
