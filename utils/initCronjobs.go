package utils

import (
	"time"
	"xicserver/models"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func CronCleaner(db *gorm.DB, logger *zap.Logger) {
	c := cron.New()

	// GameStateをexpiredに更新するジョブ（毎日特定の時間に実行）
	c.AddFunc("@daily", func() {
		logger.Info("GameStateを更新する処理を開始")
		// 24時間更新がないルームをexpiredに更新し、そのIDを取得
		expiredRoomIDs := []uint{}
		db.Model(&models.GameRoom{}).
			Where("game_state = ? AND updated_at <= ?", "created", time.Now().Add(-24*time.Hour)).
			Pluck("id", &expiredRoomIDs).
			Update("game_state", "expired")

		// 関連する入室申請のStatusをdisabledに更新
		for _, roomID := range expiredRoomIDs {
			// ルーム作成者のHasRoomをfalseに更新
			var room models.GameRoom
			db.First(&room, roomID)
			db.Model(&models.User{}).Where("id = ?", room.UserID).Update("has_room", false)

			// 申請者のHasRequestをfalseに更新
			challengers := []models.Challenger{}
			db.Where("game_room_id = ?", roomID).Find(&challengers)
			for _, challenger := range challengers {
				db.Model(&models.User{}).Where("id = ?", challenger.UserID).Update("has_request", false)
			}

			db.Model(&models.Challenger{}).
				Where("game_room_id = ?", roomID).
				Update("status", "disabled")
		}
	})

	// expired状態のルームを削除するジョブ（"分 時 日 月 曜日"）
	c.AddFunc("0 3 * * *", func() {
		logger.Info("expired状態のルームを削除する処理を開始")
		// expired状態のルームを取得
		expiredRoomIDs := []uint{}
		db.Model(&models.GameRoom{}).
			Where("game_state = ? AND updated_at <= ?", "expired", time.Now().Add(-48*time.Hour)).
			Pluck("id", &expiredRoomIDs)

		// それぞれのルームに対して入室申請を削除
		if len(expiredRoomIDs) > 0 {
			db.Where("game_room_id IN ?", expiredRoomIDs).Delete(&models.Challenger{})
		}

		// 最後にルーム自体を削除
		result := db.Where("id IN ?", expiredRoomIDs).Delete(&models.GameRoom{})
		if result.Error != nil {
			logger.Error("expired状態のルーム削除に失敗しました", zap.Error(result.Error))
		} else {
			logger.Info("expired状態のルーム削除完了", zap.Int("rooms_deleted", int(result.RowsAffected)))
		}
	})

	c.Start()
}
