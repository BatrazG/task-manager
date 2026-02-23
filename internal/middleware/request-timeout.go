// [CHANGE-CONTEXT] В отдельном файле сделаем таймаут на обработку запроса
// Делаем так подробно, чтобы вы глубже разобрались с темой
// И поняли, почему и как контекст должен проникать через слои
package middleware

import (
	"context"
	"net/http"
	"time"
)

// RequestTimeoutMiddleware выставляет таймаут на обработку запроса через context.WithTimeout.
//
// Важно: это НЕ "магический убийца" хендлеров.
// Таймаут сработает только если нижние слои реально проверяют ctx.Done()/ctx.Err().
func requestTimeoutMiddleware(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
