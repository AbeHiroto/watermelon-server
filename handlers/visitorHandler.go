package handlers

// import (
// 	"net/http"

// 	"github.com/gin-gonic/gin"
// )

// // リクエストデータ用の構造体
// type GameRoomRequest struct {
// 	UserID             string `json:"userID" binding:"required"`
// 	SubscriptionStatus string `json:"subscriptionStatus" binding:"required"`
// 	// その他のフィールド...
// }

// // ハンドラー関数
// func CreateGameRoom(c *gin.Context) {
// 	var req GameRoomRequest
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
// 		return
// 	}

// 	// バリデーションが成功した場合の処理...
// }
