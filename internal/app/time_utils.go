package app

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

// Schedule Структура для хранения расписания
type Schedule struct {
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	PhoneNumber string `json:"phone_number"`
}

var scheduleFile = "schedule.json"
var schedules []Schedule

// LoadSchedules Функция для загрузки расписания из JSON-файла
func LoadSchedules() error {
	file, err := os.Open(scheduleFile)
	if err != nil {
		if os.IsNotExist(err) {
			return SaveSchedules() // Если файл не существует, создаем его
		}
		return err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	err = json.Unmarshal(bytes, &schedules)
	if err != nil {
		return err
	}

	return nil
}

// SaveSchedules Функция для сохранения расписания в JSON-файл
func SaveSchedules() error {
	bytes, err := json.MarshalIndent(schedules, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(scheduleFile, bytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

// AddSchedule Функция для добавления нового расписания через Telegram-бот
func AddSchedule(startTime, endTime, phoneNumber string) error {
	_, err := time.Parse("15:04", startTime)
	if err != nil {
		return errors.New("invalid start time format, use HH:MM")
	}

	_, err = time.Parse("15:04", endTime)
	if err != nil {
		return errors.New("invalid end time format, use HH:MM")
	}

	newSchedule := Schedule{
		StartTime:   startTime,
		EndTime:     endTime,
		PhoneNumber: phoneNumber,
	}

	schedules = append(schedules, newSchedule)
	return SaveSchedules()
}

// GetPhoneNumberByTime Функция для получения номера телефона по текущему времени
func GetPhoneNumberByTime() string {
	currentTime := time.Now().In(time.FixedZone("MSK", 3*60*60)).Format("15:04")

	for _, schedule := range schedules {
		startTime, _ := time.Parse("15:04", schedule.StartTime)
		endTime, _ := time.Parse("15:04", schedule.EndTime)

		currentParsedTime, _ := time.Parse("15:04", currentTime)

		if currentParsedTime.After(startTime) && currentParsedTime.Before(endTime) {
			return schedule.PhoneNumber
		}
	}

	return "" // Номер по умолчанию (если не найдено соответствие)
}
