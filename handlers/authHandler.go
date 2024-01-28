package handlers

import (
	"net/http"
	"xicserver/middlewares"
	"xicserver/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ユーザー登録リクエストの構造体
type RegisterRequest struct {
	// 必要に応じてフィールドを追加
}

// ユーザー登録ハンドラー
func RegisterUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		user := models.User{
			// 他のフィールド（SubscriptionStatus, ValidRoomCountなど）の初期化
		}

		result := db.Create(&user) // ユーザーをデータベースに保存
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User registered", "userId": user.ID})
	}
}

// トークン生成リクエストの構造体
type TokenRequest struct {
	UserID string `json:"userId" binding:"required"`
	// 他のフィールド...
}

// GenerateToken - トークン生成ハンドラー
func GenerateToken(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TokenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var user models.User
		if err := db.Where("user_id = ?", req.UserID).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		token, err := middlewares.GenerateToken(user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": token})
	}
}
