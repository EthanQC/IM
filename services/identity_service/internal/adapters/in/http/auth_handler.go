package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/vo"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/in"
	authErr "github.com/EthanQC/IM/services/identity_service/pkg/errors"
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

type authRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type smsLoginRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	AccessJTI string `json:"access_jti"`
}

type authResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *AuthHandler) loginByPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request"})
		return
	}
	at, err := h.authUC.LoginByPassword(ctx, req.Identifier, req.Password)
	if err != nil {
		status := mapAuthError(err)
		writeJSON(w, status, errorResponse{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, authResponse{at.AccessToken, at.RefreshToken})
}

func (h *AuthHandler) loginBySMS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req smsLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request"})
		return
	}
	phoneVO, err := vo.NewPhone(req.Phone)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{authErr.ErrInvalidPhone.Error()})
		return
	}
	at, err := h.authUC.LoginBySMS(ctx, *phoneVO, req.Code)
	if err != nil {
		status := mapAuthError(err)
		writeJSON(w, status, errorResponse{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, authResponse{at.AccessToken, at.RefreshToken})
}

func (h *AuthHandler) refreshToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request"})
		return
	}
	at, err := h.authUC.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, authResponse{at.AccessToken, at.RefreshToken})
}

func (h *AuthHandler) logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req logoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request"})
		return
	}
	if err := h.authUC.Logout(ctx, req.AccessJTI); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{"logout failed"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func mapAuthError(err error) int {
	switch {
	case errors.Is(err, authErr.ErrInvalidPassword),
		errors.Is(err, authErr.ErrInvalidToken):
		return http.StatusUnauthorized
	case errors.Is(err, authErr.ErrUserBlocked):
		return http.StatusForbidden
	default:
		return http.StatusBadRequest
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
