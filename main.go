package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

const (
	firingEmoji   = "❗️"
	resolvedEmoji = "✅"
)

var (
	bot            *tgbotapi.BotAPI
	warningChatID  string
	criticalChatID string
)

type Alert struct {
	Status string `json:"status"`
	Labels struct {
		AlertName string `json:"alertname"`
		Severity  string `json:"severity"`
	} `json:"labels"`
	Annotations struct {
		Summary     string `json:"summary"`
		Description string `json:"description"`
	} `json:"annotations"`
	StartsAt time.Time `json:"startsAt"`
	EndsAt   time.Time `json:"endsAt,omitempty"`
}

type AlertManagerMessage struct {
	Alerts []Alert `json:"alerts"`
}

func getChatID(severity string) string {
	switch severity {
	case "Warning":
		return warningChatID
	case "Critical":
		return criticalChatID
	default:
		return ""
	}
}

func formatAlertMessage(alert Alert) string {
	var emoji, status string

	switch alert.Status {
	case "firing":
		emoji = firingEmoji
		status = "FIRING"
	case "resolved":
		emoji = resolvedEmoji
		status = "RESOLVED"
	default:
		return "" // Unsupported alert status
	}

	messageText := fmt.Sprintf("%s %s\n🔔 Summary: %s\n📝 Description: %s\n⚠️ Severity: %s\n🕒 Started at: %s UTC",
		emoji,
		status,
		alert.Annotations.Summary,
		alert.Annotations.Description,
		alert.Labels.Severity,
		alert.StartsAt.Format("Jan 02, 15:04:05"))

	if alert.Status == "resolved" {
		messageText += fmt.Sprintf("\n🕒 Resolved at: %s UTC", alert.EndsAt.Format("Jan 02, 15:04:05"))
	}

	return messageText
}

func alertHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received alert")
	var msg AlertManagerMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		log.Printf("Error decoding JSON: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, alert := range msg.Alerts {
		log.Printf("Processing alert: %v", alert)
		chatID := getChatID(alert.Labels.Severity)
		if chatID == "" {
			log.Printf("Unknown severity level: %v", alert.Labels.Severity)
			continue
		}

		chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
		if err != nil {
			log.Printf("Error parsing chat ID: %v", err)
			continue
		}

		messageText := formatAlertMessage(alert)

		if messageText == "" {
			log.Printf("Unsupported alert status: %v", alert.Status)
			continue
		}

		message := tgbotapi.NewMessage(chatIDInt, messageText)
		if _, err := bot.Send(message); err != nil {
			log.Printf("Error sending message to Telegram: %v", err)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func main() {
	var botToken string
	flag.StringVar(&botToken, "token", "", "Telegram Bot API token")
	flag.StringVar(&warningChatID, "warning-chat-id", "", "Chat ID for warning severity alerts")
	flag.StringVar(&criticalChatID, "critical-chat-id", "", "Chat ID for critical severity alerts")
	flag.Parse()

	if botToken == "" || warningChatID == "" || criticalChatID == "" {
		log.Fatalf("All flags -token, -warning-chat-id, and -critical-chat-id are required")
	}

	var err error
	bot, err = tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Error creating Telegram bot: %v", err)
	}
	http.HandleFunc("/alert", alertHandler)
	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
