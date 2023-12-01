package model

type TgUser struct {
	ID         int    `gorm:"primaryKey"`
	UserID     int64  `json:"user_id" gorm:"type:int(11);not null"` // Telegram 用户ID
	ChatID     int64  `json:"chat_id" gorm:"type:int(11);not null"`
	Username   string `json:"username" gorm:"type:varchar(500);not null"` // Telegram 用户名
	Balance    int    `json:"balance" gorm:"type:int(11);not null"`
	SignInTime string `json:"sign_in_time" gorm:"type:varchar(500)"` // 签到时间
}
