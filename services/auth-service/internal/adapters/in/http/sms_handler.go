package http

import (
	"encoding/json"
	"net/http"

	"github.com/EthanQC/IM/services/auth-service/internal/domain/vo"
	"github.com/EthanQC/IM/services/auth-service/internal/ports/in"
)

type SMSHandler struct {
	smsUC in.SMSUseCase
}

func NewSMSHandler(smsUC in.SMSUseCase) *SMSHandler {
	return &SMSHandler{smsUC: smsUC}
}

func (h *SMSHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/sms/send", h.sendCode)
}

func (h *SMSHandler) sendCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		Phone string `json:"phone"`
		IP    string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	phoneVO, err := vo.NewPhone(req.Phone)
	if err != nil {
		http.Error(w, "invalid phone number", http.StatusBadRequest)
		return
	}
	if err := h.smsUC.SendSMSCode(ctx, *phoneVO, req.IP); err != nil {
		http.Error(w, "send sms failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
