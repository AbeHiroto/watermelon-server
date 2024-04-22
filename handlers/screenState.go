package handlers

import (
	"net/http"
	"strings"

	//"time" //ListHandler関数用
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

	var userToken bool = true // トークンがある場合をデフォルトとする
	// トークンが存在しない場合
	if tokenString == "" {
		userToken = false // トークンがないことを示す
		c.JSON(http.StatusOK, gin.H{
			"hasToken":    userToken,
			"hasRoom":     false,
			"hasRequest":  false,
			"replyStatus": "none",
			"roomStatus":  "none",
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
	hasRoom := user.HasRoom
	hasRequest := user.HasRequest

	var acceptedRequestCount int64
	// ユーザーの入室申請のステータスが"accepted"であるかどうかをチェック
	err = db.Model(&models.Challenger{}).Where("user_id = ? AND status = 'accepted'", userID).Count(&acceptedRequestCount).Error
	replyStatus := "none"
	if err == nil && acceptedRequestCount > 0 {
		replyStatus = "accepted"
	}

	// ユーザーが作成したルームに対して"pending"状態の入室申請があるかどうかをチェック
	var pendingRequestExists bool
	db.Model(&models.Challenger{}).
		Joins("join game_rooms on game_rooms.id = challengers.game_room_id").
		Where("game_rooms.user_id = ? AND challengers.status = 'pending'", userID).
		Select("count(*) > 0").Find(&pendingRequestExists)

	// ユーザーが作成したルームに対して"accepted"状態の入室申請があるかどうかをチェック
	var acceptedRequestExists bool
	db.Model(&models.Challenger{}).
		Joins("join game_rooms on game_rooms.id = challengers.game_room_id").
		Where("game_rooms.user_id = ? AND challengers.status = 'accepted'", userID).
		Select("count(*) > 0").Find(&acceptedRequestExists)

	var roomStatus string
	if acceptedRequestExists {
		roomStatus = "sent"
	} else if pendingRequestExists {
		roomStatus = "waiting"
	} else {
		roomStatus = "none"
	}

	response := gin.H{
		"hasToken":    userToken,
		"hasRoom":     hasRoom,
		"hasRequest":  hasRequest,
		"replyStatus": replyStatus,
		"roomStatus":  roomStatus,
	}

	c.JSON(http.StatusOK, response)
}

// // ListHandler は未実装のリスト画面用のハンドラです
// func ListHandler(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
// 	// トークンからユーザーIDを取得
// 	userID, err := GetUserIDFromToken(c, logger)
// 	if err != nil {
// 		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
// 		return
// 	}

// 	var rooms []struct {
// 		CreatedAt        time.Time `json:"createdAt"`
// 		ChallengersCount int       `json:"challengersCount"`
// 		RoomTheme        string    `json:"roomTheme"`
// 	}
// 	// ユーザーが作成したゲームルームの情報を取得
// 	if err := db.Model(&models.GameRoom{}).
// 		Where("user_id = ?", userID).
// 		Select("created_at", "challengers_count", "room_theme").
// 		Find(&rooms).Error; err != nil {
// 		logger.Error("Failed to retrieve rooms list", zap.Error(err))
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve rooms list"})
// 		return
// 	}

// 	var requests []struct {
// 		RoomCreator string `json:"roomCreator"`
// 	}
// 	// ユーザーが送信した入室申請の情報を取得
// 	if err := db.Model(&models.Challenger{}).
// 		Joins("join game_rooms on game_rooms.id = challengers.game_room_id").
// 		Where("challengers.user_id = ?", userID).
// 		Select("game_rooms.room_creator").
// 		Find(&requests).Error; err != nil {
// 		logger.Error("Failed to retrieve requests list", zap.Error(err))
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve requests list"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"rooms":    rooms,
// 		"requests": requests,
// 	})
// }
