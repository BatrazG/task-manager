// Package middleware содержит HTTP‑middleware: функции-обёртки над http.Handler,
// которые добавляют общий функционал (логирование, авторизация, заголовки)
// вокруг основного обработчика без изменения его кода.
package middleware

import (
	"log"
	"net/http"
	"time"
)

// LoggingMiddleware измеряет время обработки запроса и пишет запись в лог
// после того, как основной обработчик завершил работу.
//
// Важно: логирование идёт "после" next.ServeHTTP, поэтому в duration входит
// вся обработка запроса обработчиком и другими middleware внутри цепочки.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now() // фиксируем момент начала обработки

		// Оборачиваем ResponseWriter, чтобы узнать status code и bytes (готовность к эксплуатации)
		rec := newStatusRecorder(w)
		next.ServeHTTP(rec, r) // передаём управление следующему обработчику

		// request-id добавляет связность "клиент ↔ логи"
		reqID := GetRequestID(r.Context())

		log.Printf("request_id=%s method=%s path=%s status=%d bytes=%d server_in=%v", reqID, r.Method, r.URL.Path, rec.status, rec.bytes, time.Since(start))
	})
}

// JSONHeaderMiddleware проставляет заголовок Content-Type для JSON‑ответов.
//
// Это удобно, когда обработчики всегда возвращают JSON.
// Важно: заголовки нужно выставлять ДО записи тела ответа (до w.Write / Encode).
func JSONHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r) // дальше обработчик пишет JSON-тело
	})
}
