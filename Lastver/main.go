package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

const (
	firingEmoji   = "❗️"
	resolvedEmoji = "✅"
	maxMsgLength  = 4096 // Telegram message length limit
)

var (
	bot            *tgbotapi.BotAPI
	warningChatID  string
	criticalChatID string
	resolvedAlerts = make(map[string]bool)
	mu             sync.Mutex
	alertChannel   = make(chan AlertManagerMessage, 100) // Buffered channel for incoming alerts
)

type Alert struct {
	Status string `json:"status"`
	Labels struct {
		AlertName    string `json:"alertname"`
		Severity     string `json:"severity"`
		ErrorMessage string `json:"errorMessage"`
		StrategyName string `json:"strategyName"`
		AlertGroup   string `json:"alertgroup"` // Added alertgroup
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

	var messages []string
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Printf("Error loading location: %v", err)
		return ""
	}

	for _, alert := range alerts {
		var emoji string
		if alert.Status == "firing" {
			emoji = firingEmoji
		} else if alert.Status == "resolved" {
			emoji = resolvedEmoji
		} else {
			continue
		}

		startsAtMoscow := alert.StartsAt.In(loc)
		messageText := fmt.Sprintf("%s %s\n🔔 Summary: %s\n📝 Description: %s\n⚠️ Severity: %s\n🕒 Started at: %s MSK",
			emoji,
			strings.ToUpper(alert.Status),
			alert.Annotations.Summary,
			alert.Annotations.Description,
			alert.Labels.Severity,
			startsAtMoscow.Format("Jan 02, 15:04:05"),
		)

		if alert.Status == "resolved" && !alert.EndsAt.IsZero() {
			endsAtMoscow := alert.EndsAt.In(loc)
			messageText += fmt.Sprintf("\n🕒 Resolved at: %s MSK", endsAtMoscow.Format("Jan 02, 15:04:05"))
		}

		messages = append(messages, messageText)
	}

	return strings.Join(messages, "\n\n")
}

func splitLongMessage(message string) []string {
	if len(message) <= maxMsgLength {
		return []string{message}
	}

	var result []string
	for len(message) > maxMsgLength {
		splitIndex := maxMsgLength
		for splitIndex > 0 && message[splitIndex] != '\n' {
			splitIndex--
		}
		if splitIndex == 0 {
			splitIndex = maxMsgLength
		}

		result = append(result, message[:splitIndex])
		message = message[splitIndex:]
	}
	result = append(result, message)
	return result
}

func sendMessage(chatID int64, messageText string) {
	messages := splitLongMessage(messageText)
	for _, msg := range messages {
		message := tgbotapi.NewMessage(chatID, msg)
		if _, err := bot.Send(message); err != nil {
			log.Printf("Error sending message to Telegram: %v", err)
		}
	}
}

func processAlerts(alerts []Alert) {
	alertGroups := make(map[string][]Alert)

	// Group alerts by AlertGroup
	for _, alert := range alerts {
		groupKey := alert.Labels.AlertGroup
		if groupKey == "" {
			groupKey = "NoAlertGroup" // Fallback if no alertgroup specified
		}
		alertGroups[groupKey] = append(alertGroups[groupKey], alert)
	}

	// Separate groups into firing and resolved alerts
	firingAlerts := make(map[string][]Alert)
	resolvedAlerts := make(map[string][]Alert)

	for groupKey, groupAlerts := range alertGroups {
		for _, alert := range groupAlerts {
			if alert.Status == "firing" {
				firingAlerts[groupKey] = append(firingAlerts[groupKey], alert)
			} else if alert.Status == "resolved" {
				resolvedAlerts[groupKey] = append(resolvedAlerts[groupKey], alert)
			}
		}
	}

	// Send messages for firing alerts
	for _, groupedAlerts := range firingAlerts {
		if len(groupedAlerts) == 0 {
			continue
		}

		chatID := getChatID(groupedAlerts[0].Labels.Severity)
		if chatID == "" {
			log.Printf("Unknown severity level: %v", groupedAlerts[0].Labels.Severity)
			continue
		}

		chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
		if err != nil {
			log.Printf("Error parsing chat ID: %v", err)
			continue
		}

		messageText := formatAlertMessage(groupedAlerts)
		if messageText == "" {
			log.Printf("Unsupported alert status: %v", groupedAlerts[0].Status)
			continue
		}

		sendMessage(chatIDInt, messageText)
	}

	// Send messages for resolved alerts
	for _, groupedAlerts := range resolvedAlerts {
		if len(groupedAlerts) == 0 {
			continue
		}

		chatID := getChatID(groupedAlerts[0].Labels.Severity)
		if chatID == "" {
			log.Printf("Unknown severity level: %v", groupedAlerts[0].Labels.Severity)
			continue
		}

		chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
		if err != nil {
			log.Printf("Error parsing chat ID: %v", err)
			continue
		}

		messageText := formatAlertMessage(groupedAlerts)
		if messageText == "" {
			log.Printf("Unsupported alert status: %v", groupedAlerts[0].Status)
			continue
		}

		sendMessage(chatIDInt, messageText)
	}
}

func worker() {
	for msg := range alertChannel {
		processAlerts(msg.Alerts)
	}
}

func alertHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received alert")
	var msg AlertManagerMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		log.Printf("Error decoding JSON: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Send the alert message to the worker pool
	alertChannel <- msg

	w.WriteHeader(http.StatusOK)
}

func main() {
	var botToken string
	var numWorkers int
	flag.StringVar(&botToken, "token", "", "Telegram Bot API token")
	flag.StringVar(&warningChatID, "warning-chat-id", "", "Chat ID for warning severity alerts")
	flag.StringVar(&criticalChatID, "critical-chat-id", "", "Chat ID for critical severity alerts")
	flag.IntVar(&numWorkers, "num-workers", 5, "Number of worker goroutines to process alerts")
	flag.Parse()

	if botToken == "" || warningChatID == "" || criticalChatID == "" {
		log.Fatalf("All flags -token, -warning-chat-id, and -critical-chat-id are required")
	}

	var err error
	bot, err = tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Error creating Telegram bot: %v", err)
	}

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		go worker()
	}

	http.HandleFunc("/alert", alertHandler)
	log.Println("Starting server on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
