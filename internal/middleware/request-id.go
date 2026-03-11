package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

// Ключ для context.WithValue должен быть уникальным типом, чтобы избежать коллизий между пакетами.
type ctxKeyRequestID struct{}

// RequestIDMiddleware добавляет/пробрасывает X-Request-ID.
// 1) Если клиент прислал X-Request-ID -- используем его.
// 2) Иначе -- генерируем.
// 3) Кладём в context и возвращаем в заголовке ответа.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if reqID == "" {
			reqID = newRequestID()
		}

		w.Header().Set("X-Request-ID", reqID)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID{}, reqID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID возвращает request-id из контекста (если был установлен middleware).
func GetRequestID(ctx context.Context) string {
	v := ctx.Value(ctxKeyRequestID{})
	s, _ := v.(string)
	return s
}

// Простой генератор request-id без внешних зависимостей (crypto/rand + hex).
func newRequestID() string {
	var b [16]byte
	_, err := rand.Read(b[:])
	if err != nil {
		// крайне редкий случай; fallback -- "нулевой" id
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b[:])
}
