package http

import (
	"encoding/json"
	"net/http"

	"github.com/EthanQC/IM/services/auth-service/internal/domain/vo"
	"github.com/EthanQC/IM/services/auth-service/internal/ports/in"
)

type AuthHandler struct {
	authUC in.AuthUseCase
}

func NewAuthHandler(authUC in.AuthUseCase) *AuthHandler {
	return &AuthHandler{authUC: authUC}
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/login/password", h.loginByPassword)
	mux.HandleFunc("/login/sms", h.loginBySMS)
	mux.HandleFunc("/token/refresh", h.refreshToken)
	mux.HandleFunc("/logout", h.logout)
}

func (h *AuthHandler) loginByPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	pw, err := vo.NewPassword(req.Password)
	if err != nil {
		http.Error(w, "invalid password format", http.StatusBadRequest)
		return
	}
	at, err := h.authUC.LoginByPassword(ctx, req.Identifier, *pw)
	if err != nil {
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(at)
}

func (h *AuthHandler) loginBySMS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
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
	at, err := h.authUC.LoginBySMS(ctx, *phoneVO, req.Code)
	if err != nil {
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(at)
}

func (h *AuthHandler) refreshToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		RefreshJTI string `json:"refresh_jti"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	at, err := h.authUC.RefreshToken(ctx, req.RefreshJTI)
	if err != nil {
		http.Error(w, "refresh failed", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(at)
}

func (h *AuthHandler) logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		AccessJTI string `json:"access_jti"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := h.authUC.Logout(ctx, req.AccessJTI); err != nil {
		http.Error(w, "logout failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
