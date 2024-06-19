package database

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"xicserver/models"

	"go.uber.org/zap"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ValidateSessionID checks the session ID from Redis and returns the client if the session is valid.
func ValidateSessionID(ctx context.Context, r *http.Request, rdb *redis.Client, sessionID string, logger *zap.Logger) *models.Client {
	if sessionID == "" {
		logger.Error("Session ID is empty")
		return nil
	}

	sessionInfoJSON, err := rdb.Get(ctx, "session:"+sessionID).Result()
	if err != nil {
		logger.Error("Failed to retrieve session info", zap.Error(err))
		return nil
	}

	var sessionInfo map[string]interface{}
	if err := json.Unmarshal([]byte(sessionInfoJSON), &sessionInfo); err != nil {
		logger.Error("Failed to decode session info", zap.Error(err))
		return nil
	}

	userID, ok := sessionInfo["userID"].(float64) // JSONの数値はfloat64としてデコードされます
	if !ok {
		logger.Error("Invalid session info: missing userID")
		return nil
	}
	roomID, ok := sessionInfo["roomID"].(float64)
	if !ok {
		logger.Error("Invalid session info: missing roomID")
		return nil
	}
	role, ok := sessionInfo["role"].(string)
	if !ok {
		logger.Error("Invalid session info: missing role")
		return nil
	}

	// 有効なセッション情報を基にClientオブジェクトを作成
	client := &models.Client{
		UserID: uint(userID),
		RoomID: uint(roomID),
		Role:   role,
	}
	return client
}

func GenerateAndStoreSessionID(ctx context.Context, client *models.Client, rdb *redis.Client, logger *zap.Logger) error {
	sessionID := uuid.New().String()

	// セッション情報をJSON形式でエンコード
	sessionInfo := map[string]interface{}{
		"userID": client.UserID,
		"roomID": client.RoomID,
		"role":   client.Role,
	}
	sessionInfoJSON, err := json.Marshal(sessionInfo)
	if err != nil {
		logger.Error("Error encoding session info", zap.Error(err))
		return err
	}

	// セッションIDとセッション情報をRedisに保存
	err = rdb.Set(ctx, "session:"+sessionID, sessionInfoJSON, 24*time.Hour).Err() // 24時間の有効期限
	if err != nil {
		logger.Error("Error storing session info in Redis", zap.Error(err))
		return err
	}

	// セッションIDをクライアントに送り返す
	return sendSessionIDToClient(client, sessionID, logger)
}

func sendSessionIDToClient(client *models.Client, sessionID string, logger *zap.Logger) error {
	// セッションIDをクライアントに送信するためのレスポンスを作成
	response := map[string]interface{}{
		"sessionID": sessionID,
		"userID":    client.UserID,
	}
	//response := map[string]string{"sessionID": sessionID}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		logger.Error("Error marshalling session ID response", zap.Error(err))
		return err
	}

	// クライアントにセッションIDを含むレスポンスを送信
	if client.Conn != nil {
		if err := client.Conn.WriteMessage(websocket.TextMessage, responseJSON); err != nil {
			logger.Error("Error sending session ID to client", zap.Error(err))
			return err
		}
		logger.Info("Successfully sent session ID to client", zap.String("sessionID", sessionID))
	} else {
		logger.Warn("WebSocket connection is not established, cannot send session ID")
	}

	return nil
}
