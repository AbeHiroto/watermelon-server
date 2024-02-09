package models

import (
	"gorm.io/gorm"
)

type Challenger struct {
	gorm.Model         // gormのデフォルト属性（ID, CreatedAt, UpdatedAt, DeletedAt）を追加
	UserID             uint
	GameRoomID         uint   // GameRoomテーブルのIDを参照
	ChallengerNickname string // 挑戦希望者のニックネーム
	Status             string // 申請状態を表す（例：pending, accepted, rejected）
}
