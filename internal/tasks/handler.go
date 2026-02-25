// [CHANGE-CONTEXT] Модель задач вынесли в отдельный файл task.go
// Handler — HTTP-слой модуля задач.
package tasks

import (
	// [CHANGE-CONTEXT]
	"context"
	"encoding/json" // [CHANGE-CONTEXT]
	"errors"
	"net/http"
	"strconv"
	"time" // [CHANGE-CONTEXT]

	// "task-manager/internal/middleware"
	appMiddleware "task-manager/internal/middleware" // подключаем middleware-пакет (алиас, чтобы не путать с chi/middleware)

	"github.com/go-chi/chi/v5"
)

// Теперь Handler - HTTP слой модуля задач
//
// Здесь лежит всё, что относится к HTTP:
// роуты, парсинг JSON, выставление заголовков, коды ответов, middleware.
//
// [CHANGE-CONTEXT] Состояние и бизнес-логика живут в Service, чтобы была цепочка:
// handler -> service -> store.
type Handler struct {
	svc *Service // [CHANGE-CONTEXT]
}

// NewHandler создаёт Handler и загружает данные из хранилища.
//
// [CHANGE-CONTEXT] Загрузка данных переехала в Service (а не в Handler),
// чтобы демонстрировать "протекание" ctx при каждом запросе.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Router собирает HTTP-роутер для задач.
//
// Здесь размещаем всё связывание путей с обработчиками.
func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()

	r.Route("/api/v1/tasks", func(r chi.Router) {
		// JSONHeaderMiddleware вешаем на весь tasks API,
		// чтобы убрать дублирующиеся Content-Type из хендлеров.
		r.Use(appMiddleware.JSONHeaderMiddleware)

		// [CHANGE-CONTEXT] Таймаут на каждый запрос tasks API.
		// Для демо удобно держать небольшим, чтобы легко ловить DeadlineExceeded.
		r.Use(appMiddleware.RequestTimeoutMiddleware(2 * time.Second))

		// GET / (список), POST / (создание)
		r.Get("/", h.getAllTasks)
		r.Post("/", h.createTask)

		// GET /{id}
		r.Get("/{id}", h.getTaskByID)

		// PUT: обновление
		r.Put("/{id}", h.updateTask)

		r.With(appMiddleware.BasicAuthMiddleware).Delete("/{id}", h.deleteTask)
	})
	return r
}

// getAllTasks обрабатывает GET /api/v1/tasks/
//
// Возвращает полный список задач в JSON.
//
// [CHANGE-CONTEXT] Поддерживает демо медленного I/O: ?delay=2s (ParseDuration).
func (h *Handler) getAllTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context() // [CHANGE-CONTEXT]

	delay, err := parseDelayParam(r)
	if err != nil {
		http.Error(w, "Invalid delay. Use e.g. ?delay=200ms or ?delay=2s", http.StatusBadRequest)
		return
	}

	tasks, err := h.svc.ListTasks(ctx, delay)
	if err != nil {
		if h.handleContextError(w, err) {
			return
		}
		http.Error(w, "Failed to load tasks", http.StatusInternalServerError) // 500
		return
	}

	// Content-Type выставляет JSONHeaderMiddleware
	//w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tasks)
}

// createTask обрабатывает POST /api/v1/tasks/
//
// Создаёт задачу, выдаёт ID, сохраняет список на диск, возвращает созданную задачу.
func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context() // [CHANGE-CONTEXT]

	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Минимальная валидация.
	if task.Title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	// [CHANGE-CONTEXT]
	created, err := h.svc.CreateTask(ctx, task)
	if err != nil {
		if h.handleContextError(w, err) {
			return
		}
		http.Error(w, "Failed to save task", http.StatusInternalServerError)
		return
	}

	// Возвращаем JSON созданной задачи.
	// Content-Type выставляет JSONHeaderMiddleware
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

// getTaskByID обрабатывает GET /api/v1/tasks/{id}
//
// Находит задачу по ID и возвращает её.
func (h *Handler) getTaskByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context() // [CHANGE-CONTEXT]

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// [CHANGE-CONTEXT]
	task, ok, err := h.svc.GetTask(ctx, id)
	if err != nil {
		if h.handleContextError(w, err) {
			return
		}
		http.Error(w, "Failed to get task", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound) // 404
		return
	}

	// [CHANGE] Content-Type выставляет JSONHeaderMiddleware
	_ = json.NewEncoder(w).Encode(task)

}

// [CHANGE-CONTEXT]
// updateTask обрабатывает PUT /api/v1/tasks/{id}
//
// Обновляет Title/Done у задачи, сохраняет список на диск, возвращает обновлённую задачу.
func (h *Handler) updateTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context() // [CHANGE-CONTEXT]

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var incoming Task
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	updated, ok, err := h.svc.UpdateTask(ctx, id, incoming)
	if err != nil {
		if h.handleContextError(w, err) {
			return
		}
		http.Error(w, "Failed to save tasks", http.StatusInternalServerError)
		return
	}
	if !ok {
		// Если задача с запрашиваемым ID не найдена
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// [CHANGE] Content-Type выставляет JSONHeaderMiddleware
	_ = json.NewEncoder(w).Encode(updated)
}

// deleteTask обрабатывает DELETE /api/v1/tasks/{id}
//
// Удаляет задачу, сохраняет список на диск, возвращает 204.
func (h *Handler) deleteTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context() // [CHANGE-CONTEXT]

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ok, err := h.svc.DeleteTask(ctx, id)
	if err != nil {
		if h.handleContextError(w, err) {
			return
		}
		http.Error(w, "Failed to save tasks", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseDelayParam парсит query-параметр ?delay=...
//
// [CHANGE-CONTEXT] Нужен для демо отмены/таймаута.
// Например: ?delay=2s или ?delay=200ms.
func parseDelayParam(r *http.Request) (time.Duration, error) {
	raw := r.URL.Query().Get("delay")
	if raw == "" {
		return 0, nil
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, err
	}

	if d < 0 {
		return 0, errors.New("delay must be >= 0")
	}
	return d, nil
}

// handleContextError делает понятную обработку ошибок отмены/таймаута.
//
// [CHANGE-CONTEXT] Это важно в учебном коде: показываем, что ctx.Err() - нормальная причина остановки.
func (h *Handler) handleContextError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, context.Canceled):
		// Запрос отменён: клиент ушёл ИЛИ сервер делает graceful shutdown.
		// Часто отвечать уже некому (соединение закрыто), поэтому просто прекращаем работу.
		return true
	case errors.Is(err, context.DeadlineExceeded):
		// Таймаут запроса (например, наш RequestTimeoutMiddleware).
		http.Error(w, "Request timeout", http.StatusRequestTimeout) // 408
		return true
	default:
		return false
	}
}
