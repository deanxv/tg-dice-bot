package model

type TgUser struct {
	ID         int   `gorm:"primaryKey"`
	UserID     int64 // Telegram 用户ID
	ChatID     int64
	Username   string // Telegram 用户名
	Balance    int
	SignInTime string // 签到时间
}
