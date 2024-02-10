package models

// RoomCreateRequestはクライアントからのログインリクエストを表します。
// トークンが提供されている場合、それを使用してユーザーを認証します。
// トークンがない場合、サブスクリプションステータスに基づいて新しいトークンが生成されます。
type myRoomInfoRequest struct {
	Token string `json:"token,omitempty"` // 既存のユーザー固有のJWTトークン
}
