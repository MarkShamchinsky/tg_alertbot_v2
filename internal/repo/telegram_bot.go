package repo

import (
	"errors"
	"fmt"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
	"log"
	"strconv"
	"strings"
)

const maxMsgLength = 4096

type BotAPI interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

type TelegramBot struct {
	Bot            BotAPI
	warningChatID  string
	criticalChatID string
	ScheduleSvc    *ScheduleService
}

type TelegramSender interface {
	SendMessage(chatID int64, messageText string) error
	GetChatID(severity string) (int64, error)
	HandleSetScheduleCommand(message *tgbotapi.Message) error
	HandleUpdates(update tgbotapi.Update)
}

func NewTelegramBot(token, warningChatID, criticalChatID string, scheduleSvc *ScheduleService) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &TelegramBot{
		Bot:            bot,
		warningChatID:  warningChatID,
		criticalChatID: criticalChatID,
		ScheduleSvc:    scheduleSvc,
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

func (t *TelegramBot) HandleUpdates(update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	if update.Message.IsCommand() {
		switch update.Message.Command() {
		case "set_schedule":
			err := t.HandleSetScheduleCommand(update.Message)
			if err != nil {
				log.Printf("Error handling set_schedule command: %v", err)
			}
		default:
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Unknown command")
			t.Bot.Send(msg)
		}
	} else {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please send a command!")
		t.Bot.Send(msg)
	}
}

// HandleSetScheduleCommand обрабатывает команду добавления расписания
func (t *TelegramBot) HandleSetScheduleCommand(message *tgbotapi.Message) error {
	args := message.CommandArguments()
	parts := strings.Split(args, " ")
	if len(parts) != 3 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Usage: /set_schedule <start_time> <end_time> <phone_number>\nExample: /set_schedule 09:00 17:00 +123456789")
		_, err := t.Bot.Send(msg)
		return err
	}

	startTime, endTime, phoneNumber := parts[0], parts[1], parts[2]

	err := t.ScheduleSvc.AddSchedule(startTime, endTime, phoneNumber)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Failed to add schedule: %s", err))
		_, err := t.Bot.Send(msg)
		return err
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "Schedule added successfully!")
	_, err = t.Bot.Send(msg)
	return err
}
