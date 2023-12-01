package model

type ChatDiceConfig struct {
	ID               int `gorm:"primaryKey"`
	ChatID           int64
	LotteryDrawCycle int // 开奖周期(分钟)
	Enable           int // 开启状态
}
