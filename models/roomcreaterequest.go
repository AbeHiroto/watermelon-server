package models

// RoomCreateRequestはクライアントからのログインリクエストを表します。
// トークンが提供されている場合、それを使用してユーザーを認証します。
// トークンがない場合、サブスクリプションステータスに基づいて新しいトークンが生成されます。
type RoomCreateRequest struct {
	Token              string `json:"token,omitempty"`              // 既存のユーザー固有のJWTトークン
	SubscriptionStatus string `json:"subscriptionStatus,omitempty"` // 課金ステータス
	Nickname           string `json:"nickname"`                     // ニックネーム
	RoomTheme          string `json:"roomTheme"`                    // ルームのテーマ
}
