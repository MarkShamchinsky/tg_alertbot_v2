package main

import (
	"AlertManagerBot/internal/adapter/http/ctrl"
	"AlertManagerBot/internal/app"
	"AlertManagerBot/internal/repo"
	"github.com/spf13/viper"
	"log"
	"net/http"
	"os"
)

func initConfig() {
	configDir := "config"
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		err = os.Mkdir(configDir, 0755)
		if err != nil {
			log.Fatalf("Error creating config directory: %v\n", err)
		}
	}
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}
}

func main() {
	initConfig()

	plusofonApi := viper.GetString("plusofon_token")
	clientID := viper.GetString("client_id")
	botToken := viper.GetString("bot_token")
	warningChatID := viper.GetString("warning_chat_id")
	criticalChatID := viper.GetString("critical_chat_id")

	if plusofonApi == "" || clientID == "" || botToken == "" || warningChatID == "" || criticalChatID == "" {
		log.Fatalf("All configuration values are required")
	}

	scheduleRepo := repo.NewFileScheduleRepository("config/schedule.json")
	scheduleService := repo.NewScheduleService(scheduleRepo)

	plusofon, err := repo.NewPlusofon(plusofonApi, clientID)
	if err != nil {
		log.Fatalf("Error to connect to plusofon API")
	}

	telegramBot, err := repo.NewTelegramBot(botToken, warningChatID, criticalChatID, scheduleService)
	if err != nil {
		log.Fatalf("Error creating Telegram bot: %v", err)
	}

	if err != nil {
		log.Fatalf("Error loading schedules: %v", err)
	}

	alertUseCase := app.NewAlertUseCase(telegramBot, plusofon, scheduleService)
	alertController := ctrl.NewAlertController(alertUseCase)

	http.HandleFunc("/alert", alertController.HandleAlert)
	log.Println("Starting server on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
