// Handler -- HTTP-слой модуля задач.
package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	appMiddleware "task-manager/internal/middleware" // подключаем middleware-пакет (алиас, чтобы не путать с chi/middleware)

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

// Теперь Handler - HTTP слой модуля задач
//
// Здесь лежит всё, что относится к HTTP:
// роуты, парсинг JSON, выставление заголовков, коды ответов, middleware.
//
// Состояние и бизнес-логика живут в Service, чтобы была цепочка:
// handler -> service -> store.
type Handler struct {
	svc      *Service
	validate *validator.Validate
}

// NewHandler создаёт Handler и загружает данные из хранилища.
func NewHandler(svc *Service) *Handler {
	return &Handler{
		svc:      svc,
		validate: validator.New(),
	}
}

// Router собирает HTTP-роутер для задач.
//
// Здесь размещаем всё связывание путей с обработчиками.
func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()

	// NEWединый JSON контракт -- выставляем Content-Type на весь router, включая 404/405.
	r.Use(appMiddleware.JSONHeaderMiddleware)

	// NEW404/405 тоже часть контракта; возвращаем в едином JSON-формате.
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		appMiddleware.WriteError(w, req, http.StatusNotFound, "not_found", "Route not found",
			map[string]any{"path": req.URL.Path})
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		appMiddleware.WriteError(w, req, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed",
			map[string]any{"method": req.Method, "path": req.URL.Path})
	})

	r.Route("/api/v1/tasks", func(r chi.Router) {
		// JSONHeaderMiddleware вешаем на весь tasks API,
		// чтобы убрать дублирующиеся Content-Type из хендлеров.
		// r.Use(appMiddleware.JSONHeaderMiddleware)

		// Таймаут на каждый запрос tasks API.
		// Для демо удобно держать небольшим, чтобы легко ловить DeadlineExceeded.
		r.Use(appMiddleware.RequestTimeoutMiddleware(2 * time.Second))

		// NEW-TEACH: базовая защита от слишком больших request body (устойчивость сервиса).
		r.Use(appMiddleware.BodyLimitMiddleware(1 << 20)) // 1 MiB

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
// Поддерживает демо медленного I/O: ?delay=2s (ParseDuration).
func (h *Handler) getAllTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context() // [CHANGE-CONTEXT]

	delay, err := parseDelayParam(r)
	if err != nil {
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "bad_request", "Invalid delay. Use e.g. ?delay=200ms or ?delay=2s",
			map[string]any{"error": err.Error()}) // NEW-TEACH: единый формат ошибок
		return
	}

	tasks, err := h.svc.ListTasks(ctx, delay)
	if err != nil {
		if h.handleContextError(w, r, err) {
			return
		}
		// внутреннюю причину логируем, клиенту отдаём стабильное сообщение
		log.Printf("request_id=%s getAllTasks error: %v", appMiddleware.GetRequestID(ctx), err)
		appMiddleware.WriteError(w, r, http.StatusInternalServerError, "internal", "Failed to load tasks", nil)
		return
	}

	_ = json.NewEncoder(w).Encode(tasks)
}

// createTask обрабатывает POST /api/v1/tasks/
//
// Создаёт задачу, выдаёт ID, сохраняет список на диск, возвращает созданную задачу.
func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context() // [CHANGE-CONTEXT]

	var req CreateTaskRequest // [Валидация] входящие данные читаем в DTO, чтобы валидировать контракт запроса
	if err := decodeJSONStrict(r, &req); err != nil {
		h.writeDecodeError(w, r, err) // NEW-TEACH: аккуратно маппим ошибки JSON/лимитов в HTTP
		return
	}

	if err := h.validate.Struct(req); err != nil { // [Валидация] Fail Fast: не пускаем невалидные данные в Service/Storage
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "validation_error", "Validation failed",
			validationDetails(err)) // NEW-TEACH: стабильный details вместо "сырого" текста
		return
	}

	// Валидация
	incoming := Task{
		Title:    req.Title,
		Done:     req.Done,
		Priority: req.Priority,
	}

	// CONTEXT
	created, err := h.svc.CreateTask(ctx, incoming)
	if err != nil {
		if h.handleContextError(w, r, err) {
			return
		}
		log.Printf("request_id=%s createTask error: %v", appMiddleware.GetRequestID(ctx), err) // NEW-TEACH
		appMiddleware.WriteError(w, r, http.StatusInternalServerError, "internal", "Failed to save task", nil)
		return
	}

	// NEW: Location -- маленькое, но полезное улучшение HTTP-контракта для 201 Created.
	w.Header().Set("Location", fmt.Sprintf("/api/v1/tasks/%d", created.ID))

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
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "bad_request", "Invalid ID",
			map[string]any{"id": idStr}) // NEW-TEACH
		return
	}

	task, ok, err := h.svc.GetTask(ctx, id)
	if err != nil {
		if h.handleContextError(w, r, err) {
			return
		}
		log.Printf("request_id=%s getTaskByID error: %v", appMiddleware.GetRequestID(ctx), err) // NEW-TEACH
		appMiddleware.WriteError(w, r, http.StatusInternalServerError, "internal", "Failed to get task", nil)
		return
	}
	if !ok {
		appMiddleware.WriteError(w, r, http.StatusNotFound, "not_found", "Task not found",
			map[string]any{"id": id}) // NEW-TEACH
		return
	}

	// [CHANGE] Content-Type выставляет JSONHeaderMiddleware
	_ = json.NewEncoder(w).Encode(task)

}

// updateTask обрабатывает PUT /api/v1/tasks/{id}
//
// Обновляет Title/Done у задачи, сохраняет список на диск, возвращает обновлённую задачу.
func (h *Handler) updateTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "bad_request", "Invalid ID",
			map[string]any{"id": idStr}) // NEW-TEACH
		return
	}

	// NEW: PUT валидируем через DTO, чтобы контракт был таким же строгим, как в POST.
	var req UpdateTaskRequest
	if err := decodeJSONStrict(r, &req); err != nil {
		h.writeDecodeError(w, r, err)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "validation_error", "Validation failed",
			validationDetails(err))
		return
	}

	incoming := Task{
		Title:    req.Title,
		Done:     req.Done,
		Priority: req.Priority,
	}

	updated, ok, err := h.svc.UpdateTask(ctx, id, incoming)
	if err != nil {
		if h.handleContextError(w, r, err) {
			return
		}
		log.Printf("request_id=%s updateTask error: %v", appMiddleware.GetRequestID(ctx), err) // NEW-TEACH
		appMiddleware.WriteError(w, r, http.StatusInternalServerError, "internal", "Failed to save tasks", nil)
		return
	}
	if !ok {
		// Если задача с запрашиваемым ID не найдена
		appMiddleware.WriteError(w, r, http.StatusNotFound, "not_found", "Task not found",
			map[string]any{"id": id}) // NEW-TEACH
		return
	}

	_ = json.NewEncoder(w).Encode(updated)
}

// deleteTask обрабатывает DELETE /api/v1/tasks/{id}
//
// Удаляет задачу, сохраняет список на диск, возвращает 204.
func (h *Handler) deleteTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "bad_request", "Invalid ID",
			map[string]any{"id": idStr}) // NEW
		return
	}

	ok, err := h.svc.DeleteTask(ctx, id)
	if err != nil {
		if h.handleContextError(w, r, err) {
			return
		}
		log.Printf("request_id=%s deleteTask error: %v", appMiddleware.GetRequestID(ctx), err) // NEW-TEACH
		appMiddleware.WriteError(w, r, http.StatusInternalServerError, "internal", "Failed to save tasks", nil)
		return
	}
	if !ok {
		appMiddleware.WriteError(w, r, http.StatusNotFound, "not_found", "Task not found",
			map[string]any{"id": id}) // NEW
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
func (h *Handler) handleContextError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, context.Canceled):
		// Запрос отменён: клиент ушёл ИЛИ сервер делает graceful shutdown.
		// Часто отвечать уже некому (соединение закрыто), поэтому просто прекращаем работу.
		return true
	case errors.Is(err, context.DeadlineExceeded):
		// Таймаут запроса (например, наш RequestTimeoutMiddleware).
		// http.Error(w, "Request timeout", http.StatusRequestTimeout) // 408

		// NEW-TEACH: таймаут -- часть контракта; возвращаем единый JSON error.
		appMiddleware.WriteError(w, r, http.StatusRequestTimeout, "timeout", "Request timeout", nil)
		return true
	default:
		return false
	}
}

// NEW-TEACH: строгий JSON decode -- "ровно один JSON", неизвестные поля запрещены.
func decodeJSONStrict(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}

	// Проверяем, что в body нет второго JSON значения.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

// NEW: аккуратно маппим ошибки декодирования/лимита в стабильные HTTP ответы.
func (h *Handler) writeDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	var maxErr *http.MaxBytesError
	switch {
	case errors.As(err, &maxErr):
		appMiddleware.WriteError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large",
			"Request body is too large", map[string]any{"limit_bytes": maxErr.Limit})
	case errors.Is(err, io.EOF):
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "bad_request",
			"Empty request body", nil)
	default:
		// Для 400 допустимо дать "details" с причиной, это полезно клиенту при отладке.
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "bad_request",
			"Invalid JSON", map[string]any{"error": err.Error()})
	}
}

// NEW: преобразуем ошибки validator в стабильный details для клиента (без внутренних названий структур).
func validationDetails(err error) any {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return map[string]any{"error": err.Error()}
	}

	type item struct {
		Field string `json:"field"`
		Rule  string `json:"rule"`
	}
	out := make([]item, 0, len(verrs))
	for _, fe := range verrs {
		out = append(out, item{
			Field: fe.Field(),
			Rule:  fe.Tag(),
		})
	}
	return out
}
