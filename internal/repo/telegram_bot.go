package repo

import (
	ent "AlertManagerBot/internal/entity"
	"errors"
	"fmt"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
	"log"
	"strconv"
	"strings"
	"time"
)

type BotAPI interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

type TelegramBot struct {
	Bot            BotAPI
	warningChatID  string
	criticalChatID string
	ScheduleSvc    *ScheduleService
	muteUntil      time.Time
}

type TelegramSender interface {
	SendMessage(chatID int64, messageText string) error
	GetChatID(severity string) (int64, error)
	HandleSetScheduleCommand(message *tgbotapi.Message) error
	HandleUpdates(update tgbotapi.Update)
	splitLongMessage(message string) []string
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
	messages := t.splitLongMessage(messageText)
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

func (t *TelegramBot) splitLongMessage(message string) []string {
	if len(message) <= ent.MaxMsgLength {
		return []string{message}
	}

	var result []string
	for len(message) > ent.MaxMsgLength {
		splitIndex := ent.MaxMsgLength
		for splitIndex > 0 && message[splitIndex] != '\n' {
			splitIndex--
		}
		if splitIndex == 0 {
			splitIndex = ent.MaxMsgLength
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
		case "mute":
			err := t.HandleSetMuteUntilCommand(update.Message)
			if err != nil {
				log.Printf("Error handling mute command: %v", err)
			}
		default:
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Unknown command")
			_, err := t.Bot.Send(msg)
			if err != nil {
				log.Printf("Error sending message to Telegram: %v", err)
			}
		}
	} else {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please send a command!")
		_, err := t.Bot.Send(msg)
		if err != nil {
			log.Printf("Error sending message to Telegram: %v", err)
		}
	}
}

// HandleSetScheduleCommand handles the command for adding a schedule
func (t *TelegramBot) HandleSetScheduleCommand(message *tgbotapi.Message) error {
	args := message.CommandArguments()
	parts := strings.Split(args, " ")

	if len(parts)%3 != 0 {
		log.Printf("Invalid command format: %s", args)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Usage: /set_schedule <start_time1> <end_time1> <phone_number1> [<start_time2> <end_time2> <phone_number2> ...]\nExample: /set_schedule 09:00 17:00 +123456789 18:00 20:00 +987654321")
		_, err := t.Bot.Send(msg)
		return err
	}

	for i := 0; i < len(parts); i += 3 {
		startTime, endTime, phoneNumber := parts[i], parts[i+1], parts[i+2]

		log.Printf("Adding schedule for %s - %s: %s", startTime, endTime, phoneNumber)
		err := t.ScheduleSvc.AddSchedule(startTime, endTime, phoneNumber)
		if err != nil {
			log.Printf("Failed to add schedule for %s - %s: %s", startTime, endTime, err)
			msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Failed to add schedule for %s - %s: %s", startTime, endTime, err))
			_, err := t.Bot.Send(msg)
			return err
		}
	}

	log.Printf("All schedules added successfully!")
	msg := tgbotapi.NewMessage(message.Chat.ID, "All schedules added successfully!")
	_, err := t.Bot.Send(msg)
	return err
}

func (t *TelegramBot) HandleSetMuteUntilCommand(message *tgbotapi.Message) error {
	t.muteUntil = time.Now().Add(2 * time.Hour)

	msg := tgbotapi.NewMessage(message.Chat.ID, "Mute for 2 hours")
	_, err := t.Bot.Send(msg)
	return err
}
