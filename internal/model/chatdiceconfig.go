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

func GetByEnableAndChatId(db *gorm.DB, enable int, chatID int64) (*ChatDiceConfig, error) {
	var chatDiceConfig *ChatDiceConfig
	result := db.Where("enable = ? AND chat_id = ?", enable, chatID).First(&chatDiceConfig)
	if result.Error != nil {
		return nil, result.Error
	}
	return chatDiceConfig, nil
}

func GetByChatId(db *gorm.DB, chatID int64) (*ChatDiceConfig, error) {
	var chatDiceConfig *ChatDiceConfig
	result := db.Where("chat_id = ?", chatID).First(&chatDiceConfig)
	if result.Error != nil {
		return nil, result.Error
	}
	return chatDiceConfig, nil
}
