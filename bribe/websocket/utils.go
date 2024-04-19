package websocket

import (
	"encoding/json"
	"math/rand"

	"time"
	"xicserver/models"

	"go.uber.org/zap"

	"github.com/gorilla/websocket"
)

// 乱数は先攻後攻の決定や選択したセルに印を置かれるかなどの決定に使用
func createLocalRandGenerator() *rand.Rand {
	source := rand.NewSource(time.Now().UnixNano())
	return rand.New(source)
}

// ゲームの状態をブロードキャストするヘルパー関数
func broadcastGameState(game *models.Game, logger *zap.Logger) {
	playersInfo := make([]map[string]interface{}, len(game.Players))
	for i, player := range game.Players {
		if player != nil {
			playersInfo[i] = map[string]interface{}{
				"id":       player.ID,
				"nickName": player.NickName,
				"symbol":   player.Symbol,
			}
		}
	}

	gameState := map[string]interface{}{
		"type":          "gameState",
		"board":         game.Board,
		"currentTurn":   game.CurrentTurn,
		"status":        game.Status,
		"playersOnline": game.PlayersOnlineStatus,
		"playersInfo":   playersInfo,
		"bias":          game.Bias,
		"refereeStatus": game.RefereeStatus,
		"winners":       game.Winners,
	}
	messageJSON, _ := json.Marshal(gameState)

	for _, player := range game.Players {
		if player != nil {
			if err := player.Conn.WriteMessage(websocket.TextMessage, messageJSON); err != nil {
				logger.Error("Failed to broadcast game state", zap.Error(err))
			}
		}
	}
}

func notifyOpponentOnlineStatus(roomID uint, userID uint, isOnline bool, clients map[*models.Client]bool, logger *zap.Logger) {
	for client := range clients {
		if client.RoomID == roomID && client.UserID != userID {
			onlineStatusMessage := map[string]interface{}{
				"type":     "onlineStatus",
				"userID":   userID,
				"isOnline": isOnline,
			}
			messageJSON, err := json.Marshal(onlineStatusMessage)
			if err != nil {
				logger.Error("Failed to marshal online status message", zap.Error(err))
				continue
			}
			if err := client.Conn.WriteMessage(websocket.TextMessage, messageJSON); err != nil {
				logger.Error("Failed to send online status message", zap.Error(err))
			}
		}
	}
}

// func generateGameID() string {
// 	return uuid.New().String() // UUIDを生成して返す
// }
