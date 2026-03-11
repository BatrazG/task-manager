package middleware

import "net/http"

// BodyLimitMiddleware ограничивает размер тела запроса.
// Это базовая защита от случайных/злонамеренных больших payload.
func BodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// MaxBytesReader вернёт *http.MaxBytesError при превышении лимита
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
