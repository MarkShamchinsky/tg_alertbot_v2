package repo

import (
	"errors"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
	"log"
	"strconv"
)

const maxMsgLength = 4096

type BotAPI interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

type TelegramBot struct {
	Bot            BotAPI
	warningChatID  string
	criticalChatID string
}

type TelegramSender interface {
	SendMessage(chatID int64, messageText string) error
	GetChatID(severity string) (int64, error)
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
