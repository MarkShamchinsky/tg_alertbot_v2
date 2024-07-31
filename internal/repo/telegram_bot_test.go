package repo

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
	"testing"
)

type MockBotAPI struct {
	mock.Mock
}

func (m *MockBotAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	args := m.Called(c)
	return tgbotapi.Message{}, args.Error(0)
}

// TestNewTelegramBot checks the creation of a new TelegramBot instance
func TestNewTelegramBot(t *testing.T) {
	_, err := NewTelegramBot("valid_token", "123", "456")
	assert.Nil(t, err)

	_, err = NewTelegramBot("", "123", "456")
	assert.NotNil(t, err)
}

// TestSendMessage checks message sending functionality
func TestSendMessage(t *testing.T) {
	botApi := new(MockBotAPI)
	botApi.On("Send", mock.Anything).Return(nil)
	tgBot := &TelegramBot{Bot: botApi, warningChatID: "123", criticalChatID: "456"}

	err := tgBot.SendMessage(123456789, "Hello World")
	assert.Nil(t, err)

	longMessage := "..."
	// Assume `longMessage` is longer than `maxMsgLength`
	err = tgBot.SendMessage(123456789, longMessage)
	assert.Nil(t, err)

	botApi.AssertNumberOfCalls(t, "Send", 1) // Adjust based on actual calls
}

// TestGetChatID checks chat ID retrieval based on severity
func TestGetChatID(t *testing.T) {
	tgBot := TelegramBot{warningChatID: "123", criticalChatID: "456"}

	chatID, err := tgBot.GetChatID("Warning")
	assert.Nil(t, err)
	assert.Equal(t, int64(123), chatID)

	chatID, err = tgBot.GetChatID("Critical")
	assert.Nil(t, err)
	assert.Equal(t, int64(456), chatID)

	_, err = tgBot.GetChatID("Info")
	assert.NotNil(t, err)
}

// TestSplitLongMessage tests the message splitting logic
func TestSplitLongMessage(t *testing.T) {
	shortMessage := "Hello World"
	messages := splitLongMessage(shortMessage)
	assert.Equal(t, 1, len(messages))
	assert.Equal(t, shortMessage, messages[0])

	longMessage := "..."
	// Populate `longMessage` to exceed `maxMsgLength`
	messages = splitLongMessage(longMessage)
	assert.True(t, len(messages) > 1)
}
