package ctrl

import (
	"AlertManagerBot/internal/app"
	ent "AlertManagerBot/internal/entity"
	"encoding/json"
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
	var msg ent.AlertManagerMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.alertUseCase.SendAlerts(msg.Alerts)
	w.WriteHeader(http.StatusOK)
}

func (h *AlertController) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	var webhookData struct {
		To        string `json:"to"`
		HookEvent string `json:"hook_event"`
	}

	if err := json.NewDecoder(r.Body).Decode(&webhookData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if webhookData.HookEvent == "call_success" {
		err := h.alertUseCase.MarkCallSuccessful(webhookData.To)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// обработка других событий
	}

	w.WriteHeader(http.StatusOK)
}
