package bot

import (
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gorm.io/gorm"
	"log"
	"os"
	"tg-dice-bot/internal/database"
	"tg-dice-bot/internal/model"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	TelegramAPIToken = "TELEGRAM_API_TOKEN"
)

var (
	db      *gorm.DB
	redisDB *redis.Client
)

func StartBot() {
	initDB()

	bot := initTelegramBot()

	initDiceTask(bot)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60
	updates := bot.GetUpdatesChan(updateConfig)

	for update := range updates {
		if update.Message != nil {
			go handleMessage(bot, update.Message)
		} else if update.CallbackQuery != nil {
			go handleCallbackQuery(bot, update.CallbackQuery)
		}
	}
}

func initDiceTask(bot *tgbotapi.BotAPI) {

	// 查出所有已开启的对话
	chatDiceConfigs, err := model.ListByEnable(db, 1)
	if err != nil {
		log.Fatal("初始化任务失败:", err)
	}
	for _, config := range chatDiceConfigs {
		// 查询当前对话在缓存中是否有未执行的任务
		redisKey := fmt.Sprintf(RedisCurrentIssueKey, config.ChatID)
		issueNumberResult := redisDB.Get(redisDB.Context(), redisKey)
		if errors.Is(issueNumberResult.Err(), redis.Nil) || issueNumberResult == nil {
			// 没有未开奖的任务，开始新的期号
			log.Printf("键 %s 不存在", redisKey)
			issueNumber := time.Now().Format("20060102150405")

			go StartDice(bot, config.ChatID, issueNumber)
			continue
		} else if issueNumberResult.Err() != nil {
			log.Println("获取值时发生错误:", issueNumberResult.Err())
			continue
		} else {
			// 有未开奖的任务
			result, _ := issueNumberResult.Result()
			log.Printf("有未开奖的任务期号:%s", result)
			go StartDice(bot, config.ChatID, result)
			continue
		}
	}

}

func initDB() {
	var err error
	db, err = database.InitDB(os.Getenv(database.DBConnectionString))
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	err = db.AutoMigrate(&model.LotteryRecord{})
	if err != nil {
		log.Fatal("自动迁移表结构失败:", err)
	}

	err = db.AutoMigrate(&model.TgUser{})
	if err != nil {
		log.Fatal("自动迁移表结构失败:", err)
	}

	err = db.AutoMigrate(&model.BetRecord{})
	if err != nil {
		log.Fatal("自动迁移表结构失败:", err)
	}

	err = db.AutoMigrate(&model.ChatDiceConfig{})
	if err != nil {
		log.Fatal("自动迁移表结构失败:", err)
	}

	redisDB, err = database.InitRedisDB(os.Getenv(database.RedisDBConnectionString))
	if err != nil {
		log.Fatal("连接Redis数据库失败:", err)
	}

}
func initTelegramBot() *tgbotapi.BotAPI {
	bot, err := tgbotapi.NewBotAPI(os.Getenv(TelegramAPIToken))
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = false
	log.Printf("已授权帐户 %s", bot.Self.UserName)
	return bot
}
