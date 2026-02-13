package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Модель задачи
type Task struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

// Наша "база данных" в памяти
var tasks = []Task{
	{ID: 1, Title: "Learn Go routing", Done: false},
	{ID: 2, Title: "Dinner", Done: true},
}

// Простой счетчик для ID (в реальности это делает БД)
var nextID = 3

// AdminOnly — middleware (Advanced): проверяет доступ по заголовку.
// Пункт задания: "Написать свой middleware AdminOnly... X-Admin-Key: secret123... иначе 403".
// Как работает: оборачивает handler; при неверном ключе отвечает 403 и не передаёт управление дальше.
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Admin-Key") != "secret123" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GET / (список)
func getAllTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tasks)
}

// GET /{id} (получение одной задачи) — Advanced
func getTaskByID(w http.ResponseWriter, r *http.Request) {
	// Исправление в рамках рефакторинга роутинга: chi.URLParam(r, "id") читается из шаблона "/{id}".
	// Раньше роут был задан некорректно (двойной "/{id}" в Route + Get), из-за чего параметр мог не матчиться.
	idStr := chi.URLParam(r, "id")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	for _, t := range tasks {
		if t.ID == id {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(t)
			return
		}
	}

	http.Error(w, "Task not found", http.StatusNotFound)
}

// POST / (создание)
func createTask(w http.ResponseWriter, r *http.Request) {
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// ID формируется примитивно (in-memory хранилище), как требуется по условию.
	task.ID = nextID
	nextID++

	tasks = append(tasks, task)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(task)
}

// PUT /{id} (обновление) — возвращаем PUT
func updateTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var incoming Task
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for i, t := range tasks {
		if t.ID == id {
			// Обновляем поля (ID фиксируем по URL, даже если в body пришёл другой)
			tasks[i].Title = incoming.Title
			tasks[i].Done = incoming.Done

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(tasks[i])
			return
		}
	}

	http.Error(w, "Task not found", http.StatusNotFound)
}

// DELETE /{id} (удаление) — с AdminOnly (Advanced)
func deleteTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	for i, task := range tasks {
		if task.ID == id {
			tasks = append(tasks[:i], tasks[i+1:]...)
			w.WriteHeader(http.StatusNoContent) // 204
			return
		}
	}
	http.Error(w, "Task not found", http.StatusNotFound)
}

func main() {
	r := chi.NewRouter()

	// MVP: middleware.Logger и middleware.Recoverer.
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Исправление по пункту задания: группировка под префиксом /api/v1/tasks.
	// Раньше было "/tasks", что не соответствует требованиям тикета.
	r.Route("/api/v1/tasks", func(r chi.Router) {
		// MVP: GET / (список), POST / (создание)
		r.Get("/", getAllTasks)
		r.Post("/", createTask)

		// Advanced: GET /{id}
		r.Get("/{id}", getTaskByID)

		// PUT: обновление
		r.Put("/{id}", updateTask)

		// MVP: DELETE /{id}; Advanced: применяем AdminOnly только к DELETE.
		r.With(AdminOnly).Delete("/{id}", deleteTask)
	})

	fmt.Println("Server running on port :8080")

	// Исправление: раньше использовалась переменная mux, которой не существует, из-за чего код не компилируется.
	// Правильно: передаём chi-router r в ListenAndServe.
	if err := http.ListenAndServe(":8080", r); err != nil {
		fmt.Printf("Server start error: %v\n", err)
	}
}
