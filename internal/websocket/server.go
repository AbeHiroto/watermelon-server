package websocket

import (
	"context"
	"encoding/json"

	//"fmt"
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

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// WebSocket接続のアップグレードに失敗
		logger.Error("Error upgrading WebSocket", zap.Error(err))
		http.Error(w, "Error upgrading WebSocket", http.StatusInternalServerError)
		return
	}
	client := &Client{Conn: conn, UserID: claims.UserID, RoomID: roomID, Role: role}
	// ここで新しいクライアントを処理（例：クライアントリストに追加）
	clients[client] = true
	logger.Info("New client added", zap.Uint("UserID", client.UserID), zap.Uint("RoomID", roomID), zap.String("Role", role))

	// Ping/Pongを管理するゴルーチンを起動
	go func(c *Client) {
		defer func() {
			c.Conn.Close()     // ゴルーチンが終了する時にWebSocket接続を閉じる
			delete(clients, c) // クライアントリストから削除
			logger.Info("Client removed", zap.Uint("UserID", c.UserID))
		}()

		for {
			// Pingを送信
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Error("Error sending ping", zap.Error(err))
				return // エラーが発生した場合はゴルーチンを終了
			}

			// Pingの送信間隔
			time.Sleep(10 * time.Second)
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

// handleMessage handles incoming messages from clients
func handleMessage(client *Client, messageType int, payload []byte) {
	// Process message
}
