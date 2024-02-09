package handlers

import (
	"net/http"
	"strings"

	"xicserver/models"

	"go.uber.org/zap"
	"gorm.io/gorm"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

// 作成者が自分のルームをタップして表示される情報を取得するハンドラー
func myRoomInfoHandler(c *gin.Context, db *gorm.DB, logger *zap.Logger) {
	// JWTトークンからユーザーIDを取得
	userID, err := getUserIDFromToken(c, logger)
	if err != nil {
		logger.Error("Failed to get user ID from token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "認証に失敗しました"})
		return
	}

	// ユーザーが作成したルームのIDを取得（例：URLパラメータから取得）
	roomID := c.Param("roomID")

	// ルームの所有者かどうかを確認
	var room models.GameRoom
	if err := db.Where("id = ? AND user_id = ?", roomID, userID).First(&room).Error; err != nil {
		logger.Error("Room not found or not owned by the user", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "ルームが見つからないか、ユーザーが所有していません"})
		return
	}

	// ルームIDに基づいて、該当する入室申請情報を取得
	var challengers []struct {
		VisitorID          uint   `json:"visitorId"`
		ChallengerNickname string `json:"challengerNickname"`
		Status             string `json:"status"`
	}
	if err := db.Model(&models.Challenger{}).Select("visitor_id", "challenger_nickname", "status").
		Where("game_room_id = ?", roomID).Scan(&challengers).Error; err != nil {
		logger.Error("Failed to retrieve challengers list", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "入室申請一覧の取得に失敗しました"})
		return
	}

	// 申請一覧をレスポンスとして返す
	c.JSON(http.StatusOK, gin.H{"challengers": challengers})
}

// getUserIDFromToken はリクエストからJWTトークンを取得し、ユーザーIDを解析して返します。
func getUserIDFromToken(c *gin.Context, logger *zap.Logger) (uint, error) {
	// トークンをリクエストヘッダーから取得
	tokenString := c.GetHeader("Authorization")

	// Bearerトークンのプレフィックスを削除
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

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
