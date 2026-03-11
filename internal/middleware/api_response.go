package middleware

import (
	"encoding/json"
	"net/http"
)

// Единый формат ошибки -- часть HTTP контракта (важно для FE/QA/интеграции).
type ErrorResponse struct {
	Error APIError `json:"api_error"`
}

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Details   any    `json:"details,omitempty"`
}

// WriteJSON пишет JSON-ответ и выставляет статус.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	// Выставляем Content-Type централизованно, чтобы не зависеть от "вешали ли middleware".
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// WriteError пишет ошибку в едином формате + добавляет request_id.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, details any) {
	resp := ErrorResponse{
		Error: APIError{
			Code:      code,
			Message:   message,
			RequestID: GetRequestID(r.Context()),
			Details:   details,
		},
	}
	WriteJSON(w, status, resp)
}
