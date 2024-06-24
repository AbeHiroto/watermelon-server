package actions

import (
	"xicserver/bribe/broadcast"
	"xicserver/models"

	"go.uber.org/zap"
)

func handleBribe(game *models.Game, client *models.Client, logger *zap.Logger) {
	// RefereeStatusが"normal"でなければ賄賂は無視
	if game.RefereeStatus != "normal" {
		logger.Info("Bribe ignored, referee status is not normal", zap.Uint("PlayerID", client.UserID))
		sendSystemMessage(client, "SYSTEM: Bribe ignored, referee status is not normal", logger)
		return
	}

	// 現在のターンのプレイヤーを特定
	var biasAdjustment int
	playerIndex := -1 //BribeCountsのインクリメントで使用
	if game.Players[0].ID == client.UserID {
		biasAdjustment = 1 // Players[0] が賄賂を贈った場合
		playerIndex = 0
	} else if game.Players[1].ID == client.UserID {
		biasAdjustment = -1 // Players[1] が賄賂を贈った場合
		playerIndex = 1
	} else {
		logger.Error("Player not found in the game", zap.Uint("PlayerID", client.UserID))
		return
	}

	// 賄賂回数のインクリメント
	if playerIndex != -1 {
		game.BribeCounts[playerIndex] += 1
	}

	// biasDegreeを更新
	newBiasDegree := game.BiasDegree + biasAdjustment
	// 新しいbiasDegreeが-1, 0, 1の範囲に収まるように調整
	if newBiasDegree >= -1 && newBiasDegree <= 1 {
		game.BiasDegree = newBiasDegree
	}

	sendSystemMessage(client, "SYSTEM: Your Bribe accepted!", logger)
	logger.Info("Bribe accepted", zap.Uint("PlayerID", client.UserID), zap.Int("NewBiasDegree", game.BiasDegree))

	// ゲーム状態のブロードキャスト
	broadcast.BroadcastGameState(game, logger)
}

func handleAccuse(game *models.Game, client *models.Client, logger *zap.Logger) {
	// 審判の状態が "normal" でない場合は、糾弾は無効
	if game.RefereeStatus != "normal" {
		logger.Info(" Accusation is ineffective! The referee is in an abnormal state.", zap.String("RefereeStatus", game.RefereeStatus))
		sendSystemMessage(client, "SYSTEM:Accusation is ineffective! The referee is in an abnormal state.", logger)
		return
	}

	// 対戦相手が賄賂を贈っていたかどうか判定し、対応する処理を実行
	// 一行目は審判が公平（BiasDegreeが"0"）だった場合
	if game.BiasDegree == 0 {
		if client.UserID == game.Players[0].ID {
			game.RefereeStatus = "angry"
			game.BiasDegree = -1
		} else if client.UserID == game.Players[1].ID {
			game.RefereeStatus = "angry"
			game.BiasDegree = 1
		}
		game.RefereeCount = 4 // ここでRefereeCountを設定
	} else if (client.UserID == game.Players[0].ID && game.BiasDegree < 0) ||
		(client.UserID == game.Players[1].ID && game.BiasDegree > 0) {
		// 対戦相手が賄賂を贈っていた場合
		game.RefereeStatus = "sad"
		game.BiasDegree *= -1 // BiasDegreeを反転させて、糾弾したプレイヤーに有利にする
		game.RefereeCount = 4 // ここでRefereeCountを設定
	} else {
		// 賄賂を贈っていたのが自分だった場合
		game.RefereeStatus = "angry"
		game.BiasDegree *= -1 // 既に有利なBiasDegreeが設定されているのでそれを反転させる
		game.RefereeCount = 4 // ここでRefereeCountを設定
	}

	logger.Info("Accusation has sent.", zap.Uint("PlayerID", client.UserID), zap.Int("NewBiasDegree", game.BiasDegree))

	// ゲーム状態のブロードキャスト
	broadcast.BroadcastGameState(game, logger)
}

func sendSystemMessage(client *models.Client, message string, logger *zap.Logger) {
	chatMessage := map[string]interface{}{
		"type":    "chatMessage",
		"message": message,
		"from":    0, // 0 indicates system message
	}
	err := client.Conn.WriteJSON(chatMessage)
	if err != nil {
		logger.Error("Failed to send system message", zap.Error(err))
	} else {
		logger.Info("System message sent", zap.String("message", message), zap.Uint("PlayerID", client.UserID))
	}
}
