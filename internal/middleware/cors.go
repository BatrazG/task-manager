package middleware

import (
	"net/http"

	"github.com/rs/cors"
)

// NewCORSMiddleware собирает настроенный фильтр CORS для защиты браузерных запросов.
func NewCORSMiddleware() func(http.Handler) http.Handler {
	return cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // Разрешаем запросы отовсюду на этапе разработки
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Request-ID"},
		ExposedHeaders:   []string{"Link", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300, // Кэшировать preflight-ответ на 5 минут
	}).Handler
}
