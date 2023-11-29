package model

import "gorm.io/gorm"

type BetRecord struct {
	ID            uint   `gorm:"primarykey"`
	TgUserID      int64  `json:"tg_user_id" gorm:"type:bigint(20);not null"` // 用户ID
	ChatID        int64  `json:"chat_id" gorm:"type:bigint(20);not null;index"`
	IssueNumber   string `json:"issue_number" gorm:"type:varchar(64);not null"`
	BetType       string `json:"bet_type" gorm:"type:varchar(64);not null"`        // 下注类型
	BetAmount     int    `json:"bet_amount" gorm:"type:int(11);not null"`          // 下注金额
	SettleStatus  int    `json:"settle_status" gorm:"type:int(11);not null"`       // 结算状态
	BetResultType *int   `json:"bet_result_type" gorm:"type:int(11);default:null"` // 下注结果输赢
	UpdateTime    string `json:"update_time" gorm:"type:varchar(255);not null"`
	CreateTime    string `json:"create_time" gorm:"type:varchar(255);not null"`
}

// GetBetRecordsByChatIDAndIssue 根据对话ID和期号获取用户下注记录
func GetBetRecordsByChatIDAndIssue(db *gorm.DB, chatID int64, issueNumber string) ([]*BetRecord, error) {
	var betRecords []*BetRecord
	result := db.Where("chat_id = ? AND issue_number = ?", chatID, issueNumber).Find(&betRecords)
	if result.Error != nil {
		return nil, result.Error
	}
	return betRecords, nil
}

// ListBySettleStatus
func ListBySettleStatus(db *gorm.DB, betRecord *BetRecord) ([]*BetRecord, error) {
	var betRecords []*BetRecord
	result := db.Where("tg_user_id = ? AND chat_id = ? AND settle_status = ?", betRecord.TgUserID, betRecord.ChatID, 0).Find(&betRecords)
	if result.Error != nil {
		return nil, result.Error
	}
	return betRecords, nil
}

func ListByChatAndUser(db *gorm.DB, betRecord *BetRecord) ([]*BetRecord, error) {
	var betRecords []*BetRecord
	result := db.Where("tg_user_id = ? AND chat_id = ?", betRecord.TgUserID, betRecord.ChatID).Limit(10).Order("issue_number desc").Find(&betRecords)
	if result.Error != nil {
		return nil, result.Error
	}
	return betRecords, nil
}
