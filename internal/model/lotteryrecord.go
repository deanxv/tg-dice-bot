package model

import "gorm.io/gorm"

type LotteryRecord struct {
	ID           uint   `gorm:"primarykey"`
	ChatID       int64  `json:"chat_id" gorm:"type:bigint(20);not null;index"`
	IssueNumber  string `json:"issue_number" gorm:"type:varchar(64);not null"`
	ValueA       int    `json:"value_a" gorm:"type:int(11);not null"`
	ValueB       int    `json:"value_b" gorm:"type:int(11);not null"`
	ValueC       int    `json:"value_c" gorm:"type:int(11);not null"`
	Total        int    `json:"total" gorm:"type:int(11);not null"`
	SingleDouble string `json:"single_double" gorm:"type:varchar(255);not null"`
	BigSmall     string `json:"big_small" gorm:"type:varchar(255);not null"`
	Triplet      int    `json:"triplet" gorm:"type:int(11);not null"`
	Timestamp    string `json:"timestamp" gorm:"type:varchar(255);not null"`
}

func GetAllRecordsByChatID(db *gorm.DB, chatID int64) ([]LotteryRecord, error) {
	var records []LotteryRecord

	result := db.Where("chat_id = ?", chatID).Limit(10).Order("issue_number desc").Find(&records)
	if result.Error != nil {
		return nil, result.Error
	}

	return records, nil
}
