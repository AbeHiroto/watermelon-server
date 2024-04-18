package websocket

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func generateAndStoreSessionID(ctx context.Context, client *Client, rdb *redis.Client, logger *zap.Logger) error {
	sessionID := uuid.New().String()

	// セッション情報をJSON形式でエンコード
	sessionInfo := map[string]interface{}{
		"userID": client.UserID,
		"roomID": client.RoomID,
		"role":   client.Role,
		// "ipAddress": clientIpAddress, // クライアントのIPアドレスを取得するロジックが必要
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

func sendSessionIDToClient(client *Client, sessionID string, logger *zap.Logger) error {
	// セッションIDをクライアントに送信するためのレスポンスを作成
	response := map[string]string{"sessionID": sessionID}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		logger.Error("Error marshalling session ID response", zap.Error(err))
		return err
	}

	// クライアントにセッションIDを含むレスポンスを送信
	if err := client.Conn.WriteMessage(websocket.TextMessage, responseJSON); err != nil {
		logger.Error("Error sending session ID to client", zap.Error(err))
		return err
	}

	return nil
}
