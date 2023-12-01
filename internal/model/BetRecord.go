package model

import "gorm.io/gorm"

type BetRecord struct {
	ID          uint   `gorm:"primarykey"`
	UserID      int64  // 用户ID
	ChatID      int64  // 对话ID，用于隔离对话
	IssueNumber string `json:"issue_number" gorm:"type:varchar(64);not null"`
	BetType     string // 下注类型
	BetAmount   int    // 下注金额
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
