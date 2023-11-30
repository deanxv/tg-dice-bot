package bot

import (
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"tg-dice-bot/internal/model"
)

var (
	stopFlags = make(map[int64]chan struct{})
	stopMutex sync.Mutex
)

// handleCallbackQuery 处理回调查询。
func handleCallbackQuery(bot *tgbotapi.BotAPI, callbackQuery *tgbotapi.CallbackQuery) {
	if callbackQuery.Data == "betting_history" {
		handleBettingHistoryQuery(bot, callbackQuery)
	}
}

// handleBettingHistoryQuery 处理 "betting_history" 回调查询。
func handleBettingHistoryQuery(bot *tgbotapi.BotAPI, callbackQuery *tgbotapi.CallbackQuery) {
	records, err := model.GetAllRecordsByChatID(db, callbackQuery.Message.Chat.ID)
	if err != nil {
		log.Println("获取开奖历史错误:", err)
		return
	}

	msgText := generateBettingHistoryMessage(records)
	msg := tgbotapi.NewMessage(callbackQuery.Message.Chat.ID, msgText)

	sentMsg, err := bot.Send(msg)
	if err != nil {
		log.Println("发送消息错误:", err)
	}

	go func(messageID int) {
		time.Sleep(1 * time.Minute)
		deleteMsg := tgbotapi.NewDeleteMessage(callbackQuery.Message.Chat.ID, messageID)
		_, err := bot.Request(deleteMsg)
		if err != nil {
			log.Println("删除消息错误:", err)
		}
	}(sentMsg.MessageID)
}

// generateBettingHistoryMessage 生成开奖历史消息文本。
func generateBettingHistoryMessage(records []model.LotteryRecord) string {
	var msgText string

	for _, record := range records {
		triplet := ""
		if record.Triplet == 1 {
			triplet = "【豹子】"
		}
		msgText += fmt.Sprintf("%s期: %d %d %d  %d  %s  %s %s\n",
			record.IssueNumber, record.ValueA, record.ValueB, record.ValueC, record.Total, record.SingleDouble, record.BigSmall, triplet)
	}
	return msgText
}

// handleMessage 处理传入的消息。
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
			handleGroupCommand(bot, user.UserName, chatMember, message.Command(), chatID, messageID)
		} else {
			handlePrivateCommand(bot, chatID, messageID, message.Command())
		}
	}
}

// handleGroupCommand 处理群聊中的命令。
func handleGroupCommand(bot *tgbotapi.BotAPI, username string, chatMember tgbotapi.ChatMember, command string, chatID int64, messageID int) {
	if chatMember.IsAdministrator() || chatMember.IsCreator() {
		switch command {
		case "stop":
			handleStopCommand(bot, chatID, messageID)
		case "start":
			handleStartCommand(bot, chatID, messageID)
		case "help":
			handleHelpCommand(bot, chatID, messageID)
		}
	} else {
		log.Printf("%s 不是管理员\n", username)
		msgConfig := tgbotapi.NewMessage(chatID, "你不是管理员")
		msgConfig.ReplyToMessageID = messageID
		sendMessage(bot, &msgConfig)
	}
}

// handlePrivateCommand 处理私聊中的命令。
func handlePrivateCommand(bot *tgbotapi.BotAPI, chatID int64, messageID int, command string) {
	switch command {
	case "stop":
		handleStopCommand(bot, chatID, messageID)
	case "start":
		handleStartCommand(bot, chatID, messageID)
	case "help":
		handleHelpCommand(bot, chatID, messageID)
	}
}

// handleStopCommand 处理 "stop" 命令。
func handleStopCommand(bot *tgbotapi.BotAPI, chatID int64, messageID int) {
	msgConfig := tgbotapi.NewMessage(chatID, "已关闭")
	msgConfig.ReplyToMessageID = messageID
	sendMessage(bot, &msgConfig)
	stopDice(chatID)
}

// handleStartCommand 处理 "start" 命令。
func handleStartCommand(bot *tgbotapi.BotAPI, chatID int64, messageID int) {
	msgConfig := tgbotapi.NewMessage(chatID, "已开启")
	msgConfig.ReplyToMessageID = messageID
	sendMessage(bot, &msgConfig)

	issueNumber := time.Now().Format("20060102150405")
	lotteryDrawTipMsgConfig := tgbotapi.NewMessage(chatID, fmt.Sprintf("第%s期 1分钟后开奖", issueNumber))
	sendMessage(bot, &lotteryDrawTipMsgConfig)

	go startDice(bot, chatID, issueNumber)
}

// handleHelpCommand 处理 "help" 命令。
func handleHelpCommand(bot *tgbotapi.BotAPI, chatID int64, messageID int) {
	msgConfig := tgbotapi.NewMessage(chatID, "/start 开启机器人\n/stop 关闭机器人\n开奖周期: 1分钟")
	msgConfig.ReplyToMessageID = messageID
	sentMsg, err := sendMessage(bot, &msgConfig)
	if err != nil {
		return
	}
	go func(messageID int) {
		time.Sleep(1 * time.Minute)
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
		_, err := bot.Request(deleteMsg)
		if err != nil {
			log.Println("删除消息错误:", err)
		}
	}(sentMsg.MessageID)
}

// sendMessage 使用提供的消息配置发送消息。
func sendMessage(bot *tgbotapi.BotAPI, msgConfig *tgbotapi.MessageConfig) (tgbotapi.Message, error) {
	sentMsg, err := bot.Send(msgConfig)
	if err != nil {
		log.Println("发送消息错误:", err)
		return sentMsg, err
	}
	return sentMsg, nil
}

// getChatMember 获取有关聊天成员的信息。
func getChatMember(bot *tgbotapi.BotAPI, chatID int64, userID int) (tgbotapi.ChatMember, error) {
	chatMemberConfig := tgbotapi.ChatConfigWithUser{
		ChatID: chatID,
		UserID: int64(userID),
	}

	return bot.GetChatMember(tgbotapi.GetChatMemberConfig{ChatConfigWithUser: chatMemberConfig})
}

// stopDice 停止特定聊天ID的骰子滚动。
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

// startDice 启动特定聊天ID的骰子滚动。
func startDice(bot *tgbotapi.BotAPI, chatID int64, issueNumber string) {
	stopDice(chatID)
	stopMutex.Lock()
	defer stopMutex.Unlock()

	stopFlags[chatID] = make(chan struct{})
	go func(stopCh <-chan struct{}) {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				nextIssueNumber := handleDiceRoll(bot, chatID, issueNumber)
				issueNumber = nextIssueNumber
			case <-stopCh:
				log.Printf("已关闭任务：%v", chatID)
				return
			}
		}
	}(stopFlags[chatID])
}

// handleDiceRoll 处理骰子滚动过程。
func handleDiceRoll(bot *tgbotapi.BotAPI, chatID int64, issueNumber string) (nextIssueNumber string) {
	currentTime := time.Now().Format("2006-01-02 15:04:05")

	diceValues := rollDice(bot, chatID, 3)
	count := sumDiceValues(diceValues)
	singleOrDouble, bigOrSmall := determineResult(count)

	time.Sleep(3 * time.Second)
	triplet := 0
	if diceValues[0] == diceValues[1] && diceValues[1] == diceValues[2] {
		triplet = 1
	}
	message := formatMessage(diceValues[0], diceValues[1], diceValues[2], count, singleOrDouble, bigOrSmall, triplet, issueNumber)

	insertLotteryRecord(chatID, issueNumber, diceValues[0], diceValues[1], diceValues[2], count, singleOrDouble, bigOrSmall, triplet, currentTime)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("开奖历史", "betting_history"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ReplyMarkup = keyboard
	sendMessage(bot, &msg)

	issueNumberInt, _ := strconv.Atoi(issueNumber)
	nextIssueNumber = strconv.Itoa(issueNumberInt + 60)
	lotteryDrawTipMsgConfig := tgbotapi.NewMessage(chatID, fmt.Sprintf("第%s期 1分钟后开奖", nextIssueNumber))
	sendMessage(bot, &lotteryDrawTipMsgConfig)
	return nextIssueNumber
}

// rollDice 模拟多次掷骰子。
func rollDice(bot *tgbotapi.BotAPI, chatID int64, numDice int) []int {
	diceValues := make([]int, numDice)
	diceConfig := tgbotapi.NewDiceWithEmoji(chatID, "🎲")

	for i := 0; i < numDice; i++ {
		diceMsg, _ := bot.Send(diceConfig)
		diceValues[i] = diceMsg.Dice.Value
	}

	return diceValues
}

// sumDiceValues 计算骰子值的总和。
func sumDiceValues(diceValues []int) int {
	sum := 0
	for _, value := range diceValues {
		sum += value
	}
	return sum
}

// determineResult 根据骰子值的总和确定结果（单/双，大/小）。
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

// formatMessage 格式化开奖结果消息。
func formatMessage(valueA int, valueB int, valueC int, count int, singleOrDouble, bigOrSmall string, triplet int, issueNumber string) string {
	tripletStr := ""
	if triplet == 1 {
		tripletStr = "【豹子】\n"
	}
	return fmt.Sprintf(""+
		"点数: %d %d %d \n"+
		"总点数: %d \n"+
		"[单/双]: %s \n"+
		"[大/小]: %s \n"+
		"%s"+
		"期号: %s ",
		valueA, valueB, valueC,
		count,
		singleOrDouble,
		bigOrSmall,
		tripletStr,
		issueNumber,
	)
}

// insertLotteryRecord 将开奖记录插入数据库。
func insertLotteryRecord(chatID int64, issueNumber string, valueA, valueB, valueC, total int, singleOrDouble string, bigOrSmall string, triplet int, currentTime string) {
	record := model.LotteryRecord{
		ChatID:       chatID,
		IssueNumber:  issueNumber,
		ValueA:       valueA,
		ValueB:       valueB,
		ValueC:       valueC,
		Total:        total,
		SingleDouble: singleOrDouble,
		BigSmall:     bigOrSmall,
		Triplet:      triplet,
		Timestamp:    currentTime,
	}

	result := db.Create(&record)
	if result.Error != nil {
		log.Println("插入开奖记录错误:", result.Error)
	}
}
