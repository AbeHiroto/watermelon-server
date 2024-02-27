package websocket

import (
	"context"
	"encoding/json"

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
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var logger *zap.Logger // Global logger
var ctx = context.Background()
var rdb *redis.Client

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// ここでオリジンの検証を行うか、全てのオリジンを許可する
		return true
	},
}

func InitializeRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // or your Redis server address
		Password: "",               // no password set
		DB:       0,                // use default DB
	})
}

// Client represents a WebSocket client
type Client struct {
	Conn   *websocket.Conn
	UserID uint // JWTから抽出したユーザーID
	RoomID uint
	Role   string // User role (e.g., "creator", "challenger")
}

// clients keeps track of all active clients.
var clients = make(map[*Client]bool)

// handleConnections handles incoming WebSocket connections
func handleConnections(w http.ResponseWriter, r *http.Request, db *gorm.DB, rdb *redis.Client) {
	// JWTトークンをリクエストヘッダーから取得
	tokenString := r.Header.Get("Authorization")
	userID := r.Header.Get("UserID") // 仮のヘッダー名、実際には適切なものを設定
	role := r.Header.Get("Role")     // "Creator" または "Challenger"
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
				generateAndStoreSessionID(client, rdb)
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
		if err := db.Where("id = ? AND user_id = ?", roomID, userID).First(&gameRoom).Error; err == nil {
			accessGranted = true
		}
	} else if role == "Challenger" {
		var challenger models.Challenger
		if err := db.Where("game_room_id = ? AND user_id = ? AND status = 'accepted'", roomID, userID).First(&challenger).Error; err == nil {
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

	// クライアントごとにメッセージ読み取りゴルーチンを起動（）
	go handleClient(client, clients)

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
	generateAndStoreSessionID(client, rdb)
	if err != nil {
		logger.Error("Failed to generate or store session ID", zap.Error(err))
		// 適切なエラーハンドリング
	}
}

func generateAndStoreSessionID(client *Client, rdb *redis.Client) error {
	sessionID := uuid.New().String()

	// セッション情報をJSON形式でエンコード
	sessionInfo := map[string]uint{"userID": client.UserID, "roomID": client.RoomID}
	sessionInfoJSON, err := json.Marshal(sessionInfo)
	if err != nil {
		logger.Error("Error encoding session info", zap.Error(err))
		return err
	}

	// セッションIDとセッション情報をRedisに保存
	err = rdb.Set(ctx, "session:"+sessionID, sessionInfoJSON, 24*time.Hour).Err() // 24時間の有効期限
	if err != nil {
		logger.Error("Error storing session info in Redis", zap.Error(err))
		return err
	}

	// セッションIDをクライアントに送り返す
	return sendSessionIDToClient(client, sessionID)
}

func sendSessionIDToClient(client *Client, sessionID string) error {
	// セッションIDをクライアントに送信するためのレスポンスを作成
	response := map[string]string{"sessionID": sessionID}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		logger.Error("Error marshalling session ID response", zap.Error(err))
		return err
	}

	// クライアントにセッションIDを含むレスポンスを送信
	if err := client.Conn.WriteMessage(websocket.TextMessage, responseJSON); err != nil {
		logger.Error("Error sending session ID to client", zap.Error(err))
		return err
	}

	return nil
}

// Helper function to send error message to the client via WebSocket
func sendErrorMessage(client *Client, errorMessage string) {
	errorResponse := map[string]string{"error": errorMessage}
	errorJSON, _ := json.Marshal(errorResponse)
	client.Conn.WriteMessage(websocket.TextMessage, errorJSON) // Ignoring error for simplicity
}

func handleClient(client *Client, clients map[*Client]bool) {
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
			break // エラーが発生したらループを抜ける
		}

		// 受信したメッセージに対する処理
		logger.Info("Received message", zap.ByteString("message", message))
	}
}

// handleMessage handles incoming messages from clients
func handleMessage(client *Client, messageType int, payload []byte) {
	// Process message
}
