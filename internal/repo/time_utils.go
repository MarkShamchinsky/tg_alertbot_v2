package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// Schedule описывает структуру расписания
type Schedule struct {
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	PhoneNumber string `json:"phone_number"`
}

// ScheduleRepository определяет интерфейс для работы с хранилищем расписаний
type ScheduleRepository interface {
	Load() ([]Schedule, error)
	Save(schedules []Schedule) error
}

// FileScheduleRepository реализует интерфейс ScheduleRepository для работы с файлами
type FileScheduleRepository struct {
	filePath string
}

// NewFileScheduleRepository создает новый экземпляр FileScheduleRepository
func NewFileScheduleRepository(filePath string) *FileScheduleRepository {
	// Проверка на существование файла, если его нет - создаем
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		_, err := os.Create(filePath)
		if err != nil {
			fmt.Printf("Error creating file: %v\n", err)
		}
	}
	return &FileScheduleRepository{filePath: filePath}
}

// Load загружает расписание из JSON-файла
func (r *FileScheduleRepository) Load() ([]Schedule, error) {
	file, err := os.Open(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Schedule{}, nil // Если файл не существует, возвращаем пустое расписание
		}
		return nil, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var schedules []Schedule
	err = json.Unmarshal(bytes, &schedules)
	if err != nil {
		return nil, err
	}
	fmt.Println(schedules)
	return schedules, nil
}

// Save сохраняет расписание в JSON-файл
func (r *FileScheduleRepository) Save(schedules []Schedule) error {
	bytes, err := json.MarshalIndent(schedules, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(r.filePath, bytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

// ScheduleService предоставляет методы для управления расписаниями
type ScheduleService struct {
	repo ScheduleRepository
}

// SchedulerService определяет методы для работы с расписаниями
type SchedulerService interface {
	AddSchedule(startTime, endTime, phoneNumber string) error
	GetPhoneNumberByTime() (string, error)
}

// NewScheduleService создает новый экземпляр ScheduleService
func NewScheduleService(repo ScheduleRepository) *ScheduleService {
	return &ScheduleService{repo: repo}
}

// AddSchedule добавляет новое расписание
func (s *ScheduleService) AddSchedule(startTime, endTime, phoneNumber string) error {
	if err := validateTimeFormat(startTime); err != nil {
		return err
	}

	if err := validateTimeFormat(endTime); err != nil {
		return err
	}

	schedules, err := s.repo.Load()
	if err != nil {
		return err
	}

	newSchedule := Schedule{
		StartTime:   startTime,
		EndTime:     endTime,
		PhoneNumber: phoneNumber,
	}

	schedules = append(schedules, newSchedule)
	return s.repo.Save(schedules)
}

// GetPhoneNumberByTime возвращает номер телефона по текущему времени
func (s *ScheduleService) GetPhoneNumberByTime() (string, error) {
	schedules, err := s.repo.Load()
	if err != nil {
		return "", err
	}

	currentTime := time.Now().Format("15:04")

	for _, schedule := range schedules {
		startTime, _ := time.Parse("15:04", schedule.StartTime)
		endTime, _ := time.Parse("15:04", schedule.EndTime)
		currentParsedTime, _ := time.Parse("15:04", currentTime)

		if currentParsedTime.After(startTime) && currentParsedTime.Before(endTime) {
			return schedule.PhoneNumber, nil
		}
	}

	return "", errors.New("no phone number found for the current time")
}

// validateTimeFormat проверяет правильность формата времени
func validateTimeFormat(timeStr string) error {
	_, err := time.Parse("15:04", timeStr)
	if err != nil {
		return errors.New("invalid time format, use HH:MM")
	}
	return nil
}
