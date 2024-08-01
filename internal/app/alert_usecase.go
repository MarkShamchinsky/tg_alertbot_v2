package app

import (
	ent "AlertManagerBot/internal/entity"
	"AlertManagerBot/internal/repo"

	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type AlertSender interface {
	SendAlerts(alerts []ent.Alert)
	MakeCall(data ent.CallData) error
	MarkCallSuccessful(phoneNumber string) error
}

type alertUseCase struct {
	alertRepo   repo.TelegramSender
	plusofon    *repo.Plusofon
	scheduleSvc *repo.ScheduleService
}

func NewAlertUseCase(ts repo.TelegramSender, p *repo.Plusofon, scheduleSvc *repo.ScheduleService) AlertSender {
	return &alertUseCase{
		alertRepo:   ts,
		plusofon:    p,
		scheduleSvc: scheduleSvc,
	}
}

func (u *alertUseCase) SendAlerts(alerts []ent.Alert) {
	alertGroups := u.groupAlertsByAlertGroup(alerts)
	firingAlerts, resolvedAlerts := u.separateAlertsByStatus(alertGroups)

	u.sendGroupedAlerts(firingAlerts)
	u.sendGroupedAlerts(resolvedAlerts)
	for _, alerts := range firingAlerts {
		for _, alert := range alerts {
			if alert.Labels.Severity == "Critical" {
				phoneNumber, err := u.scheduleSvc.GetPhoneNumberByTime()
				if err != nil {
					log.Printf("Error getting phone number by time: %v", err)
					return
				}
				callData := ent.CallData{
					Number:     phoneNumber,
					LineNumber: "74951332210",
					SipID:      "51326",
				}
				err = u.MakeCall(callData)
				if err != nil {
					log.Printf("Error making call: %v", err)
				}
			}
		}
	}
}

func (u *alertUseCase) MakeCall(data ent.CallData) error {

	var attempts int
	maxAttempts := 3
	phoneNumber := data.Number

	for {
		// Формируем запрос на звонок
		url := "https://restapi.plusofon.ru/api/v1/call/quickcall"
		JsonPayLoad, _ := json.Marshal(data)

		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(JsonPayLoad))
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Client", u.plusofon.ClientID)
		req.Header.Add("Authorization", "Bearer "+u.plusofon.PlusofonToken)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			// Если звонок успешен, помечаем его как успешный
			err = u.scheduleSvc.MarkCallSuccessful(phoneNumber)
			if err != nil {
				log.Printf("Error marking call successful: %v", err)
			}
			return nil // Звонок успешен, выходим из функции
		}

		// Проверяем на ошибки выполнения HTTP-запроса и статус ответа
		if err != nil {
			log.Printf("Error making call: %v", err)
		} else if resp.StatusCode != http.StatusOK {
			log.Printf("Failed to make call, status: %s", resp.Status)
		}

		attempts++
		if attempts >= maxAttempts {
			// Если попытки исчерпаны, переходим к следующему номеру
			nextPhoneNumber, err := u.scheduleSvc.GetNextPhoneNumber(phoneNumber)
			if err != nil {
				log.Printf("Error getting next phone number: %v", err)
				return err
			}
			phoneNumber = nextPhoneNumber
			data.Number = phoneNumber
			attempts = 0 // Сбрасываем счетчик попыток для нового номера
		}
	}
}

func (u *alertUseCase) groupAlertsByAlertGroup(alerts []ent.Alert) map[string][]ent.Alert {
	alertGroups := make(map[string][]ent.Alert)
	for _, alert := range alerts {
		groupKey := alert.Labels.AlertGroup
		if groupKey == "" {
			groupKey = "NoAlertGroup" // Если группа не указана, назначается "NoAlertGroup"
		}
		alertGroups[groupKey] = append(alertGroups[groupKey], alert)
	}
	return alertGroups
}

func (u *alertUseCase) separateAlertsByStatus(alertGroups map[string][]ent.Alert) (map[string][]ent.Alert, map[string][]ent.Alert) {
	firingAlerts := make(map[string][]ent.Alert)
	resolvedAlerts := make(map[string][]ent.Alert)
	for groupKey, groupAlerts := range alertGroups {
		for _, alert := range groupAlerts {
			if alert.Status == "firing" {
				firingAlerts[groupKey] = append(firingAlerts[groupKey], alert)
			} else if alert.Status == "resolved" {
				resolvedAlerts[groupKey] = append(resolvedAlerts[groupKey], alert)
			}
		}
	}
	return firingAlerts, resolvedAlerts
}

func (u *alertUseCase) sendGroupedAlerts(alerts map[string][]ent.Alert) {
	for _, groupedAlerts := range alerts {
		if len(groupedAlerts) == 0 {
			continue
		}
		chatID, err := u.alertRepo.GetChatID(groupedAlerts[0].Labels.Severity)
		if err != nil {
			log.Printf("Error getting chat ID: %v", err)
			continue
		}

		messageText := u.formatAlertMessage(groupedAlerts)
		if messageText == "" {
			log.Printf("Unsupported alert status: %v", groupedAlerts[0].Status)
			continue
		}

		if err := u.alertRepo.SendMessage(chatID, messageText); err != nil {
			log.Printf("Error sending message to Telegram: %v", err)
		}
	}
}

func (u *alertUseCase) formatAlertMessage(alerts []ent.Alert) string {
	var messages []string
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Printf("Error loading location: %v", err)
		return ""
	}

	for _, alert := range alerts {
		var emoji string
		if alert.Status == "firing" {
			emoji = ent.FiringEmoji
		} else if alert.Status == "resolved" {
			emoji = ent.ResolvedEmoji
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

func (u *alertUseCase) MarkCallSuccessful(phoneNumber string) error {
	return u.scheduleSvc.MarkCallSuccessful(phoneNumber)
}
