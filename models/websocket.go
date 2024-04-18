package models

import (
	"github.com/gorilla/websocket"
)

// Websocketクライアントを定義
type Client struct {
	Conn   *websocket.Conn
	UserID uint // JWTから抽出したユーザーID
	RoomID uint
	Role   string // User role (e.g., "creator", "challenger")
}

// 各ゲームのインスタンス
type Game struct {
	ID                  uint
	Board               [][]string
	Players             [2]*Player
	PlayersOnlineStatus map[uint]bool // キー: Player ID, 値: オンライン状態
	CurrentTurn         uint          // "player1" または "player2"
	Status              string        // "waiting", "in progress", "finished", "round1", "round2" など
	BribeCounts         [2]int        // プレイヤー1とプレイヤー2の賄賂回数
	Bias                string        // "fair" または "biased"、不正の有無
	BiasDegree          int           // 不正度合い。賄賂の影響による変動値
	RefereeStatus       string        // 審判の状態（例: "normal", "biased", "sad", "angry"）
	RefereeCount        uint          // 0以上の場合はRefereeStatusが異常値に固定される
	RoomTheme           string        // ゲームモード
	Winners             []uint        // 各ラウンドの勝者のID。3要素までのスライス。引き分けの場合は、0やnil
	RetryRequests       map[uint]bool // キー: Player ID, 値: 再戦リクエストの有無
}

// PlayerはUserに紐づく
type Player struct {
	ID       uint
	Symbol   string // "X" or "O"
	NickName string
	Conn     *websocket.Conn
}
