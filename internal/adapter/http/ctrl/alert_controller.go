package ctrl

import (
	"AlertManagerBot/internal/app"
	ent "AlertManagerBot/internal/entity"
	"encoding/json"
	"log"
	"net/http"
)

type AlertController struct {
	alertUseCase app.AlertSender
}

func NewAlertController(u app.AlertSender) *AlertController {
	return &AlertController{
		alertUseCase: u,
	}
}

func (h *AlertController) HandleAlert(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling Alert request")

	var msg ent.AlertManagerMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		log.Printf("Error decoding JSON: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Decoded message: %v", msg)

	h.alertUseCase.SendAlerts(msg.Alerts)

	log.Println("Alerts sent successfully")

	w.WriteHeader(http.StatusOK)
}

func (h *AlertController) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Любой поступивший webhook считается успешным
	log.Println("Webhook received, marking call as successful")

	// Вызов метода для пометки звонка как успешного
	err := h.alertUseCase.MarkCallSuccessful("webhook")
	if err != nil {
		log.Printf("Error marking call successful: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println("Call marked successful")
	w.WriteHeader(http.StatusOK)
}
