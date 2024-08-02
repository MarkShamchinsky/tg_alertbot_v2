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
	groupAlertsByAlertGroup(alerts []ent.Alert) map[string][]ent.Alert
	separateAlertsByStatus(alertGroups map[string][]ent.Alert) (map[string][]ent.Alert, map[string][]ent.Alert)
	sendGroupedAlerts(alerts map[string][]ent.Alert)
	formatAlertMessage(alerts []ent.Alert) string
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
	log.Println("Starting SendAlerts")
	if len(alerts) == 0 {
		log.Println("No alerts to send")
		return
	}

	firstAlert := alerts[0]

	if firstAlert.Labels.Severity == "Critical" {
		log.Println("Severity is Critical")
		phoneNumber, err := u.scheduleSvc.GetPhoneNumberByTime()
		if err != nil {
			log.Printf("Error getting phone number by time: %v", err)
			return
		}
		log.Printf("Phone number: %s", phoneNumber)
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

	alertGroups := u.groupAlertsByAlertGroup(alerts)
	firingAlerts, resolvedAlerts := u.separateAlertsByStatus(alertGroups)

	log.Printf("Number of firing alerts: %d", len(firingAlerts))
	log.Printf("Number of resolved alerts: %d", len(resolvedAlerts))

	u.sendGroupedAlerts(firingAlerts)
	u.sendGroupedAlerts(resolvedAlerts)

	log.Println("Finished SendAlerts")
}

func (u *alertUseCase) MakeCall(callData ent.CallData) error {
	if u.scheduleSvc.IsMuted() {
		log.Printf("Call was muted")
		return nil
	}
	const maxAttempts = 3
	for attempts := 0; attempts < maxAttempts; attempts++ {
		payload, err := json.Marshal(callData)
		if err != nil {
			return fmt.Errorf("failed to marshal call data: %w", err)
		}

		req, err := http.NewRequest(http.MethodPost, "https://restapi.plusofon.ru/api/v1/call/quickcall", bytes.NewBuffer(payload))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Client", u.plusofon.ClientID)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", u.plusofon.PlusofonToken))

		client := &http.Client{}
		resp, err := client.Do(req)

		if err != nil {
			return fmt.Errorf("failed to make call: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			if err := u.scheduleSvc.MarkCallSuccessful(callData.Number); err != nil {
				return fmt.Errorf("failed to mark call successful: %w", err)
			}
			return nil
		}

		nextPhoneNumber, err := u.scheduleSvc.GetNextPhoneNumber(callData.Number)
		if err != nil {
			return fmt.Errorf("failed to get next phone number: %w", err)
		}
		callData.Number = nextPhoneNumber
	}

	return fmt.Errorf("maximum attempts reached")
}

func (u *alertUseCase) MarkCallSuccessful(phoneNumber string) error {
	return u.scheduleSvc.MarkCallSuccessful(phoneNumber)
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
