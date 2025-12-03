package http

import (
	"encoding/json"
	"net/http"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/vo"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/in"
	smsErr "github.com/EthanQC/IM/services/identity_service/pkg/errors"
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

type sendCodeRequest struct {
	Phone string `json:"phone"`
	IP    string `json:"ip"`
}

func (h *SMSHandler) sendCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req sendCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request"})
		return
	}
	phoneVO, err := vo.NewPhone(req.Phone)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{smsErr.ErrInvalidCode.Error()})
		return
	}
	if err := h.smsUC.SendSMSCode(ctx, *phoneVO, req.IP); err != nil {
		// 可以根据 smsErr.ErrTooManyAttempts 等进一步区分
		writeJSON(w, http.StatusInternalServerError, errorResponse{err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
