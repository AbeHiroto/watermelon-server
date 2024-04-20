package actions

import (
	"math/rand"

	"xicserver/bribe/broadcast"
	"xicserver/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func handleMarkCell(client *models.Client, msg map[string]interface{}, game *models.Game, randGen *rand.Rand, db *gorm.DB, logger *zap.Logger) {
	// msgからセルの位置を取得
	x, okX := msg["x"].(int)
	y, okY := msg["y"].(int)
	if !okX || !okY || x < 0 || y < 0 || x >= len(game.Board) || y >= len(game.Board[0]) {
		sendErrorMessage(client, "Invalid cell coordinates")
		return
	}

	// 選択されたセルが空かどうかチェック
	if game.Board[x][y] != "" {
		sendErrorMessage(client, "Cell is already marked")
		return
	}

	// クライアントのUserIDがCurrentTurnと一致するか確認
	if game.CurrentTurn != client.UserID {
		sendErrorMessage(client, "Not your turn")
		return
	}

	currentPlayerIndex := 0
	if game.Players[1].ID == client.UserID {
		currentPlayerIndex = 1
	}
	biasAdvantage := game.BiasDegree * (1 - 2*currentPlayerIndex)

	markDecisionMade := false
	if biasAdvantage > 0 || (biasAdvantage == 0 && randGen.Float32() < 0.3) {
		// 選択されたセルに印を置く
		game.Board[x][y] = getCurrentPlayerSymbol(client, game)
		markDecisionMade = true
	}

	if !markDecisionMade {
		// 空のセルのリストを取得し、ランダムに選ぶ
		emptyCells := getEmptyCellsExcept(game.Board, x, y)
		if len(emptyCells) > 0 {
			randIndex := randGen.Intn(len(emptyCells))
			chosenCell := emptyCells[randIndex]
			game.Board[chosenCell[0]][chosenCell[1]] = getCurrentPlayerSymbol(client, game)
			markDecisionMade = true
		} else {
			// 空のセルが選択されたセル以外に存在しない場合は、選択されたセルに印を置く
			game.Board[x][y] = getCurrentPlayerSymbol(client, game)
			markDecisionMade = true
		}
	}

	// 印が確実に置かれた場合にのみ実行する
	if markDecisionMade {
		// 審判の状態とカウントダウンを管理
		if game.RefereeCount > 0 {
			game.RefereeCount--

			// RefereeCountが0になったらRefereeStatusを"normal"に戻す
			if game.RefereeCount == 0 && game.RefereeStatus != "normal" {
				game.RefereeStatus = "normal"
			}
		}
	}

	// 勝敗判定とゲーム状態の更新
	checkAndUpdateGameStatus(game, db, logger)
}

// 指定されたセルを除いた空のセルのリストを返すヘルパー関数
func getEmptyCellsExcept(board [][]string, excludeX, excludeY int) [][2]int {
	var emptyCells [][2]int
	for i, row := range board {
		for j, cell := range row {
			if cell == "" && !(i == excludeX && j == excludeY) {
				emptyCells = append(emptyCells, [2]int{i, j})
			}
		}
	}
	return emptyCells
}

func checkAndUpdateGameStatus(game *models.Game, db *gorm.DB, logger *zap.Logger) {
	// ボードのサイズに基づいて勝利条件を設定
	winCondition := 3 // デフォルトは3x3のボードでの勝利条件
	if len(game.Board) == 5 && len(game.Board[0]) == 5 {
		winCondition = 4 // 5x5のボードでは勝利条件を4に設定
	}

	// 現在のプレイヤーのシンボルを取得
	currentPlayerSymbol := ""
	for _, player := range game.Players {
		if player.ID == game.CurrentTurn {
			currentPlayerSymbol = player.Symbol
			break
		}
	}

	// 勝敗判定
	var nextRoundStatus string
	if checkWin(game.Board, currentPlayerSymbol, winCondition) {
		// 勝者がいる場合
		game.Winners = append(game.Winners, game.CurrentTurn) // 勝者のIDを追加
		// 現在のラウンドに応じて次のステータスを設定
		switch game.Status {
		case "round1":
			nextRoundStatus = "round1_finished"
		case "round2":
			nextRoundStatus = "round2_finished"
		case "round3":
			nextRoundStatus = "finished" // 3回戦が最後なので、ここでゲーム全体を終了
		}
	} else if isBoardFull(game.Board) {
		// ボードが全て埋まっているが、勝者がいない場合（引き分け）
		game.Winners = append(game.Winners, 0) // 引き分けを示すために特別な値（ここでは0）を追加
		// 同じく現在のラウンドに応じて次のステータスを設定
		switch game.Status {
		case "round1":
			nextRoundStatus = "round1_finished"
		case "round2":
			nextRoundStatus = "round2_finished"
		case "round3":
			nextRoundStatus = "finished" // 引き分けでも3回戦が最後
		}
	} else {
		// ゲームが続行する場合、ターン更新
		if game.CurrentTurn == game.Players[0].ID {
			game.CurrentTurn = game.Players[1].ID
		} else {
			game.CurrentTurn = game.Players[0].ID
		}
	}

	// ステータスの更新が必要な場合（勝者が決定した場合や引き分けの場合）のみ、ステータスを更新
	if nextRoundStatus != "" {
		game.Status = nextRoundStatus
		if game.Status == "finished" {
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
			} else {
				broadcast.BroadcastResults(game, logger) // Only broadcast results if transaction was successful
			}
		} else {
			broadcast.BroadcastGameState(game, logger)
		}
	} else {
		broadcast.BroadcastGameState(game, logger)
	}
}

func checkWin(board [][]string, symbol string, winCondition int) bool {
	size := len(board)

	// 横列のチェック
	for row := 0; row < size; row++ {
		count := 0
		for col := 0; col < size; col++ {
			if board[row][col] == symbol {
				count++
			}
		}
		if count == winCondition {
			return true
		}
	}

	// 縦列のチェック
	for col := 0; col < size; col++ {
		count := 0
		for row := 0; row < size; row++ {
			if board[row][col] == symbol {
				count++
			}
		}
		if count == winCondition {
			return true
		}
	}

	// 斜め（左上から右下）のチェック
	count := 0
	for index := 0; index < size; index++ {
		if board[index][index] == symbol {
			count++
		}
	}
	if count == winCondition {
		return true
	}

	// 斜め（右上から左下）のチェック
	count = 0
	for index := 0; index < size; index++ {
		if board[index][size-index-1] == symbol {
			count++
		}
	}
	if count == winCondition {
		return true
	}

	return false
}

// マス目がすべて埋まっているかどうかの確認
func isBoardFull(board [][]string) bool {
	for _, row := range board {
		for _, cell := range row {
			if cell == "" {
				return false
			}
		}
	}
	return true
}
