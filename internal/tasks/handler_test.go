package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// [Setup функции: Подготавливаем полностью рабочую копию модуля HTTP]
// t.TempDir() автоматически создаст и УДАЛИТ временную папку после выполнения тестов.
// Это гарантия того, что наши тесты не сломают рабочую базу данных (tasks.json).
func setupTestRouter(t *testing.T) http.Handler {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test_db.jsn")

	store := NewTaskStore(tempFile)

	// Используем context.Background(), так как отмена в тестах нам пока не нужна
	svc, err := NewService(context.Background(), store)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	handler := NewHandler(svc)
	return handler.Router()
}

// [Table Driven Tests: Одна функция покрывает сразу множество сценариев]
func TestCreateTask(t *testing.T) {
	router := setupTestRouter(t)

	// Структура, описывающий один текстовый случай (кейс)
	type testCase struct {
		name         string
		payload      string
		expectedCode int
	}

	// Массив кейсов: легко читать, легко добавлять новые проверки
	tests := []testCase{
		{
			name:         "Success valid JSON",
			payload:      `{"title": "Write Unit Tests", "priority": "high", "done": false}`,
			expectedCode: http.StatusCreated, // 201
		},
		{
			name:         "Bad Request invalid JSON",
			payload:      `{"title": "Forgot closing bracket, oops`,
			expectedCode: http.StatusBadRequest, // 400
		},
		{
			name: "Validation Error bad priority",
			// priority должно быть low, medium или high согласно нашей модели DTO
			payload:      `{"title": "Play games", "priority": "unknown", "done": false}`,
			expectedCode: http.StatusBadRequest, // 400
		},
	}

	// [Цикл прогона: "Бежим" по таблице тестов]
	for _, tt := range tests {
		// t.Run инициализирует под-тест, это красиво выглядит в логах go test -v
		t.Run(tt.name, func(t *testing.T) {
			// 1. Создаем "фейковый" запрос (имитация отправки из Postman/cURL)
			reqBody := bytes.NewBufferString(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", reqBody)
			req.Header.Set("Content-Type", "application/json") // Без этого валидатор или парсер может ругаться

			// 2. Создаем фейковый ответ (сюда роутер запишет результат)
			rec := httptest.NewRecorder()

			// 3. Дергаем ручку (передаем запрос в наш роутер/хендлер)
			// Сервер физически не поднимается! (Listener не ждет TCP), все происходит в памяти!
			router.ServeHTTP(rec, req)

			// 4. Проверяем статус-код
			if rec.Code != tt.expectedCode {
				t.Errorf("Expected status: %d, gotL %d. Response: %s", tt.expectedCode, rec.Code, rec.Body.String())
			}

			// Проверяем содержимое JSON при успехе
			// Если мы ожидали 201 Успех, нужно убедиться, что БД выдала нам ID новой задачи
			if tt.expectedCode == http.StatusCreated {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				if err != nil {
					t.Fatalf("Failed to decode response JSON: %v", err)
				}

				// Проверяем, существует ли поле "id" в ответе
				if _, ok := response["id"]; !ok {
					t.Errorf("Expected 'id' in JSON response, but didn't find it. Got: %v", response)
				}
			}
		})
	}
}
