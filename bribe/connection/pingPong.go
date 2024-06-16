package connection

import (
	"time"

	"xicserver/bribe/broadcast"
	"xicserver/models"

	"go.uber.org/zap"

	"github.com/gorilla/websocket"
)

// MaintainWebSocketConnection はクライアントのWebSocket接続を維持し、Ping/Pongメッセージで接続をチェックします。
func MaintainWebSocketConnection(c *models.Client, clients map[*models.Client]bool, logger *zap.Logger) {
	defer func() {
		c.Conn.Close()     // ゴルーチンが終了する時にWebSocket接続を閉じる
		delete(clients, c) // クライアントリストから削除
		logger.Info("Client removed", zap.Uint("UserID", c.UserID))
		// クライアントが切断されたことを対戦相手に通知
		broadcast.NotifyOpponentOnlineStatus(c.RoomID, c.UserID, false, clients, logger)
	}()

	// Pongハンドラの設定
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second)) // 60秒の読み取りデッドラインを更新
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
			if c.Conn == nil || c.Conn.WriteMessage(websocket.PingMessage, nil) != nil {
				logger.Error("Error sending ping or connection is closed", zap.Error(c.Conn.WriteMessage(websocket.PingMessage, nil)))
				return // エラーが発生した場合はゴルーチンを終了
			}
		}
	}
}
