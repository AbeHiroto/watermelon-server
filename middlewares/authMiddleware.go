package middlewares

import (
	"net/http"
	// 必要なパッケージをインポート
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	// データベース操作用のパッケージ
)

// トークン検証とユーザーID検証を行うミドルウェア
func AuthMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		userID := c.GetHeader("UserID")

		if isValidToken(token) && isValidUserID(userID) {
			logger.Info("認証成功", zap.String("token", token), zap.String("userID", userID))
			c.Next()
		} else {
			logger.Warn("認証失敗", zap.String("token", token), zap.String("userID", userID))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		}
	}
}

// トークンが有効かどうかをチェックする関数
func isValidToken(token string) bool {
	// データベースや認証サービスでトークンを照合
	// JWTなどの標準的なトークンフォーマットを使用する場合は、ここでデコードと検証を行う
	return true // 仮の実装
}

// ユーザーIDが有効かどうかをチェックする関数
func isValidUserID(userID string) bool {
	// データベースでユーザーIDを照合
	return true // 仮の実装
}

// package middlewares

// import (
// 	"net/http"

// 	"github.com/gin-gonic/gin"
// 	"go.uber.org/zap"
// )

// // AuthMiddleware は認証を行うミドルウェア関数です。
// func AuthMiddleware(logger *zap.Logger) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		// ここでセッショントークンとユーザーIDをリクエストから取得
// 		token := c.GetHeader("Authorization")
// 		userID := c.GetHeader("UserID")

// 		// トークンとユーザーIDの検証
// 		if isValidToken(token) && isValidUserID(userID) {
// 			c.Next() // 認証成功
// 		} else {
// 			logger.Info("認証失敗", zap.String("token", token), zap.String("userID", userID))
// 			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
// 		}
// 	}
// }

// // isValidToken はトークンが有効かどうかをチェックする関数です。
// func isValidToken(token string) bool {
// 	// トークン検証ロジックの実装
// 	// 例: データベースや認証サービスと照合
// 	return token == "valid" // 仮の実装
// }

// // isValidUserID はユーザーIDが有効かどうかをチェックする関数です。
// func isValidUserID(userID string) bool {
// 	// ユーザーID検証ロジックの実装
// 	// 例: データベースとの照合
// 	return userID != "" // 仮の実装
// }
