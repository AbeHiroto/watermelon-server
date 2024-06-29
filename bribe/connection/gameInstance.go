package connection

import (
	"context"
	"math/rand"

	"xicserver/bribe"
	"xicserver/bribe/broadcast"
	"xicserver/models"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/gorilla/websocket"
)

func ManageGameInstance(ctx context.Context, db *gorm.DB, logger *zap.Logger, games map[uint]*models.Game, client *models.Client, conn *websocket.Conn) (*models.Game, error) {
	randGen := bribe.CreateLocalRandGenerator() // 乱数生成器のインスタンスを生成
	if existingGame, ok := games[client.RoomID]; ok {
		// ゲームインスタンスが既に存在する場合、参加
		game := existingGame
		alreadyJoined := false
		playerIndex := -1
		for i, player := range game.Players {
			if player != nil && player.ID == client.UserID {
				alreadyJoined = true
				playerIndex = i
				break
			}
		}
		if alreadyJoined {
			game.Players[playerIndex].Conn = conn          // 新しいWebSocket接続を設定
			game.PlayersOnlineStatus[client.UserID] = true // オンライン状態をtrueに更新
			logger.Info("Player rejoined the game", zap.Uint("UserID", client.UserID), zap.Uint("RoomID", client.RoomID))
		} else {
			var challenger models.Challenger
			db.Where("game_room_id = ? AND user_id = ?", client.RoomID, client.UserID).First(&challenger)
			nickName := challenger.ChallengerNickname // ニックネームを取得
			symbol := "O"                             // 2人目のプレイヤーには "O" を割り当て
			game.Players[1] = &models.Player{ID: client.UserID, Conn: conn, Symbol: symbol, NickName: nickName}
			game.PlayersOnlineStatus[1] = true // 2人目のプレイヤーをオンラインとしてマーク
			logger.Info("Second player joined the game", zap.Uint("UserID", client.UserID), zap.Uint("RoomID", client.RoomID))

			// 2人目のプレイヤーが参加したので、ランダムに先手を決定
			if randGen.Intn(2) == 0 {
				game.CurrentTurn = game.Players[0].ID
			} else {
				game.CurrentTurn = game.Players[1].ID
			}
			logger.Info("Turn decided", zap.Uint("CurrentTurn", game.CurrentTurn))
		}
		broadcast.BroadcastGameState(game, logger)
		logger.Info("Game state broadcasted", zap.Uint("RoomID", client.RoomID))
		return game, nil
	} else {
		var boardSize int
		var roomTheme string
		var gameRoom models.GameRoom
		var bias string // "fair"か"biased"

		err := db.Where("id = ?", client.RoomID).First(&gameRoom).Error
		if err != nil {
			logger.Error("Failed to retrieve game room from database", zap.Error(err))
			return nil, err
		}
		roomTheme = gameRoom.RoomTheme
		nickName := gameRoom.RoomCreator // ニックネームを取得
		// RoomThemeに基づいて盤面のサイズと不正度合いを設定
		switch roomTheme {
		case "3x3_biased":
			boardSize = 3
			bias = "biased"
		case "5x5_biased":
			boardSize = 5
			bias = "biased"
		default:
			boardSize = 3
			bias = "fair"
		}

		board := make([][]string, boardSize)
		for i := range board {
			board[i] = make([]string, boardSize)
		}

		symbol := "X"
		game := &models.Game{
			ID:                  client.RoomID,
			Board:               board,
			Players:             [2]*models.Player{{ID: client.UserID, Conn: conn, Symbol: symbol, NickName: nickName}, nil},
			Status:              "round1",
			RoomTheme:           roomTheme,
			Bias:                bias,
			BiasDegree:          0,
			RefereeStatus:       getRandomNormalRefereeStatus(randGen),
			PlayersOnlineStatus: make(map[uint]bool), // マップを初期化
			BribeCounts:         [2]int{0, 0},
		}
		games[client.RoomID] = game
		game.Players[0] = &models.Player{ID: client.UserID, Conn: conn, Symbol: "X", NickName: nickName}
		game.PlayersOnlineStatus[client.UserID] = true // 初期プレイヤーをオンラインとしてマーク
		logger.Info("New game instance created", zap.Uint("RoomID", client.RoomID), zap.Uint("UserID", client.UserID))

		broadcast.BroadcastGameState(game, logger)
		logger.Info("Game state broadcasted", zap.Uint("RoomID", client.RoomID))

		return game, nil
	}
}

func getRandomNormalRefereeStatus(randGen *rand.Rand) string {
	normalStatuses := []string{"normal_01", "normal_02", "normal_03", "normal_04", "normal_05", "normal_06", "normal_07"}
	return normalStatuses[randGen.Intn(len(normalStatuses))]
}
