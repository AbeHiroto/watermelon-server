package websocket

import (
	"context"
	"encoding/json"
	"math/rand"

	"net/http"
	"strconv"
	"strings"
	"time"
	"xicserver/auth"
	"xicserver/models"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

// clients keeps track of all active clients.
var clients = make(map[*Client]bool)
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		//// 信頼できるオリジン、つまり自分のドメイン名を指定
		// allowedOrigin := "https://yourapp.com"
		// return r.Header.Get("Origin") == allowedOrigin
		return true
	},
}
var games = make(map[uint]*Game) // ゲームIDをキーとするゲームインスタンスのマップ

// Client represents a WebSocket client
type Client struct {
	Conn   *websocket.Conn
	UserID uint // JWTから抽出したユーザーID
	RoomID uint
	Role   string // User role (e.g., "creator", "challenger")
}
type Game struct {
	ID          uint
	Players     [2]*Player
	CurrentTurn string // "player1" または "player2"
	Status      string // "waiting", "in progress", "finished" など
	BribeCounts [2]int // プレイヤー1とプレイヤー2の賄賂回数
	Bias        int    // 賄賂の度合い。正の値はPlayer1に、負の値はPlayer2に有利
	RoomTheme   string // ゲームモード
}

type Player struct {
	ID     uint
	Symbol string // "X" or "O"
	Conn   *websocket.Conn
}

func createLocalRandGenerator() *rand.Rand {
	source := rand.NewSource(time.Now().UnixNano())
	return rand.New(source)
}

// handleConnections handles incoming WebSocket connections
func HandleConnections(ctx context.Context, w http.ResponseWriter, r *http.Request, db *gorm.DB, rdb *redis.Client, logger *zap.Logger) {
	// JWTトークンをリクエストヘッダーから取得
	tokenString := r.Header.Get("Authorization")
	//userID := r.Header.Get("UserID") // 仮のヘッダー名、実際には適切なものを設定
	role := r.Header.Get("Role") // "Creator" または "Challenger"
	// roomIDを文字列からuintに変換
	roomIDStr := r.Header.Get("RoomID")                     // HTTPヘッダーから文字列としてroomIDを取得
	roomIDUint, err := strconv.ParseUint(roomIDStr, 10, 32) // 文字列をuintに変換
	if err != nil {
		// 変換エラーの処理
		logger.Error("Invalid roomID format", zap.Error(err))
		http.Error(w, "Invalid roomID format", http.StatusBadRequest)
		return
	}
	roomID := uint(roomIDUint) // uint型にキャスト

	// Bearerトークンのプレフィックスを確認し、存在する場合は削除
	if strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	}

	// JWTトークンを検証し、クレームを抽出
	claims := &models.MyClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return auth.JwtKey, nil // ！！！ここで使用するシークレットキーは、本番環境では環境変数で設定
	})

	if err != nil || !token.Valid {
		// トークンの検証失敗時の処理
		logger.Error("Failed to validate token", zap.Error(err))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// WebSocket接続のアップグレードに失敗
		logger.Error("Error upgrading WebSocket", zap.Error(err))
		http.Error(w, "Error upgrading WebSocket", http.StatusInternalServerError)
		return
	}

	client := &Client{Conn: conn, UserID: claims.UserID, RoomID: roomID, Role: role}
	// セッションIDの検証と復元
	sessionID := r.Header.Get("SessionID") // クライアントが送るセッションID
	if sessionID != "" {
		sessionInfoJSON, err := rdb.Get(ctx, "session:"+sessionID).Result()
		if err == nil {
			// Redisから取得したセッション情報をデコード
			var sessionInfo map[string]uint
			if err := json.Unmarshal([]byte(sessionInfoJSON), &sessionInfo); err == nil {
				// セッション情報に基づいてクライアント情報を復元
				client.UserID = sessionInfo["userID"]
				client.RoomID = sessionInfo["roomID"]
				// 旧セッションの削除
				rdb.Del(ctx, "session:"+sessionID)
				// 新しいセッションIDの発行と保存
				generateAndStoreSessionID(r.Context(), client, rdb, logger)
			} else {
				// セッション情報の復元に失敗した場合の処理
				logger.Error("Failed to decode session info", zap.Error(err))
				http.Error(w, "Failed to restore session", http.StatusInternalServerError)
				return
			}
		} else {
			// セッションIDが無効または期限切れの場合
			http.Error(w, "Invalid or expired session ID", http.StatusUnauthorized)
			return
		}
	}

	// WebSocketのCloseHandlerを設定
	client.Conn.SetCloseHandler(func(code int, text string) error {
		// Closeイベントが発生した時の処理
		logger.Info("WebSocket closed", zap.Int("code", code), zap.String("reason", text))
		client.Conn.Close()     // 念のため、接続を閉じる
		delete(clients, client) // クライアントリストから削除
		return nil
	})

	// ロールに基づいてデータベースを照会
	var accessGranted bool
	if role == "Creator" {
		var gameRoom models.GameRoom
		if err := db.Where("id = ? AND user_id = ?", roomID, claims.UserID).First(&gameRoom).Error; err == nil {
			accessGranted = true
		}
	} else if role == "Challenger" {
		var challenger models.Challenger
		if err := db.Where("game_room_id = ? AND user_id = ? AND status = 'accepted'", roomID, claims.UserID).First(&challenger).Error; err == nil {
			accessGranted = true
		}
	}

	if !accessGranted {
		http.Error(w, "Not authorized", http.StatusForbidden)
		return
	}

	// ここで新しいクライアントを処理（例：クライアントリストに追加）
	clients[client] = true
	logger.Info("New client added", zap.Uint("UserID", client.UserID), zap.Uint("RoomID", roomID), zap.String("Role", role))

	// ゲームインスタンスの検索または作成
	var game *Game
	var symbol string
	// 乱数生成器のインスタンスを生成
	randGen := createLocalRandGenerator()
	if existingGame, ok := games[roomID]; ok {
		// ゲームインスタンスが既に存在する場合、参加
		game = existingGame
		// 2人目のプレイヤーとして参加
		symbol = "O" // 2人目のプレイヤーには "O" を割り当て
		game.Players[1] = &Player{ID: claims.UserID, Conn: conn, Symbol: symbol}
		game.Status = "in progress" // ゲームのステータスを更新
		// 2人目のプレイヤーが参加したので、ランダムに先手を決定
		if randGen.Intn(2) == 0 {
			game.CurrentTurn = "player1"
		} else {
			game.CurrentTurn = "player2"
		}
	} else {
		var roomTheme string
		var gameRoom models.GameRoom
		// ここでDBからGameRoomのインスタンスを取得し、RoomThemeフィールドの値を取得する
		err := db.Where("id = ?", roomID).First(&gameRoom).Error
		if err != nil {
			// データベースからゲームルームの情報を取得できなかった場合
			logger.Error("Failed to retrieve game room from database", zap.Error(err))
			http.Error(w, "Failed to retrieve game room information", http.StatusInternalServerError)
			return
		}
		roomTheme = gameRoom.RoomTheme
		// 新しいゲームインスタンスを作成
		symbol = "X" // 最初のプレイヤーには "X" を割り当て
		game = &Game{
			ID:        roomID,
			Players:   [2]*Player{{ID: claims.UserID, Conn: conn, Symbol: symbol}, nil},
			Status:    "waiting",
			RoomTheme: roomTheme,
		}
		games[roomID] = game // ゲームをマップに追加
	}

	// クライアントごとにメッセージ読み取りゴルーチンを起動（）
	go handleClient(client, clients, logger)

	// Ping/Pongを管理するゴルーチンを起動
	go func(c *Client) {
		defer func() {
			c.Conn.Close()     // ゴルーチンが終了する時にWebSocket接続を閉じる
			delete(clients, c) // クライアントリストから削除
			logger.Info("Client removed", zap.Uint("UserID", c.UserID))
		}()

		// Pongハンドラの設定: Pongメッセージを受信したら読み取りデッドラインを更新
		c.Conn.SetPongHandler(func(string) error {
			c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second)) // ここでの60秒は例示的な値
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
			c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second)) // ここでの60秒は例示的な値
		}
	}(client)

	// Generate and store session ID, then send it back to the client
	err = generateAndStoreSessionID(r.Context(), client, rdb, logger)
	if err != nil {
		logger.Error("Failed to generate or store session ID", zap.Error(err))
		// 適切なエラーハンドリング
	}
}

// func generateGameID() string {
// 	return uuid.New().String() // UUIDを生成して返す
// }
