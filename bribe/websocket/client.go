package websocket

import (
	"encoding/json"
	"math/rand"

	"xicserver/models"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Helper function to send error message to the client via WebSocket
func sendErrorMessage(client *models.Client, errorMessage string) {
	errorResponse := map[string]string{"error": errorMessage}
	errorJSON, _ := json.Marshal(errorResponse)
	client.Conn.WriteMessage(websocket.TextMessage, errorJSON) // Ignoring error for simplicity
}

// 現在のプレイヤーのシンボルを取得するヘルパー関数
func getCurrentPlayerSymbol(client *models.Client, game *models.Game) string {
	if game.Players[0].ID == client.UserID {
		return game.Players[0].Symbol
	}
	return game.Players[1].Symbol
}

// クライアントごとにメッセージ読み取りするゴルーチン
func handleClient(client *models.Client, clients map[*models.Client]bool, games map[uint]*models.Game, randGen *rand.Rand, db *gorm.DB, logger *zap.Logger) {
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
			break
		}

		// 受信したメッセージをJSON形式でデコード
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.Error("Error decoding message", zap.Error(err))
			continue
		}

		game, exists := games[client.RoomID]
		if !exists {
			// ゲームが見つからないエラー処理
			sendErrorMessage(client, "Game not found")
			continue
		}

		// メッセージタイプに基づいて適切なアクションを実行
		switch msg["type"].(string) {
		case "markCell", "bribe", "accuse", "retry":
			// ここでさらにアクションタイプに応じて処理を分岐
			actionType := msg["actionType"].(string)
			switch actionType {
			case "markCell":
				handleMarkCell(client, msg, game, randGen, db, logger)
			case "bribe":
				handleBribe(game, client, logger)
			case "accuse":
				handleAccuse(game, client, logger)
			case "retry":
				// "retry"メッセージタイプの場合、再戦リクエストの処理
				handleRetry(game, client, clients, msg, logger)
			default:
				logger.Info("Unknown action type", zap.String("actionType", actionType))
			}
		case "chatMessage":
			handleChatMessage(client, msg, clients, logger)
		default:
			logger.Info("Received unknown message type", zap.Any("message", msg))
		}
	}
}
