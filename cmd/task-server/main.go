package main

import (
	"fmt"
	"net/http"

	"task-manager/internal/middleware" // [CHANGE] подключаем наш пакет middleware
	"task-manager/internal/tasks"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware" // [CHANGE] алиас, чтобы не конфликтовать с internal/middleware
)

// Здесь только:
// - создание зависимостей;
// - настройка middleware;
// - запуск HTTP-сервера.
func main() {
	// Инициализируем файловое хранилище.
	store := tasks.NewTaskStore("tasks.json")

	// Инициализируем HTTP-обработчики задач.
	handler := tasks.NewHandler(store)

	// Собираем роутер.
	// Роуты переехали в internal/tasks (HTTP-слой), main только подключает.
	r := chiWithMiddleware(handler.Router())

	fmt.Println("Server running on port :8080")

	if err := http.ListenAndServe(":8080", r); err != nil {
		fmt.Printf("Server start error: %v\n", err)
	}
}

// chiWithMiddleware навешивает базовые middleware на уже собранный роутер.
//
//	Вынесено в отдельную функцию, чтобы main был читаемым и "про запуск".
func chiWithMiddleware(h http.Handler) http.Handler {
	// Используем chi.Router, чтобы навесить middleware, не меняя роуты модуля.
	// Это позволяет internal/tasks оставаться независимым от общесервисных middleware.
	r := chi.NewRouter()

	// middleware.Logger и middleware.Recoverer.
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)

	// [CHANGE] подключаем кастомный логгер на весь сервис
	r.Use(middleware.LoggingMiddleware)

	r.Mount("/", h)
	return r
}
