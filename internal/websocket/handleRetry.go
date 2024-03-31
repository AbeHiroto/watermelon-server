package websocket

import (
	"encoding/json"
	"time"

	// "math/rand"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

func handleRetry(game *Game, client *Client, msg map[string]interface{}, logger *zap.Logger) {
	// すでに終了したゲームではない、または再戦リクエストを受け付ける状態でない場合は早期リターン
	if game.Status != "round1_finished" && game.Status != "round2_finished" {
		logger.Info("Retry request is not applicable.")
		return
	}

	// msgから再戦リクエストを取得
	wantRetry, ok := msg["wantRetry"].(bool)
	if !ok {
		// wantRetryの値が正しく取得できない場合のエラーハンドリング
		logger.Error("Invalid retry request", zap.Any("message", msg))
		return
	}

	// 再戦リクエストをGame構造体に記録
	game.RetryRequests[client.UserID] = wantRetry

	// 再戦を望まない場合、ゲームを直ちに終了させる
	if !wantRetry {
		game.Status = "finished"
		broadcastGameState(game, logger)
		return
	} else {
		// 再戦を望む場合、対戦相手に通知する
		opponentID := game.Players[0].ID
		if client.UserID == game.Players[0].ID {
			opponentID = game.Players[1].ID
		}
		sendRetryRequestNotification(client.UserID, opponentID, clients, logger)
	}

	// 両方のプレイヤーからの再戦リクエストを確認
	if len(game.RetryRequests) == 2 {
		if game.RetryRequests[game.Players[0].ID] && game.RetryRequests[game.Players[1].ID] {
			game.Status = getNextRoundStatus(game.Status)
			resetGameForNextRound(game)
			broadcastGameState(game, logger)
		} else {
			game.Status = "finished"
			broadcastGameState(game, logger)
		}
	}
}

// 次のラウンドのステータスを返すヘルパー関数
func getNextRoundStatus(currentStatus string) string {
	switch currentStatus {
	case "round1_finished":
		return "round2"
	case "round2_finished":
		return "round3"
	default:
		return "finished"
	}
}

// ゲームを次のラウンドに向けてリセットするヘルパー関数
func resetGameForNextRound(game *Game) {
	// ボードのリセット
	for i := range game.Board {
		for j := range game.Board[i] {
			game.Board[i][j] = ""
		}
	}

	// その他の状態のリセット
	game.BribeCounts = [2]int{0, 0}
	game.BiasDegree = 0
	game.RefereeStatus = "normal"
	game.RefereeCount = 0
	// 必要に応じてその他のフィールドをリセット
}

func sendRetryRequestNotification(fromUserID uint, toUserID uint, clients map[*Client]bool, logger *zap.Logger) {
	for c := range clients {
		if c.UserID == toUserID {
			chatMessage := "対戦相手が再戦を希望しています。"
			timestamp := time.Now().Format(time.RFC3339)
			message := map[string]interface{}{
				"type":      "chatMessage",
				"message":   chatMessage,
				"from":      fromUserID,
				"timestamp": timestamp,
			}
			messageJSON, _ := json.Marshal(message)
			if err := c.Conn.WriteMessage(websocket.TextMessage, messageJSON); err != nil {
				logger.Error("Failed to send retry request notification",
					zap.Uint("to", toUserID),
					zap.Error(err),
				)
			} else {
				logger.Info("Retry request notification sent",
					zap.Uint("to", toUserID),
				)
			}
		}
	}
}
