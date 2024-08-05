package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
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
	log.Println("Loading schedule from file:", r.filePath)

	file, err := os.Open(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("File does not exist, returning empty schedule")
			return []Schedule{}, nil // Если файл не существует, возвращаем пустое расписание
		}
		log.Println("Error opening file:", err)
		return nil, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

			log.Println("Error closing file:", err)
		}
	}(file)

	log.Println("Reading file contents")
	bytes, err := io.ReadAll(file)
	if err != nil {

		log.Println("Error reading file:", err)
		return nil, err
	}

	log.Println("Unmarshaling JSON")
	var schedules []Schedule
	err = json.Unmarshal(bytes, &schedules)
	if err != nil {

		log.Println("Error unmarshaling JSON:", err)
		return nil, err
	}
	log.Println("Schedule loaded:", schedules)
	return schedules, nil
}

// Save сохраняет расписание в JSON-файл
func (r *FileScheduleRepository) Save(schedules []Schedule) error {
	bytes, err := json.MarshalIndent(schedules, "", "  ")
	if err != nil {
		log.Printf("Error marshaling schedules: %v", err)
		return err
	}

	log.Printf("Saving schedules to file: %s", r.filePath)
	err = os.WriteFile(r.filePath, bytes, 0644)
	if err != nil {
		log.Printf("Error writing schedules to file: %v", err)
		return err
	}

	log.Printf("Schedules saved successfully")
	return nil
}

// ScheduleService предоставляет методы для управления расписаниями
type ScheduleService struct {
	repo            ScheduleRepository
	callAttempts    map[string]int
	successfulCalls map[string]time.Time
	muteUntil       time.Time
}

func (s *ScheduleService) IsMuted() bool {
	return time.Now().Before(s.muteUntil)
}

// SchedulerService определяет методы для работы с расписаниями
type SchedulerService interface {
	AddSchedule(startTime, endTime, phoneNumber string) error
	GetPhoneNumberByTime() (string, error)
	MarkCallSuccessful(phoneNumber string) error
	GetNextPhoneNumber(currentPhoneNumber string) (string, error)
}

var _ SchedulerService = (*ScheduleService)(nil)

// NewScheduleService создает новый экземпляр ScheduleService
func NewScheduleService(repo ScheduleRepository) *ScheduleService {
	return &ScheduleService{repo: repo, callAttempts: make(map[string]int), successfulCalls: make(map[string]time.Time)}
}

// AddSchedule добавляет новое расписание
func (s *ScheduleService) AddSchedule(startTime, endTime, phoneNumber string) error {
	log.Printf("Adding schedule for %s - %s: %s", startTime, endTime, phoneNumber)

	if err := validateTimeFormat(startTime); err != nil {
		log.Printf("Failed to validate start time: %s", err)
		return err
	}

	if err := validateTimeFormat(endTime); err != nil {
		log.Printf("Failed to validate end time: %s", err)
		return err
	}

	schedules, err := s.repo.Load()
	if err != nil {
		log.Printf("Failed to load schedules: %s", err)
		return err
	}

	newSchedule := Schedule{
		StartTime:   startTime,
		EndTime:     endTime,
		PhoneNumber: phoneNumber,
	}

	schedules = append(schedules, newSchedule)

	if err := s.repo.Save(schedules); err != nil {
		log.Printf("Failed to save schedules: %s", err)
		return err
	}

	log.Printf("Schedule added successfully!")
	return nil
}

// GetPhoneNumberByTime returns the phone number based on the current time
func (s *ScheduleService) GetPhoneNumberByTime() (string, error) {
	log.Println("Getting phone number by time...")

	schedules, err := s.repo.Load()
	if err != nil {
		log.Printf("Error loading schedules: %v", err)
		return "", err
	}

	currentTime := time.Now().Format("15:04")
	log.Printf("Current time: %s", currentTime)

	for i := 0; i < len(schedules); i++ {
		schedule := schedules[i%len(schedules)] // loop through the schedule list again
		startTime, _ := time.Parse("15:04", schedule.StartTime)
		endTime, _ := time.Parse("15:04", schedule.EndTime)
		currentParsedTime, _ := time.Parse("15:04", currentTime)

		log.Printf("Checking schedule: %s - %s", schedule.StartTime, schedule.EndTime)

		if currentParsedTime.After(startTime) && currentParsedTime.Before(endTime) {

			// Check if the call to this number was successful in the last hour
			if lastCallTime, ok := s.successfulCalls[schedule.PhoneNumber]; ok && time.Since(lastCallTime) < time.Hour {
				log.Printf("Skipping schedule for successful call within the last hour: %s", schedule.PhoneNumber)
				continue
			}
			log.Printf("Phone number found: %s", schedule.PhoneNumber)
			return schedule.PhoneNumber, nil
		}
	}

	log.Println("No phone number found for the current time")
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

func (s *ScheduleService) MarkCallSuccessful(phoneNumber string) error {
	log.Printf("Marking call successful for phone number: %s", phoneNumber)
	s.successfulCalls[phoneNumber] = time.Now()
	log.Printf("Call marked successful at time: %s", time.Now().Format("2006-01-02 15:04:05"))
	delete(s.callAttempts, phoneNumber)
	log.Printf("Call attempts for phone number %s deleted", phoneNumber)
	return nil
}

// GetNextPhoneNumber возвращает следующий номер из расписания
func (s *ScheduleService) GetNextPhoneNumber(currentPhoneNumber string) (string, error) {
	schedules, err := s.repo.Load()
	if err != nil {
		return "", err
	}

	for i, schedule := range schedules {
		if schedule.PhoneNumber == currentPhoneNumber && i+1 < len(schedules) {
			return schedules[i+1].PhoneNumber, nil
		}
	}

	return "", errors.New("no more numbers to call")
}
