package model

type ChatDiceConfig struct {
	ID               int   `gorm:"primaryKey"`
	ChatID           int64 `json:"chat_id" gorm:"type:int(11);not null"`
	LotteryDrawCycle int   `json:"lottery_draw_cycle" gorm:"type:int(11);not null"` // 开奖周期(分钟)
	Enable           int   `json:"enable" gorm:"type:int(11);not null"`             // 开启状态
}
