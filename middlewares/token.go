package middlewares

import (
	"time"

	"xicserver/auth"
	"xicserver/models"

	"gorm.io/gorm"

	jwt "github.com/dgrijalva/jwt-go"
	"go.uber.org/zap"
)

var db *gorm.DB // GORMデータベース接続を保持するグローバル変数
var logger *zap.Logger

func GenerateToken(db *gorm.DB, subscriptionStatus string, existingUserID uint) (string, uint, error) {
	var expirationTime time.Time
	var userID uint
	var err error

	if existingUserID > 0 {
		// 既存のユーザーIDを再利用
		userID = existingUserID
	} else {
		// 新しいユーザーIDを生成
		userID, err = GenerateUserID(db, subscriptionStatus)
		if err != nil {
			logger.Error("トークン生成中にエラー発生", zap.Error(err))
			return "", 0, err
		}
	}

	// トークンの有効期限を設定
	if subscriptionStatus == "paid" {
		expirationTime = time.Now().Add(72 * time.Hour) // 例: 72時間
	} else {
		expirationTime = time.Now().Add(72 * time.Hour) // 例: 72時間
	}

	// JWTトークン生成時に内包するデータ
	claims := &models.MyClaims{
		UserID:             userID,
		SubscriptionStatus: subscriptionStatus,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(auth.JwtKey)

	return tokenString, userID, err
}

// GORMによるオートインクリメントユーザーIDを生成する関数
func GenerateUserID(db *gorm.DB, subscriptionStatus string) (uint, error) {
	// データベースに新しいUserインスタンスを作成
	user := models.User{
		SubscriptionStatus: subscriptionStatus, // 課金ステータスを設定
	}
	if err := db.Create(&user).Error; err != nil {
		logger.Error("ユーザーID生成中にエラー発生", zap.Error(err))
		return 0, err // エラー発生時
	}
	return user.ID, nil // UserインスタンスのIDを返す
}
