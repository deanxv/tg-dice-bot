package bot

import (
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"tg-dice-bot/internal/model"
)

const (
	RedisCurrentIssueKey = "current_issue:%d"
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
		if message.Chat.IsSuperGroup() || message.Chat.IsGroup() {
			handleGroupCommand(bot, user.UserName, chatMember, message.Command(), chatID, messageID)
		} else {
			handlePrivateCommand(bot, chatMember, chatID, messageID, message.Command())
		}
	} else if message.Text != "" {
		log.Println("text:" + message.Text)
		handleBettingCommand(bot, user.ID, chatID, messageID, message.Text)
	}
}

// handleBettingCommand 处理下注命令
func handleBettingCommand(bot *tgbotapi.BotAPI, userID int64, chatID int64, messageID int, text string) {

	// 解析下注命令，示例命令格式：#单 20
	// 这里需要根据实际需求进行合适的解析，示例中只是简单示范
	parts := strings.Fields(text)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "#") {
		return
	}

	// 获取下注类型和下注积分
	betType := parts[0][1:]
	if betType != "单" && betType != "双" && betType != "大" && betType != "小" && betType != "豹子" {
		return
	}

	betAmount, err := strconv.Atoi(parts[1])
	if err != nil || betAmount <= 0 {
		return
	}

	var chatDiceConfig model.ChatDiceConfig
	result := db.Where("enable = ? AND chat_id = ?", 1, chatID).First(&chatDiceConfig)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		registrationMsg := tgbotapi.NewMessage(chatID, "功能未开启！")
		registrationMsg.ReplyToMessageID = messageID
		_, err := bot.Send(registrationMsg)
		if err != nil {
			log.Println("功能未开启提示消息错误:", err)
		}
		return
	} else if result.Error != nil {
		log.Println("下注命令错误", result.Error)
		return
	}
	// 获取当前进行的期号
	redisKey := fmt.Sprintf(RedisCurrentIssueKey, chatID)
	issueNumberResult := redisDB.Get(redisDB.Context(), redisKey)
	if errors.Is(issueNumberResult.Err(), redis.Nil) || issueNumberResult == nil {
		log.Printf("键 %s 不存在", redisKey)
		replyMsg := tgbotapi.NewMessage(chatID, "当前暂无开奖活动!")
		replyMsg.ReplyToMessageID = messageID
		_, err = bot.Send(replyMsg)
		return
	} else if issueNumberResult.Err() != nil {
		log.Println("获取值时发生错误:", issueNumberResult.Err())
		return
	}

	issueNumber, _ := issueNumberResult.Result()

	// 存储下注记录到数据库，并扣除用户余额
	err = storeBetRecord(bot, userID, chatID, issueNumber, messageID, betType, betAmount)
	if err != nil {
		// 回复余额不足信息等
		log.Println("存储下注记录错误:", err)
		return
	}

	// 回复下注成功信息
	replyMsg := tgbotapi.NewMessage(chatID, "下注成功!")
	replyMsg.ReplyToMessageID = messageID

	sentMsg, err := bot.Send(replyMsg)
	if err != nil {
		log.Println("发送消息错误:", err)
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

// storeBetRecord 函数中扣除用户余额并保存下注记录
func storeBetRecord(bot *tgbotapi.BotAPI, userID int64, chatID int64, issueNumber string, messageID int, betType string, betAmount int) error {
	// 获取用户对应的互斥锁
	userLock := getUserLock(userID)
	userLock.Lock()
	defer userLock.Unlock()

	// 获取用户信息
	var user model.TgUser
	result := db.Where("user_id = ? AND chat_id = ?", userID, chatID).First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// 用户不存在，发送注册提示
		registrationMsg := tgbotapi.NewMessage(chatID, "你还未注册，使用 /register 进行注册。")
		registrationMsg.ReplyToMessageID = messageID
		_, err := bot.Send(registrationMsg)
		if err != nil {
			log.Println("发送注册提示消息错误:", err)
			return err
		}
		return result.Error
	}

	// 检查用户余额是否足够
	if user.Balance < betAmount {
		// 用户不存在，发送注册提示
		balanceInsufficientMsg := tgbotapi.NewMessage(chatID, "你的余额不足！")
		balanceInsufficientMsg.ReplyToMessageID = messageID
		_, err := bot.Send(balanceInsufficientMsg)
		if err != nil {
			log.Println("你的余额不足提示错误:", err)
			return err
		} else {
			return errors.New("余额不足")
		}
	}

	// 扣除用户余额
	user.Balance -= betAmount
	result = db.Save(&user)
	if result.Error != nil {
		log.Println("扣除用户余额错误:", result.Error)
		return result.Error
	}

	// 保存下注记录
	betRecord := model.BetRecord{
		UserID:      userID,
		ChatID:      chatID,
		BetType:     betType,
		BetAmount:   betAmount,
		IssueNumber: issueNumber,
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
	}

	result = db.Create(&betRecord)
	if result.Error != nil {
		log.Println("保存下注记录错误:", result.Error)
		// 如果保存下注记录失败，需要返还用户余额
		user.Balance += betAmount
		db.Save(&user)
		return result.Error
	}

	return nil
}

// handleGroupCommand 处理群聊中的命令。
func handleGroupCommand(bot *tgbotapi.BotAPI, username string, chatMember tgbotapi.ChatMember, command string, chatID int64, messageID int) {
	if command == "start" {
		if !chatMember.IsAdministrator() && !chatMember.IsCreator() {
			msgConfig := tgbotapi.NewMessage(chatID, "请勿使用管理员命令")
			msgConfig.ReplyToMessageID = messageID
			sendMessage(bot, &msgConfig)
			return
		}
		handleStartCommand(bot, chatID, messageID)
	} else if command == "stop" {
		if !chatMember.IsAdministrator() && !chatMember.IsCreator() {
			msgConfig := tgbotapi.NewMessage(chatID, "请勿使用管理员命令")
			msgConfig.ReplyToMessageID = messageID
			sendMessage(bot, &msgConfig)
			return
		}
		handleStopCommand(bot, chatID, messageID)
	} else if command == "register" {
		handleRegisterCommand(bot, chatMember, chatID, messageID)
	} else if command == "sign" {
		handleSignInCommand(bot, chatMember, chatID, messageID)
	} else if command == "my" {
		handleMyCommand(bot, chatMember, chatID, messageID)
	} else if command == "iampoor" {
		handlePoorCommand(bot, chatMember, chatID, messageID)
	} else if command == "help" {
		handleHelpCommand(bot, chatID, messageID)
	}

}

func handleRegisterCommand(bot *tgbotapi.BotAPI, chatMember tgbotapi.ChatMember, chatID int64, messageID int) {
	// 获取用户对应的互斥锁
	userLock := getUserLock(chatMember.User.ID)
	userLock.Lock()
	defer userLock.Unlock()

	var user model.TgUser
	result := db.Where("user_id = ? AND chat_id = ?", chatMember.User.ID, chatID).First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// 没有找到记录
		err := registerUser(chatMember.User.ID, chatMember.User.UserName, chatID)
		if err != nil {
			log.Println("用户注册错误:", err)
		} else {
			msgConfig := tgbotapi.NewMessage(chatID, "注册成功！奖励1000积分！")
			msgConfig.ReplyToMessageID = messageID
			sendMessage(bot, &msgConfig)
		}
		return
	} else if result.Error != nil {
		log.Println("查询错误:", result.Error)
		return
	} else {
		msgConfig := tgbotapi.NewMessage(chatID, "请勿重复注册！")
		msgConfig.ReplyToMessageID = messageID
		sendMessage(bot, &msgConfig)
		return
	}
}

func handleSignInCommand(bot *tgbotapi.BotAPI, chatMember tgbotapi.ChatMember, chatID int64, messageID int) {
	// 获取用户对应的互斥锁
	userLock := getUserLock(chatMember.User.ID)
	userLock.Lock()
	defer userLock.Unlock()

	var user model.TgUser
	result := db.Where("user_id = ? AND chat_id = ?", chatMember.User.ID, chatID).First(&user)
	if result.Error != nil {
		log.Println("查询错误:", result.Error)
		return
	} else if user.ID == 0 {
		// 没有找到记录
		msgConfig := tgbotapi.NewMessage(chatID, "请发送 /register 注册用户！")
		msgConfig.ReplyToMessageID = messageID
		sendMessage(bot, &msgConfig)
		return
	} else {
		if user.SignInTime != "" {
			signInTime, err := time.Parse("2006-01-02 15:04:05", user.SignInTime)
			if err != nil {
				fmt.Println("时间解析错误:", err)
				return
			}
			// 获取当前时间
			currentTime := time.Now()
			currentMidnight := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, 0, currentTime.Location())
			if !signInTime.Before(currentMidnight) {
				msgConfig := tgbotapi.NewMessage(chatID, "今天已签到过了哦！")
				msgConfig.ReplyToMessageID = messageID
				sendMessage(bot, &msgConfig)
				return
			}
		}
		user.SignInTime = time.Now().Format("2006-01-02 15:04:05")
		user.Balance += 1000
		result = db.Save(&user)
		msgConfig := tgbotapi.NewMessage(chatID, "签到成功！奖励1000积分！")
		msgConfig.ReplyToMessageID = messageID
		sendMessage(bot, &msgConfig)

	}
}

func handleMyCommand(bot *tgbotapi.BotAPI, chatMember tgbotapi.ChatMember, chatID int64, messageID int) {
	var user model.TgUser
	result := db.Where("user_id = ? AND chat_id = ?", chatMember.User.ID, chatID).First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// 没有找到记录
		msgConfig := tgbotapi.NewMessage(chatID, "请发送 /register 注册用户！")
		msgConfig.ReplyToMessageID = messageID
		sendMessage(bot, &msgConfig)
		return
	} else if result.Error != nil {
		log.Println("查询错误:", result.Error)
		return
	} else {
		msgConfig := tgbotapi.NewMessage(chatID, fmt.Sprintf("%s 你的积分余额为%d", chatMember.User.LastName, user.Balance))
		msgConfig.ReplyToMessageID = messageID
		sendMessage(bot, &msgConfig)
	}
}

func handlePoorCommand(bot *tgbotapi.BotAPI, chatMember tgbotapi.ChatMember, chatID int64, messageID int) {
	// 获取用户对应的互斥锁
	userLock := getUserLock(chatMember.User.ID)
	userLock.Lock()
	defer userLock.Unlock()

	var user model.TgUser
	result := db.Where("user_id = ? AND chat_id = ?", chatMember.User.ID, chatID).First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// 没有找到记录
		msgConfig := tgbotapi.NewMessage(chatID, "请发送 /register 注册用户！")
		msgConfig.ReplyToMessageID = messageID
		sendMessage(bot, &msgConfig)
		return
	} else if result.Error != nil {
		log.Println("查询错误:", result.Error)
		return
	} else {
		if user.Balance > 1000 {
			msgConfig := tgbotapi.NewMessage(chatID, "1000积分以下才可以领取低保哦")
			msgConfig.ReplyToMessageID = messageID
			sendMessage(bot, &msgConfig)
			return
		}
		user.Balance += 1000
		result = db.Save(&user)
		msgConfig := tgbotapi.NewMessage(chatID, "领取低保成功！获得1000积分！")
		msgConfig.ReplyToMessageID = messageID
		sendMessage(bot, &msgConfig)
	}
}

// registerUser 函数用于用户注册时插入初始数据到数据库
func registerUser(userID int64, userName string, chatID int64) error {
	initialBalance := 1000
	newUser := model.TgUser{
		UserID:   userID,
		ChatID:   chatID,
		Username: userName,
		Balance:  initialBalance,
	}

	result := db.Create(&newUser)
	return result.Error
}

// handlePrivateCommand 处理私聊中的命令。
func handlePrivateCommand(bot *tgbotapi.BotAPI, chatMember tgbotapi.ChatMember, chatID int64, messageID int, command string) {
	switch command {
	case "stop":
		handleStopCommand(bot, chatID, messageID)
	case "start":
		handleStartCommand(bot, chatID, messageID)
	case "help":
		handleHelpCommand(bot, chatID, messageID)
	case "register":
		handleRegisterCommand(bot, chatMember, chatID, messageID)
	case "sign":
		handleSignInCommand(bot, chatMember, chatID, messageID)
	case "my":
		handleMyCommand(bot, chatMember, chatID, messageID)
	case "iampoor":
		handlePoorCommand(bot, chatMember, chatID, messageID)
	}
}

// handleStopCommand 处理 "stop" 命令。
func handleStopCommand(bot *tgbotapi.BotAPI, chatID int64, messageID int) {

	var chatDiceConfig model.ChatDiceConfig
	// 更新开奖配置
	chatDiceConfigResult := db.First(&chatDiceConfig, "chat_id = ?", chatID)
	if errors.Is(chatDiceConfigResult.Error, gorm.ErrRecordNotFound) {
		msgConfig := tgbotapi.NewMessage(chatID, "开启后才可关闭！")
		msgConfig.ReplyToMessageID = messageID
		sendMessage(bot, &msgConfig)
		return
	} else if chatDiceConfigResult.Error != nil {
		log.Println("开奖配置初始化错误", chatDiceConfigResult.Error)
		return
	} else {
		chatDiceConfig.Enable = 0
		result := db.Model(&model.ChatDiceConfig{}).Where("chat_id = ?", chatID).Update("enable", 0)
		if result.Error != nil {
			log.Println("开奖配置初始化失败: " + result.Error.Error())
			return
		}
	}

	msgConfig := tgbotapi.NewMessage(chatID, "已关闭")
	msgConfig.ReplyToMessageID = messageID
	sendMessage(bot, &msgConfig)
	stopDice(chatID)
}

// handleStartCommand 处理 "start" 命令。
func handleStartCommand(bot *tgbotapi.BotAPI, chatID int64, messageID int) {
	var chatDiceConfig *model.ChatDiceConfig
	// 更新开奖配置
	chatDiceConfigResult := db.First(&chatDiceConfig, "chat_id = ?", chatID)
	if errors.Is(chatDiceConfigResult.Error, gorm.ErrRecordNotFound) {
		// 开奖配置不存在 则保存
		chatDiceConfig = &model.ChatDiceConfig{
			ChatID:           chatID,
			LotteryDrawCycle: 1, // 开奖周期(分钟)
			Enable:           1, // 开启状态
		}
		db.Create(&chatDiceConfig)
	} else if chatDiceConfigResult.Error != nil {
		log.Println("开奖配置初始化错误", chatDiceConfigResult.Error)
		return
	} else {
		chatDiceConfig.Enable = 1
		result := db.Model(&model.ChatDiceConfig{}).Where("chat_id = ?", chatID).Update("enable", 1)
		if result.Error != nil {
			log.Println("开奖配置初始化失败: " + result.Error.Error())
			return
		}
	}

	msgConfig := tgbotapi.NewMessage(chatID, "已开启")
	msgConfig.ReplyToMessageID = messageID
	sendMessage(bot, &msgConfig)

	issueNumber := time.Now().Format("20060102150405")

	// 查找上个未开奖的期号
	redisKey := fmt.Sprintf(RedisCurrentIssueKey, chatID)
	issueNumberResult := redisDB.Get(redisDB.Context(), redisKey)
	if issueNumberResult.Err() == nil {
		result, _ := issueNumberResult.Result()
		issueNumber = result
		lotteryDrawTipMsgConfig := tgbotapi.NewMessage(chatID, fmt.Sprintf("第%s期 %d分钟后开奖", issueNumber, chatDiceConfig.LotteryDrawCycle))
		sendMessage(bot, &lotteryDrawTipMsgConfig)
	} else {
		lotteryDrawTipMsgConfig := tgbotapi.NewMessage(chatID, fmt.Sprintf("第%s期 %d分钟后开奖", issueNumber, chatDiceConfig.LotteryDrawCycle))
		sendMessage(bot, &lotteryDrawTipMsgConfig)
		// 存储当前期号和对话ID
		err := redisDB.Set(redisDB.Context(), redisKey, issueNumber, 0).Err()
		if err != nil {
			log.Println("存储新期号和对话ID错误:", err)
			return
		}
	}

	//redisKey := fmt.Sprintf(RedisCurrentIssueKey, chatID)
	go startDice(bot, chatID, issueNumber)
}

// handleHelpCommand 处理 "help" 命令。
func handleHelpCommand(bot *tgbotapi.BotAPI, chatID int64, messageID int) {
	msgConfig := tgbotapi.NewMessage(chatID, "/help帮助\n"+
		"/start 开启\n"+
		"/stop 关闭\n"+
		"/register 用户注册\n"+
		"/sign 用户签到\n"+
		"/my 查询积分\n"+
		"/iampoor 领取低保\n"+
		"玩法例子(竞猜-单,下注-20): #单 20\n"+
		"默认开奖周期: 1分钟")
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
		var chatDiceConfig model.ChatDiceConfig
		db.Where("chat_id = ?", chatID).First(&chatDiceConfig)
		ticker := time.NewTicker(time.Duration(chatDiceConfig.LotteryDrawCycle) * time.Minute)
		defer ticker.Stop()

		// 查找上个未开奖的期号
		redisKey := fmt.Sprintf(RedisCurrentIssueKey, chatID)
		issueNumberResult := redisDB.Get(redisDB.Context(), redisKey)
		if issueNumberResult == nil {
			result, _ := issueNumberResult.Result()
			issueNumber = result
		}

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

	redisKey := fmt.Sprintf(RedisCurrentIssueKey, chatID)
	// 删除当前期号和对话ID
	err := redisDB.Del(redisDB.Context(), redisKey).Err()
	if err != nil {
		log.Println("删除当前期号和对话ID错误:", err)
		return
	}

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

	//issueNumberInt, _ := strconv.Atoi(issueNumber)
	nextIssueNumber = time.Now().Format("20060102150405")
	var chatDiceConfig model.ChatDiceConfig
	db.Where("enable = ? AND chat_id = ?", 1, chatID).First(&chatDiceConfig)
	lotteryDrawTipMsgConfig := tgbotapi.NewMessage(chatID, fmt.Sprintf("第%s期 %d分钟后开奖", nextIssueNumber, chatDiceConfig.LotteryDrawCycle))
	sendMessage(bot, &lotteryDrawTipMsgConfig)

	// 设置新的期号和对话ID
	err = redisDB.Set(redisDB.Context(), redisKey, nextIssueNumber, 0).Err()
	if err != nil {
		log.Println("存储新期号和对话ID错误:", err)
	}

	// 遍历下注记录，计算竞猜结果
	go func() {
		// 获取所有参与竞猜的用户下注记录
		betRecords, err := model.GetBetRecordsByChatIDAndIssue(db, chatID, issueNumber)
		if err != nil {
			log.Println("获取用户下注记录错误:", err)
			return
		}
		// 获取当前期数开奖结果
		var lotteryRecord model.LotteryRecord
		db.Where("issue_number = ? AND chat_id = ?", issueNumber, chatID).First(&lotteryRecord)

		for _, betRecord := range betRecords {
			// 更新用户余额
			updateBalance(betRecord, &lotteryRecord)
		}
	}()

	return nextIssueNumber
}

// updateBalance 更新用户余额
func updateBalance(betRecord model.BetRecord, lotteryRecord *model.LotteryRecord) {

	// 获取用户对应的互斥锁
	userLock := getUserLock(betRecord.UserID)
	userLock.Lock()
	defer userLock.Unlock()

	var user model.TgUser
	result := db.Where("user_id = ? and chat_id = ?", betRecord.UserID, lotteryRecord.ChatID).First(&user)
	if result.Error != nil {
		log.Println("获取用户信息错误:", result.Error)
		return
	}

	if betRecord.BetType == lotteryRecord.SingleDouble ||
		betRecord.BetType == lotteryRecord.BigSmall {
		user.Balance += betRecord.BetAmount * 2
	} else if betRecord.BetType == "豹子" && lotteryRecord.Triplet == 1 {
		user.Balance += betRecord.BetAmount * 10
	}

	result = db.Save(&user)
	if result.Error != nil {
		log.Println("更新用户余额错误:", result.Error)
	}
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
