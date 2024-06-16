package broadcast

import (
	"encoding/json"

	"xicserver/models"

	"go.uber.org/zap"

	"github.com/gorilla/websocket"
)

// ゲームの状態をブロードキャストするヘルパー関数
func BroadcastGameState(game *models.Game, logger *zap.Logger) {
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
		"bribeCounts":   game.BribeCounts,
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

func BroadcastResults(game *models.Game, logger *zap.Logger) {
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

	results := map[string]interface{}{
		"type":          "gameResults",
		"bribeCounts":   game.BribeCounts,
		"board":         game.Board,
		"currentTurn":   game.CurrentTurn,
		"status":        game.Status,
		"playersOnline": game.PlayersOnlineStatus,
		"playersInfo":   playersInfo,
		"bias":          game.Bias,
		"refereeStatus": game.RefereeStatus,
		"winners":       game.Winners,
	}
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		logger.Error("Failed to marshal game results", zap.Error(err))
		return
	}

	// ゲームに参加している全プレイヤーに結果をブロードキャスト
	for _, player := range game.Players {
		if player != nil && player.Conn != nil {
			if err := player.Conn.WriteMessage(websocket.TextMessage, resultsJSON); err != nil {
				logger.Error("Failed to broadcast game results", zap.Error(err))
			}
		}
	}
}

func NotifyOpponentOnlineStatus(roomID uint, userID uint, isOnline bool, clients map[*models.Client]bool, logger *zap.Logger) {
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
