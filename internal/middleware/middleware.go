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

// BasicAuthMiddleware защищает эндпоинт HTTP Basic Auth.
//
// r.BasicAuth() парсит заголовок Authorization и возвращает (username, password, ok).
// Если аутентификация не пройдена, middleware:
// 1) выставляет WWW-Authenticate (чтобы браузер/клиент понял, что нужен логин/пароль)
// 2) возвращает 401 Unauthorized и НЕ вызывает next.
func BasicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, pass, ok := r.BasicAuth()
		if !ok || name != "admin" || pass != "secret" {
			// realm — "зона" аутентификации, отображается клиентам (например, в браузере).
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			// http.Error(w, "Unauthorized", http.StatusUnauthorized) // 401
			// Единый JSON-формат ошибок даже для middleware (консистентный HTTP контракт)
			WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Unauthorized", nil)
			return
		}
		next.ServeHTTP(w, r) // доступ разрешён — продолжаем цепочку
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
