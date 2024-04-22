package connection

import (
	"context"

	"net/http"

	"strings"

	"fmt"

	"xicserver/auth"
	"xicserver/bribe/database"
	"xicserver/models"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-redis/redis/v8"
)

// ClientContext はクライアントのセッション情報を保持するための構造体です。
type ClientContext struct {
	UserID uint
	RoomID uint
	Role   string
	Claims *models.MyClaims // JWTクレームを含む
}

func FetchClientContext(ctx context.Context, r *http.Request, db *gorm.DB, logger *zap.Logger) (*ClientContext, error) {
	claims, err := TokenValidation(r, logger)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	var user models.User
	if err := db.First(&user, claims.UserID).Error; err != nil {
		logger.Error("Failed to fetch user", zap.Error(err))
		return nil, fmt.Errorf("user fetch failed: %w", err)
	}

	if !user.HasRoom && !user.HasRequest {
		return nil, fmt.Errorf("user has no active room or request")
	}

	var roomID uint
	var role string
	if user.HasRoom {
		role = "Creator"
		var gameRoom models.GameRoom
		if err := db.Where("user_id = ?", claims.UserID).First(&gameRoom).Error; err != nil {
			logger.Error("Failed to fetch game room", zap.Error(err))
			return nil, fmt.Errorf("game room fetch failed: %w", err)
		}
		roomID = gameRoom.ID
	} else if user.HasRequest {
		role = "Challenger"
		var challenger models.Challenger
		if err := db.Where("game_room_id = ?", claims.UserID).First(&challenger).Error; err != nil {
			logger.Error("Failed to fetch challenger data", zap.Error(err))
			return nil, fmt.Errorf("challenger fetch failed: %w", err)
		}
		roomID = challenger.GameRoomID
	} else {
		return nil, fmt.Errorf("unauthorized access: viewer role not permitted")
	}

	return &ClientContext{
		UserID: claims.UserID,
		RoomID: roomID,
		Role:   role,
		Claims: claims,
	}, nil
}

// TokenValidation 関数を新たに定義するか、FetchClientContext 内でトークン検証を実行します。
func TokenValidation(r *http.Request, logger *zap.Logger) (*models.MyClaims, error) {
	tokenString := r.Header.Get("Authorization")
	if strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	}

	claims := &models.MyClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return auth.JwtKey, nil
	})

	if err != nil || !token.Valid {
		logger.Error("Failed to validate token", zap.Error(err))
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	return claims, nil
}

// createNewSession handles creating a new session and returns a new client object
func CreateNewSession(ctx context.Context, r *http.Request, db *gorm.DB, rdb *redis.Client, logger *zap.Logger) *models.Client {
	client := new(models.Client)
	clientContext, err := FetchClientContext(ctx, r, db, logger)
	if err != nil {
		logger.Error("Error fetching client context", zap.Error(err))
		return nil
	}
	client.UserID = clientContext.UserID
	client.RoomID = clientContext.RoomID
	client.Role = clientContext.Role

	if err := database.GenerateAndStoreSessionID(ctx, client, rdb, logger); err != nil {
		logger.Error("Failed to generate or store session ID", zap.Error(err))
		return nil
	}
	return client
}
