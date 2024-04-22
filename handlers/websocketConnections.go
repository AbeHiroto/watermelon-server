package handlers

import (
	"context"
	//"encoding/json"

	"net/http"
	//"strconv"
	//"strings"
	"time"

	//"xicserver/auth"
	"xicserver/bribe"
	"xicserver/bribe/actions"
	"xicserver/bribe/broadcast"

	"xicserver/bribe/connection"
	"xicserver/bribe/database"
	"xicserver/models"

	"go.uber.org/zap"
	"gorm.io/gorm"

	//"github.com/dgrijalva/jwt-go"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

// WebSocket接続へのアップグレードを行う関数
func HandleConnections(ctx context.Context, w http.ResponseWriter, r *http.Request, db *gorm.DB, rdb *redis.Client, logger *zap.Logger, clients map[*models.Client]bool, games map[uint]*models.Game, upgrader websocket.Upgrader) {
	sessionID := r.Header.Get("SessionID")
	var client *models.Client
	// リクエストヘッダーにセッションIDがある場合はセッションの復旧を行い、無ければ新規発行
	if sessionID != "" {
		client = database.ValidateSessionID(ctx, r, rdb, sessionID, logger)
		if client == nil {
			logger.Warn("Session ID is invalid or expired, creating a new session")
			client = connection.CreateNewSession(ctx, r, db, rdb, logger)
		}
	} else {
		client = connection.CreateNewSession(ctx, r, db, rdb, logger)
	}

	if client == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// WebSocket接続へのアップグレードと確立
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// WebSocket接続のアップグレードに失敗
		logger.Error("Error upgrading WebSocket", zap.Error(err))
		http.Error(w, "Error upgrading WebSocket", http.StatusInternalServerError)
		return
	}

	// クライアントリストに新規クライアントを追加
	clients[client] = true
	logger.Info("New client added", zap.Uint("UserID", client.UserID), zap.Uint("RoomID", client.RoomID), zap.String("Role", client.Role))

	// WebSocketのCloseHandlerを設定
	client.Conn.SetCloseHandler(func(code int, text string) error {
		// Closeイベントが発生した時の処理
		logger.Info("WebSocket closed", zap.Int("code", code), zap.String("reason", text))
		client.Conn.Close()     // 念のため、接続を閉じる
		delete(clients, client) // クライアントリストから削除
		return nil
	})

	// ゲームインスタンスの検索または作成
	var game *models.Game
	var symbol string
	// 乱数生成器のインスタンスを生成
	randGen := bribe.CreateLocalRandGenerator()
	if existingGame, ok := games[client.RoomID]; ok {
		// ゲームインスタンスが既に存在する場合、参加
		game = existingGame
		// クライアントがゲームにすでに参加しているか確認
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
			// 既にゲームに参加しているクライアントの再接続処理
			game.Players[playerIndex].Conn = conn          // 新しいWebSocket接続を設定
			game.PlayersOnlineStatus[client.UserID] = true // オンライン状態をtrueに更新
		} else {
			// 挑戦者のニックネームを取得
			var challenger models.Challenger
			db.Where("game_room_id = ? AND user_id = ?", client.RoomID, client.UserID).First(&challenger)
			nickName := challenger.ChallengerNickname // ニックネームを取得
			// 2人目のプレイヤーとして参加
			symbol = "O" // 2人目のプレイヤーには "O" を割り当て
			game.Players[1] = &models.Player{ID: client.UserID, Conn: conn, Symbol: symbol, NickName: nickName}
			game.PlayersOnlineStatus[1] = true // 2人目のプレイヤーをオンラインとしてマーク
			// 2人目のプレイヤーが参加したので、ランダムに先手を決定
			if randGen.Intn(2) == 0 {
				game.CurrentTurn = game.Players[0].ID
			} else {
				game.CurrentTurn = game.Players[1].ID
			}
		}

		// ゲームの状態をブロードキャスト
		broadcast.BroadcastGameState(game, logger)

	} else {
		var boardSize int
		var roomTheme string
		var gameRoom models.GameRoom
		var bias string // "fair"か"biased"
		var biasDegree int
		var refereeStatus string
		// ここでDBからGameRoomのインスタンスを取得し、RoomThemeフィールドの値を取得する
		err := db.Where("id = ?", client.RoomID).First(&gameRoom).Error
		if err != nil {
			// データベースからゲームルームの情報を取得できなかった場合
			logger.Error("Failed to retrieve game room from database", zap.Error(err))
			http.Error(w, "Failed to retrieve game room information", http.StatusInternalServerError)
			return
		}
		roomTheme = gameRoom.RoomTheme
		nickName := gameRoom.RoomCreator // ニックネームを取得

		// RoomThemeに基づいて盤面のサイズと不正度合いを設定
		switch roomTheme {
		case "3x3_biased":
			boardSize = 3            //この場合３ｘ３マス
			bias = "biased"          //審判の不正有りに設定
			biasDegree = 0           // 初期不正度合い
			refereeStatus = "normal" //ゲーム開始時は公平
		case "3x3_fair":
			boardSize = 3
			bias = "fair"
			biasDegree = 0
			refereeStatus = "normal"
		case "5x5_biased":
			boardSize = 5
			bias = "biased"
			biasDegree = 0
			refereeStatus = "normal"
		case "5x5_fair":
			boardSize = 5
			bias = "fair"
			biasDegree = 0
			refereeStatus = "normal"
		default:
			boardSize = 3
			bias = "biased"
			biasDegree = 0
			refereeStatus = "normal"
		}

		// ボードの初期化
		board := make([][]string, boardSize)
		for i := range board {
			board[i] = make([]string, boardSize)
		}

		symbol = "X" // 最初のプレイヤーには "X" を割り当て
		game = &models.Game{
			ID:            client.RoomID,
			Board:         board,
			Players:       [2]*models.Player{{ID: client.UserID, Conn: conn, Symbol: symbol, NickName: nickName}, nil},
			Status:        "round1",
			RoomTheme:     roomTheme,
			Bias:          bias,
			BiasDegree:    biasDegree,
			RefereeStatus: refereeStatus,
		}
		games[client.RoomID] = game // ゲームをマップに追加
		game.Players[0] = &models.Player{ID: client.UserID, Conn: conn, Symbol: "X", NickName: nickName}
		game.PlayersOnlineStatus[0] = true // 作成者をオンラインとしてマーク

		// ゲームの状態をブロードキャスト
		broadcast.BroadcastGameState(game, logger)
	}

	// クライアントごとにメッセージ読み取りゴルーチンを起動（）
	go actions.HandleClient(client, clients, games, randGen, db, logger)

	// Ping/Pongを管理するゴルーチンを起動
	go func(c *models.Client) {
		defer func() {
			c.Conn.Close()     // ゴルーチンが終了する時にWebSocket接続を閉じる
			delete(clients, c) // クライアントリストから削除
			logger.Info("Client removed", zap.Uint("UserID", c.UserID))
			// クライアントが切断されたことを対戦相手に通知
			broadcast.NotifyOpponentOnlineStatus(c.RoomID, c.UserID, false, clients, logger)
		}()

		// Pongハンドラの設定: Pongメッセージを受信したら読み取りデッドラインを更新し、オンライン状態を反映
		c.Conn.SetPongHandler(func(string) error {
			c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second)) // 60秒の読み取りデッドライン
			// クライアントがオンラインであることを対戦相手に通知
			broadcast.NotifyOpponentOnlineStatus(c.RoomID, c.UserID, true, clients, logger)
			return nil
		})

		// Pingの送信間隔を設定
		pingPeriod := 10 * time.Second // 10秒ごとにPingを送信

		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Pingを送信
				if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					logger.Error("Error sending ping", zap.Error(err))
					return // エラーが発生した場合はゴルーチンを終了
				}
				// 必要に応じて、他のcase分岐を追加
			}

			// 読み取りデッドラインの初期設定（最初のPong待機に使用）
			c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second)) // 60秒の読み取りデッドライン
		}
	}(client)

	// Generate and store session ID, then send it back to the client
	err = database.GenerateAndStoreSessionID(r.Context(), client, rdb, logger)
	if err != nil {
		logger.Error("Failed to generate or store session ID", zap.Error(err))
	}
}
