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
		AlertName    string `json:"alertname"`
		Severity     string `json:"severity"`
		ErrorMessage string `json:"errorMessage"`
		StrategyName string `json:"strategyName"`
		Name         string `json:"name"`
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

func formatAlertMessage(alerts []Alert) string {
	if len(alerts) == 0 {
		return ""
	}

	alert := alerts[0] // Use the first alert for shared fields
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

	// Load the Moscow timezone location
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Printf("Error loading location: %v", err)
		return ""
	}

	// Convert start and end times to Moscow time
	startsAtMoscow := alert.StartsAt.In(loc)
	endsAtMoscow := alert.EndsAt.In(loc)

	var description string
	if alert.Labels.ErrorMessage != "" {
		// If there's an errorMessage, use it as the Description
		description = alert.Labels.ErrorMessage
	} else {
		// Otherwise, use the existing Description field
		description = alert.Annotations.Description
	}

	// Build the strategies list
	strategiesInfo := ""
	if alert.Labels.ErrorMessage != "" {
		strategiesInfo = "\n\n📋 Strategies:"
		for _, a := range alerts {
			strategiesInfo += fmt.Sprintf("\n%s - %s", a.Labels.Name, a.Labels.StrategyName)
		}
	}

	messageText := fmt.Sprintf("%s %s\n🔔 Summary: %s\n📝 Description: %s\n⚠️ Severity: %s\n🕒 Started at: %s MSK%s",
		emoji,
		status,
		alert.Annotations.Summary,
		description,
		alert.Labels.Severity,
		startsAtMoscow.Format("Jan 02, 15:04:05"),
		strategiesInfo)

	if alert.Status == "resolved" {
		messageText += fmt.Sprintf("\n🕒 Resolved at: %s MSK", endsAtMoscow.Format("Jan 02, 15:04:05"))
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

	alertGroups := make(map[string][]Alert)

	// Group alerts by ErrorMessage
	for _, alert := range msg.Alerts {
		errorMessage := alert.Labels.ErrorMessage
		if errorMessage == "" {
			// Use a special key for alerts without an error message
			errorMessage = "NoErrorMessage"
		}
		alertGroups[errorMessage] = append(alertGroups[errorMessage], alert)
	}

	// Send messages for each group of alerts
	for _, alerts := range alertGroups {
		if len(alerts) == 0 {
			continue
		}

		chatID := getChatID(alerts[0].Labels.Severity)
		if chatID == "" {
			log.Printf("Unknown severity level: %v", alerts[0].Labels.Severity)
			continue
		}

		chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
		if err != nil {
			log.Printf("Error parsing chat ID: %v", err)
			continue
		}

		messageText := formatAlertMessage(alerts)
		if messageText == "" {
			log.Printf("Unsupported alert status: %v", alerts[0].Status)
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
