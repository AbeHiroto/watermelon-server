package websocket

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Helper function to send error message to the client via WebSocket
func sendErrorMessage(client *Client, errorMessage string) {
	errorResponse := map[string]string{"error": errorMessage}
	errorJSON, _ := json.Marshal(errorResponse)
	client.Conn.WriteMessage(websocket.TextMessage, errorJSON) // Ignoring error for simplicity
}

// クライアントごとにメッセージ読み取りするゴルーチン
func handleClient(client *Client, clients map[*Client]bool, logger *zap.Logger) {
	defer func() {
		client.Conn.Close()     // クライアントの接続を閉じる
		delete(clients, client) // クライアントリストからこのクライアントを削除
	}()

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("WebSocket error", zap.Error(err))
			}
			break // エラーが発生したらループを抜ける
		}

		// 受信したメッセージをJSON形式でデコード
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.Error("Error decoding message", zap.Error(err))
			continue
		}

		// メッセージタイプに基づいて適切なアクションを実行
		switch msg["type"] {
		case "gameAction":
			handleGameAction(client, msg, clients, logger)
		case "chatMessage":
			handleChatMessage(client, msg, clients, logger)
		default:
			logger.Info("Received unknown message type", zap.Any("message", msg))
		}

		// // 受信したメッセージに対する処理
		// logger.Info("Received message", zap.ByteString("message", message))
	}
}

// ゲームアクションを処理する関数
func handleGameAction(client *Client, msg map[string]interface{}, clients map[*Client]bool, logger *zap.Logger) {
	// ゲームアクションの具体的な処理を実装
}

// チャットメッセージを処理する関数
func handleChatMessage(client *Client, msg map[string]interface{}, clients map[*Client]bool, logger *zap.Logger) {
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
