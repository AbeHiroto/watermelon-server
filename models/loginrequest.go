package models

// LoginRequest はクライアントからのログインリクエストを表します。
// トークンが提供されている場合、それを使用してユーザーを認証します。
// トークンがない場合、サブスクリプションステータスに基づいて新しいトークンが生成されます。
type LoginRequest struct {
	Token              string `json:"token,omitempty"`              // 既存のトークン
	SubscriptionStatus string `json:"subscriptionStatus,omitempty"` // 課金ステータス
}
