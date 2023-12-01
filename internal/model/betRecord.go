package model

import "gorm.io/gorm"

type BetRecord struct {
	ID          uint   `gorm:"primarykey"`
	UserID      int64  `json:"user_id" gorm:"type:int(11);not null"` // 用户ID
	ChatID      int64  `json:"chat_id" gorm:"type:bigint(20);not null;index"`
	IssueNumber string `json:"issue_number" gorm:"type:varchar(64);not null"`
	BetType     string `json:"bet_type" gorm:"type:varchar(64);not null"` // 下注类型
	BetAmount   int    `json:"bet_amount" gorm:"type:int(11);not null"`   // 下注金额
	Timestamp   string `json:"timestamp" gorm:"type:varchar(255);not null"`
}

// GetBetRecordsByChatIDAndIssue 根据对话ID和期号获取用户下注记录
func GetBetRecordsByChatIDAndIssue(db *gorm.DB, chatID int64, issueNumber string) ([]BetRecord, error) {
	var betRecords []BetRecord
	result := db.Where("chat_id = ? AND issue_number = ?", chatID, issueNumber).Find(&betRecords)
	if result.Error != nil {
		return nil, result.Error
	}
	return betRecords, nil
}
