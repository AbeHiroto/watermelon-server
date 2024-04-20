package actions

import (
	"encoding/json"
	"time"

	"xicserver/models"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// チャットメッセージを処理する関数
func handleChatMessage(client *models.Client, msg map[string]interface{}, clients map[*models.Client]bool, logger *zap.Logger) {
	// ここではmsgからチャットメッセージを取り出す
	chatMessage := msg["message"].(string)

	// 現在のタイムスタンプを取得
	timestamp := time.Now().Format(time.RFC3339)

	logger.Info("Received chat message",
		zap.String("message", chatMessage),
		zap.Uint("from", client.UserID),
		zap.String("timestamp", timestamp),
	)

	// ゲームルーム内の全クライアントにメッセージをブロードキャストする
	for c := range clients {
		// 同じゲームルーム内のクライアントにのみメッセージを送信するロジック
		if c.RoomID == client.RoomID {
			message := map[string]interface{}{
				"type":      "chatMessage",
				"message":   chatMessage,
				"from":      client.UserID, // 送信者の識別子
				"timestamp": timestamp,     // メッセージのタイムスタンプ
			}
			messageJSON, _ := json.Marshal(message)
			if err := c.Conn.WriteMessage(websocket.TextMessage, messageJSON); err != nil {
				logger.Error("Failed to send chat message",
					zap.Uint("to", c.UserID),
					zap.Error(err),
				)
			} else {
				logger.Info("Chat message sent",
					zap.Uint("to", c.UserID),
				)
			}
		}
	}
}

// // handleMessage handles incoming messages from clients
// func handleMessage(client *Client, messageType int, payload []byte) {
// 	// Process message
// }
