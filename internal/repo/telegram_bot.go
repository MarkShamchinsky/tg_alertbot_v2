package repo

import (
	"AlertManagerBot/internal/app"
	"errors"
	"log"
	"strconv"
	"strings"

	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

const maxMsgLength = 4096

type BotAPI interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	GetUpdatesChan(config tgbotapi.UpdateConfig) (tgbotapi.UpdatesChannel, error)
}

type TelegramBot struct {
	Bot            BotAPI
	warningChatID  string
	criticalChatID string
}

type TelegramSender interface {
	SendMessage(chatID int64, messageText string) error
	GetChatID(severity string) (int64, error)
	HandleMessage(message *tgbotapi.Message)
	ListenForMessages()
}

func NewTelegramBot(token, warningChatID, criticalChatID string) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &TelegramBot{
		Bot:            bot,
		warningChatID:  warningChatID,
		criticalChatID: criticalChatID,
	}, nil
}

func (t *TelegramBot) SendMessage(chatID int64, messageText string) error {
	messages := splitLongMessage(messageText)
	for _, msg := range messages {
		message := tgbotapi.NewMessage(chatID, msg)
		if _, err := t.Bot.Send(message); err != nil {
			log.Printf("Error sending message to Telegram: %v", err)
			return err
		}
	}
	return nil
}

func (t *TelegramBot) GetChatID(severity string) (int64, error) {
	var chatID string
	switch severity {
	case "Warning":
		chatID = t.warningChatID
	case "Critical":
		chatID = t.criticalChatID
	default:
		return 0, errors.New("unknown severity level")
	}
	return strconv.ParseInt(chatID, 10, 64)
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

// HandleMessage обрабатывает входящие сообщения от пользователя
func (t *TelegramBot) HandleMessage(message *tgbotapi.Message) {
	text := message.Text
	parts := strings.Split(text, " - ")
	if len(parts) != 3 {
		t.SendMessage(message.Chat.ID, "Invalid format. Use +7XXXXXXXXXX - HH:MM-HH:MM")
		return
	}

	phoneNumber := parts[0]
	timeRange := parts[1]
	times := strings.Split(timeRange, "-")
	if len(times) != 2 {
		t.SendMessage(message.Chat.ID, "Invalid time range format. Use HH:MM-HH:MM")
		return
	}

	startTime := strings.TrimSpace(times[0])
	endTime := strings.TrimSpace(times[1])

	err := app.AddSchedule(startTime, endTime, phoneNumber)
	if err != nil {
		t.SendMessage(message.Chat.ID, "Error saving schedule: "+err.Error())
	} else {
		t.SendMessage(message.Chat.ID, "Schedule saved successfully.")
	}
}

// ListenForMessages запускает обработку сообщений от пользователей
func (t *TelegramBot) ListenForMessages() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := t.Bot.GetUpdatesChan(u)
	if err != nil {
		log.Fatalf("Error getting updates: %v", err)
	}

	for update := range updates {
		if update.Message != nil {
			t.HandleMessage(update.Message)
		}
	}
}
