package main

import (
	"fmt"
	"tgbot/internal/bot"
	"time"
)

func main() {
	// 设置全局时区
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		fmt.Println("Error loading location:", err)
		return
	}
	time.Local = loc
	bot.StartBot()
}
