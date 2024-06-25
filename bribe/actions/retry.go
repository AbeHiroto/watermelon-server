package actions

import (
	"encoding/json"
	"time"

	// "math/rand"
	"xicserver/bribe/broadcast"
	"xicserver/models"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func handleRetry(game *models.Game, client *models.Client, clients map[*models.Client]bool, msg map[string]interface{}, logger *zap.Logger, db *gorm.DB) {
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

	// "game.RetryRequests"を初期化する
	if game.RetryRequests == nil {
		game.RetryRequests = make(map[uint]bool)
	}
	game.RetryRequests[client.UserID] = wantRetry

	// 再戦を望まない場合、ゲームを直ちに終了させる
	if !wantRetry {
		game.Status = "finished"
		broadcast.BroadcastGameState(game, logger)

		// データベースを更新
		err := db.Transaction(func(tx *gorm.DB) error {
			// Update game state in the database
			if err := tx.Model(&models.GameRoom{}).Where("id = ?", game.ID).Update("game_state", "finished").Error; err != nil {
				return err
			}

			// Update the room creator's HasRoom to false
			var gameRoom models.GameRoom
			if err := tx.Where("id = ?", game.ID).First(&gameRoom).Error; err != nil {
				return err
			}

			if err := tx.Model(&models.User{}).Where("id = ?", gameRoom.UserID).Update("has_room", false).Error; err != nil {
				return err
			}

			// Find all users with 'accepted' requests for this room and update their HasRequest to false
			var challengers []models.Challenger
			if err := tx.Where("game_room_id = ? AND status = 'accepted'", gameRoom.ID).Find(&challengers).Error; err != nil {
				return err
			}

			for _, challenger := range challengers {
				if err := tx.Model(&models.User{}).Where("id = ?", challenger.UserID).Update("has_request", false).Error; err != nil {
					return err
				}
			}

			return nil
		})

		if err != nil {
			logger.Error("Failed to finalize game room updates", zap.Error(err))
		}
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
			broadcast.BroadcastGameState(game, logger)
		} else {
			game.Status = "finished"
			broadcast.BroadcastGameState(game, logger)

			// データベースを更新
			err := db.Transaction(func(tx *gorm.DB) error {
				// Update game state in the database
				if err := tx.Model(&models.GameRoom{}).Where("id = ?", game.ID).Update("game_state", "finished").Error; err != nil {
					return err
				}

				// Update the room creator's HasRoom to false
				var gameRoom models.GameRoom
				if err := tx.Where("id = ?", game.ID).First(&gameRoom).Error; err != nil {
					return err
				}

				if err := tx.Model(&models.User{}).Where("id = ?", gameRoom.UserID).Update("has_room", false).Error; err != nil {
					return err
				}

				// Find all users with 'accepted' requests for this room and update their HasRequest to false
				var challengers []models.Challenger
				if err := tx.Where("game_room_id = ? AND status = 'accepted'", gameRoom.ID).Find(&challengers).Error; err != nil {
					return err
				}

				for _, challenger := range challengers {
					if err := tx.Model(&models.User{}).Where("id = ?", challenger.UserID).Update("has_request", false).Error; err != nil {
						return err
					}
				}

				return nil
			})

			if err != nil {
				logger.Error("Failed to finalize game room updates", zap.Error(err))
			}
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
func resetGameForNextRound(game *models.Game) {
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

func sendRetryRequestNotification(fromUserID uint, toUserID uint, clients map[*models.Client]bool, logger *zap.Logger) {
	for c := range clients {
		if c.UserID == toUserID {
			chatMessage := "SYSTEM: Your opponent sent retry request!"
			timestamp := time.Now().Format(time.RFC3339)
			message := map[string]interface{}{
				"type":    "chatMessage",
				"message": chatMessage,
				"from":    0,
				// "from":      fromUserID,
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
