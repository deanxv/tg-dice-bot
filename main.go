package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	DBConnectionString = "MYSQL_DSN"
	TelegramAPIToken   = "TELEGRAM_API_TOKEN"
)

var (
	db        *gorm.DB
	stopFlags = make(map[int64]chan struct{})
	stopMutex sync.Mutex
)

type LotteryRecord struct {
	ID           uint   `gorm:"primarykey"`
	ChatID       int64  `json:"chat_id" gorm:"type:bigint(20);not null;index"`
	IssueNumber  int    `json:"issue_number" gorm:"type:bigint(20);not null"`
	ValueA       int    `json:"value_a" gorm:"type:int(11);not null"`
	ValueB       int    `json:"value_b" gorm:"type:int(11);not null"`
	ValueC       int    `json:"value_c" gorm:"type:int(11);not null"`
	Total        int    `json:"total" gorm:"type:int(11);not null"`
	SingleDouble string `json:"single_double" gorm:"type:varchar(255);not null"`
	BigSmall     string `json:"big_small" gorm:"type:varchar(255);not null"`
	Timestamp    string `json:"timestamp" gorm:"type:varchar(255);not null"`
}

func main() {
	initDB()
	bot := initTelegramBot()

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60
	updates := bot.GetUpdatesChan(updateConfig)

	for update := range updates {
		if update.Message != nil {
			handleMessage(bot, update.Message)
		} else if update.CallbackQuery != nil {
			go handleCallbackQuery(bot, update.CallbackQuery)
		}
	}
}

func handleCallbackQuery(bot *tgbotapi.BotAPI, callbackQuery *tgbotapi.CallbackQuery) {
	if callbackQuery.Data == "betting_history" {
		records, err := getAllRecordsByChatID(callbackQuery.Message.Chat.ID)
		var msgText string
		for _, record := range records {
			msgText = msgText + fmt.Sprintf("%d期: %d %d %d  %d  %s  %s\n",
				record.IssueNumber, record.ValueA, record.ValueB, record.ValueC, record.Total, record.SingleDouble, record.BigSmall)
		}

		msg := tgbotapi.NewMessage(callbackQuery.Message.Chat.ID, msgText)
		_, err = bot.Send(msg)
		if err != nil {
			log.Println("发送消息错误:", err)
		}
	}
}
func getAllRecordsByChatID(chatID int64) ([]LotteryRecord, error) {
	var records []LotteryRecord

	result := db.Where("chat_id = ?", chatID).Limit(10).Order("issue_number desc").Find(&records)
	if result.Error != nil {
		return nil, result.Error
	}

	return records, nil
}

func initDB() {
	var err error
	db, err = gorm.Open(mysql.Open(os.Getenv(DBConnectionString)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	err = db.AutoMigrate(&LotteryRecord{})
	if err != nil {
		log.Fatal("自动迁移表结构失败:", err)
	}
}

func initTelegramBot() *tgbotapi.BotAPI {
	bot, err := tgbotapi.NewBotAPI(os.Getenv(TelegramAPIToken))
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("已授权帐户 %s", bot.Self.UserName)
	return bot
}

func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	user := message.From
	chatID := message.Chat.ID
	messageID := message.MessageID

	chatMember, err := getChatMember(bot, chatID, int(user.ID))
	if err != nil {
		log.Println("获取聊天成员错误:", err)
		return
	}

	if message.IsCommand() {
		if message.Chat.IsGroup() {
			handleGroupCommand(bot, user.UserName, chatMember, message.Command(), &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:           chatID,
					ReplyToMessageID: messageID,
				},
			})
		} else {
			handlePrivateCommand(bot, &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:           chatID,
					ReplyToMessageID: messageID,
				},
			}, message.Command())
		}
	}
}

func getChatMember(bot *tgbotapi.BotAPI, chatID int64, userID int) (tgbotapi.ChatMember, error) {
	chatMemberConfig := tgbotapi.ChatConfigWithUser{
		ChatID: chatID,
		UserID: int64(userID),
	}

	return bot.GetChatMember(tgbotapi.GetChatMemberConfig{ChatConfigWithUser: chatMemberConfig})
}

func handleGroupCommand(bot *tgbotapi.BotAPI, username string, chatMember tgbotapi.ChatMember, command string, msgConfig *tgbotapi.MessageConfig) {
	if chatMember.IsAdministrator() || chatMember.IsCreator() {
		switch command {
		case "stop":
			handleStopCommand(bot, msgConfig)
		case "start":
			handleStartCommand(bot, msgConfig)
		}
	} else {
		log.Printf("%s 不是管理员\n", username)
		msgConfig.Text = "你不是管理员"
		sendMessage(bot, msgConfig)
	}
}

func handlePrivateCommand(bot *tgbotapi.BotAPI, msgConfig *tgbotapi.MessageConfig, command string) {
	switch command {
	case "stop":
		handleStopCommand(bot, msgConfig)
	case "start":
		handleStartCommand(bot, msgConfig)
	}
}

func handleStopCommand(bot *tgbotapi.BotAPI, msgConfig *tgbotapi.MessageConfig) {
	msgConfig.Text = "已关闭"
	sendMessage(bot, msgConfig)
	go stopDice(msgConfig.ChatID)
}

func handleStartCommand(bot *tgbotapi.BotAPI, msgConfig *tgbotapi.MessageConfig) {
	msgConfig.Text = "已开启"
	sendMessage(bot, msgConfig)
	go sendDice(bot, msgConfig.ChatID)
}

func sendMessage(bot *tgbotapi.BotAPI, msgConfig *tgbotapi.MessageConfig) {
	_, err := bot.Send(msgConfig)
	if err != nil {
		log.Println("发送消息错误:", err)
	}
}

func sendDice(bot *tgbotapi.BotAPI, chatID int64) {
	stopDice(chatID)
	stopMutex.Lock()
	defer stopMutex.Unlock()

	stopFlags[chatID] = make(chan struct{})
	go func(stopCh <-chan struct{}) {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				handleDiceRoll(bot, chatID)
			case <-stopCh:
				log.Printf("已关闭任务：%v", chatID)
				return
			}
		}
	}(stopFlags[chatID])
}

func handleDiceRoll(bot *tgbotapi.BotAPI, chatID int64) {
	currentTime := time.Now().Format("2006-01-02 15:04:05")

	diceConfig := tgbotapi.NewDiceWithEmoji(chatID, "🎲")
	diceMsgA, _ := bot.Send(diceConfig)
	diceMsgB, _ := bot.Send(diceConfig)
	diceMsgC, _ := bot.Send(diceConfig)

	count := diceMsgA.Dice.Value + diceMsgB.Dice.Value + diceMsgC.Dice.Value
	singleOrDouble, bigOrSmall := determineResult(count)

	time.Sleep(3 * time.Second)
	issueNumber, _ := strconv.Atoi(time.Now().Format("20060102150405"))
	message := formatMessage(diceMsgA.Dice.Value, diceMsgB.Dice.Value, diceMsgC.Dice.Value, count, singleOrDouble, bigOrSmall, issueNumber)

	insertLotteryRecord(chatID, issueNumber, diceMsgA.Dice.Value, diceMsgB.Dice.Value, diceMsgC.Dice.Value, count, singleOrDouble, bigOrSmall, currentTime)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("开注历史", "betting_history"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ReplyMarkup = keyboard
	sendMessage(bot, &msg)
}

func determineResult(count int) (string, string) {
	var singleOrDouble string
	var bigOrSmall string

	if count < 10 {
		bigOrSmall = "小"
	} else {
		bigOrSmall = "大"
	}

	if count%2 == 1 {
		singleOrDouble = "单"
	} else {
		singleOrDouble = "双"
	}

	return singleOrDouble, bigOrSmall
}

func formatMessage(valueA int, valueB int, valueC int, count int, singleOrDouble, bigOrSmall string, issueNumber int) string {
	return fmt.Sprintf(""+
		"点数: %d %d %d \n"+
		"总点数: %d \n"+
		"[单/双]: %s \n"+
		"[大/小]: %s \n"+
		"期号: %d ",
		valueA, valueB, valueC,
		count,
		singleOrDouble,
		bigOrSmall,
		issueNumber,
	)
}

func insertLotteryRecord(chatID int64, issueNumber, valueA, valueB, valueC, total int, singleOrDouble, bigOrSmall, currentTime string) {
	record := LotteryRecord{
		ChatID:       chatID,
		IssueNumber:  issueNumber,
		ValueA:       valueA,
		ValueB:       valueB,
		ValueC:       valueC,
		Total:        total,
		SingleDouble: singleOrDouble,
		BigSmall:     bigOrSmall,
		Timestamp:    currentTime,
	}

	result := db.Create(&record)
	if result.Error != nil {
		log.Println("插入开奖记录错误:", result.Error)
	}
}

func stopDice(chatID int64) {
	stopMutex.Lock()
	defer stopMutex.Unlock()

	if stopFlag, ok := stopFlags[chatID]; ok {
		log.Printf("停止聊天ID的任务：%v", chatID)
		close(stopFlag)
		delete(stopFlags, chatID)
	} else {
		log.Printf("没有要停止的聊天ID的任务：%v", chatID)
	}
}
