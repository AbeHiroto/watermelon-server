package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"xicserver/auth"
	"xicserver/middlewares"
	"xicserver/models"

	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	jwt "github.com/dgrijalva/jwt-go"
)

var db *gorm.DB // GORMデータベース接続を保持するグローバル変数

var logger *zap.Logger

func init() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
}

func RoomCreate(c *gin.Context, db *gorm.DB) {
	var request models.RoomCreateRequest
	var err error
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error("Room create request bind error", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "request_binding_error",
			"message": err.Error(),
		})
		return
	}

	var userID uint
	var newToken string
	var tokenValid = false // トークンの有効性を判定するためのフラグ

	// トークンが提供されている場合
	if request.Token != "" {
		claims := &models.MyClaims{}
		token, err := jwt.ParseWithClaims(request.Token, claims, func(token *jwt.Token) (interface{}, error) {
			return auth.JwtKey, nil
		})

		if err != nil || !token.Valid {
			logger.Error("Token validation error", zap.Error(err))
			newToken, userID, err = middlewares.GenerateToken(db, request.SubscriptionStatus, 0)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  "token_invalid_error",
					"message": "トークン生成に失敗しました",
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"status":   "token_invalid",
				"newToken": newToken,
			})
			return
		} else {
			userID = claims.UserID
			// トークンの有効期限が1時間未満の場合は新しいトークンを生成
			if time.Unix(claims.ExpiresAt, 0).Sub(time.Now()) < time.Hour {
				newToken, _, err = middlewares.GenerateToken(db, claims.SubscriptionStatus, userID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"status":  "token_expired_error",
						"message": "トークン生成に失敗しました",
					})
					return
				}
				// 新しいトークンをクライアントに返す
				c.JSON(http.StatusOK, gin.H{
					"status": "token_expired",
					"token":  newToken,
				})
				return // ここで処理を終了し、ゲームルーム作成をスキップ
			} else {
				tokenValid = true // トークンが有効かつ有効期限が1時間以上ある場合tokenValid関数↓が有効化(true)
			}
		}

	} else { //トークンを送るコード追加
		newToken, userID, err = middlewares.GenerateToken(db, request.SubscriptionStatus, 0)
		if err != nil {
			logger.Error("Token generation error", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "no_token_error",
				"error":  "トークンがありません",
			})
			return
		}
		// 新しく生成されたトークンをクライアントに返す
		c.JSON(http.StatusOK, gin.H{
			"status": "no_token",
			"token":  newToken,
		})
		return
	}

	// 一意の招待URLを生成し、重複がないことを確認する部分
	var uniqueToken string
	for {
		bytes := make([]byte, 8) // 64ビットの乱数を生成
		_, err := rand.Read(bytes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate unique token"})
			return
		}
		uniqueToken = hex.EncodeToString(bytes) // 16進数の文字列に変換

		// 生成されたトークンがデータベース内で既に使用されていないかを確認
		var exists bool
		db.Model(&models.GameRoom{}).Select("count(*) > 0").Where("unique_token = ?", uniqueToken).Find(&exists)
		if !exists {
			break // 重複がなければループを抜ける
		}
		// 重複があればループを続け、新しいトークンを生成
	}

	// トークンが有効な場合のみゲームルームを作成
	if tokenValid {
		// アクティブなゲームルームの数を確認
		var count int64
		db.Model(&models.GameRoom{}).
			Where("room_creator_id = ? AND game_state IN ('created', 'visitors', 'using')", userID).
			Count(&count)

		maxRoomCount := 5 // ユーザーごとのゲームルーム作成上限数
		if count >= int64(maxRoomCount) {
			// ゲームルーム作成上限に達している場合はエラーを返す
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "room_limit",
				"error":  "Reached the limit of active rooms",
			})
			return
		}

		newGameRoom := models.GameRoom{
			UserID:      userID, // トークンから取得したユーザーID
			RoomCreator: request.Nickname,
			GameState:   "created",
			UniqueToken: uniqueToken,
			RoomTheme:   request.RoomTheme,
			// その他の初期値設定...
		}
		if err := db.Create(&newGameRoom).Error; err != nil {
			logger.Error("Failed to create a new game room", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ゲームルーム作成に失敗しました"})
			return
		}
		//ここでゲームルーム作成
		c.JSON(http.StatusOK, gin.H{
			"status":      "success",
			"gameRoomID":  newGameRoom.ID,
			"uniqueToken": newGameRoom.UniqueToken,
		})
	} else {
		// トークンが無効な場合、新しいトークンを返す
		c.JSON(http.StatusOK, gin.H{
			"status": "no_token",
			"token":  newToken,
		})
	}

	// ゲームルーム作成成功後にユーザーのValidRoomCountをインクリメント
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		logger.Error("Failed to fetch user for updating room count", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user room count"})
		return
	}

	user.ValidRoomCount += 1
	if err := db.Save(&user).Error; err != nil {
		logger.Error("Failed to increment user's valid room count", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to increment room count"})
		return
	}
}
