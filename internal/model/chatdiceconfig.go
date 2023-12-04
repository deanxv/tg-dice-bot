package model

import "gorm.io/gorm"

type ChatDiceConfig struct {
	ID               int   `gorm:"primaryKey"`
	ChatID           int64 `json:"chat_id" gorm:"type:bigint(20);not null;index"`
	LotteryDrawCycle int   `json:"lottery_draw_cycle" gorm:"type:int(11);not null"` // 开奖周期(分钟)
	Enable           int   `json:"enable" gorm:"type:int(11);not null"`             // 开启状态
}

func ListByEnable(db *gorm.DB, enable int) ([]*ChatDiceConfig, error) {
	var records []*ChatDiceConfig

	result := db.Where("enable = ?", enable).Find(&records)
	if result.Error != nil {
		return nil, result.Error
	}

	return records, nil
}
