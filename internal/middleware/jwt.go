package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// Создаем уникальный тип для ключа контекста (стандарт Go)
type contextKey string

// Константа, по которой мы (и хендлеры) будем доставать ID пользователя
const UserIDKey contextKey = "user_id"

const prefix string = "Bearer "

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, prefix) || len(authHeader) < 7 {
			WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid token", nil)
			return
		}
		tokenString := strings.TrimPrefix(authHeader, prefix)
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if err != nil || !token.Valid {
			WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid token", nil)
			return
		}

		// 1. Приводим claims к типу jwt.MapClaims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			// Если по какой-то причине claims не того типа, прерываем запрос
			WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Invalid token claims", nil)
			return
		}

		// 2. Безопасно достаем user_id
		userIDFloat, ok := claims["user_id"].(float64) // Сначала приводим строго к float64!
		if !ok {
			WriteError(w, r, http.StatusUnauthorized, "unauthorized", "User ID not found in token", nil)
			return
		}

		// 3. Превращаем float64 в привычный int
		userID := int(userIDFloat)

		// 1. Берем текущий контекст, который прилетел вместе с запросом r
		ctx := r.Context()

		// 2. Создаем на его основе новый контекст, положив туда пару Ключ -> Значение
		// Для ключа мы используем специальную константу UserIDKey
		newCtx := context.WithValue(ctx, UserIDKey, userID)

		// 3. Пробрасываем запрос дальше, обернув его в этот новый контекст
		next.ServeHTTP(w, r.WithContext(newCtx))

	})
}
