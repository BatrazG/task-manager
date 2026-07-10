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

	"task-manager/internal/middleware"
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

func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()

	// =========================================================================
	// ГЛОБАЛЬНАЯ ЦЕПОЧКА MIDDLEWARE
	// =========================================================================
	r.Use(appMiddleware.RequestIDMiddleware)                       // 1. Сквозной ID
	r.Use(appMiddleware.LoggingMiddleware)                         // 2. Логгер статус-кодов
	r.Use(appMiddleware.NewCORSMiddleware())                       // 3. CORS-фильтр (внутри папки internal/middleware)
	r.Use(appMiddleware.JSONHeaderMiddleware)                      // 4. JSON заголовок
	r.Use(appMiddleware.BodyLimitMiddleware(1 << 20))              // 5. Ограничение тела в 1 МБ
	r.Use(appMiddleware.RequestTimeoutMiddleware(2 * time.Second)) // 6. Таймаут 2 секунды

	// Настройка системных ответов 404/405
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		appMiddleware.WriteError(w, req, http.StatusNotFound, "not_found", "Route not found",
			map[string]any{"path": req.URL.Path})
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		appMiddleware.WriteError(w, req, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed",
			map[string]any{"method": req.Method, "path": req.URL.Path})
	})

	// =========================================================================
	// МАРШРУТЫ API V1
	// =========================================================================
	r.Route("/api/v1", func(r chi.Router) {

		// Группа Авторизации (Открытая)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", h.registerUser)
			r.Post("/login", h.loginUser)
		})

		// Группа Задач (Закрытая семейным токеном)
		r.Route("/tasks", func(r chi.Router) {
			r.Use(appMiddleware.AuthMiddleware) // <--- ИСПРАВИЛИ ПРЕФИКС НА appMiddleware!

			r.Get("/users", h.getAllUsers)

			r.Get("/", h.getAllTasks)
			r.Post("/", h.createTask)
			r.Get("/{id}", h.getTaskByID)
			r.Put("/{id}", h.updateTask)
			r.Delete("/{id}", h.deleteTask)
			r.Post("/{id}/subtasks", h.createSubTask)
		})
	})

	return r
}

// getAllTasks обрабатывает GET /api/v1/tasks.
// Возвращает список задач, где текущий пользователь является автором или исполнителем.
func (h *Handler) getAllTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Извлекаем ID авторизованного пользователя, записанный JWT-middleware.
	// Используем панику приведения типов .(int), так как middleware гарантирует наличие этого значения.
	userID := ctx.Value(middleware.UserIDKey).(int)

	// Передаем userID в бизнес-логику для обеспечения изоляции данных членов семьи
	tasks, err := h.svc.GetAllTasks(ctx, userID)
	if err != nil {
		if h.handleContextError(w, r, err) {
			return
		}
		log.Printf("request_id=%s getAllTasks error: %v", appMiddleware.GetRequestID(ctx), err)
		appMiddleware.WriteError(w, r, http.StatusInternalServerError, "internal", "Failed to get tasks", nil)
		return
	}

	// Если список пуст, Encode автоматически отдаст клиенту корректный пустой массив []
	_ = json.NewEncoder(w).Encode(tasks)
}

func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := ctx.Value(middleware.UserIDKey).(int)

	// 1. DTO И ВАЛИДАЦИЯ
	var req CreateTaskRequest
	if err := decodeJSONStrict(r, &req); err != nil {
		h.writeDecodeError(w, r, err)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "validation_error", "Validation failed",
			validationDetails(err))
		return
	}

	// Если исполнитель не указывается при создании,
	// то исполнителем назначается создатель задачи
	if req.AssignedTo == 0 {
		req.AssignedTo = userID
	}

	// 2. Маппим DTO в доменную модель
	incoming := Task{
		UserID:     userID,
		AssignedTo: req.AssignedTo,
		Title:      req.Title,
		Done:       req.Done,
		Priority:   req.Priority,
	}

	// 3. Отправляем в сервис по указателю (тут запишется новый ID)
	err := h.svc.CreateTask(ctx, &incoming)
	if err != nil {
		if h.handleContextError(w, r, err) {
			return
		}
		log.Printf("request_id=%s createTask error: %v", appMiddleware.GetRequestID(ctx), err)
		appMiddleware.WriteError(w, r, http.StatusInternalServerError, "internal", "Failed to save task", nil)
		return
	}

	// 4. Формируем ответ
	w.Header().Set("Location", fmt.Sprintf("/api/v1/tasks/%d", incoming.ID))
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(incoming)
}

// getTaskByID обрабатывает GET /api/v1/tasks/{id}
//
// Находит задачу по ID и возвращает её.
func (h *Handler) getTaskByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Извлекаем ID авторизованного пользователя, записанный JWT-middleware.
	// Используем панику приведения типов .(int), так как middleware гарантирует наличие этого значения.
	userID := ctx.Value(middleware.UserIDKey).(int)

	// Парсим id задачи из запроса
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "bad_request", "Invalid ID",
			map[string]any{"id": idStr})
		return
	}

	// Передаем UserID в бизнес-логику для обеспеения изоляции данных
	task, err := h.svc.GetTaskByID(ctx, id, userID)
	if errors.Is(err, ErrTaskNotFound) {
		appMiddleware.WriteError(w, r, http.StatusNotFound, "not_found", "Task not found",
			map[string]any{"id": id}) // NEW-TEACH
		return
	}
	if err != nil {
		if h.handleContextError(w, r, err) {
			return
		}
		log.Printf("request_id=%s getTaskByID error: %v", appMiddleware.GetRequestID(ctx), err) // NEW-TEACH
		appMiddleware.WriteError(w, r, http.StatusInternalServerError, "internal", "Failed to get task", nil)
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

	// Извлекаем ID авторизованного пользователя, записанный JWT-middleware.
	// Используем панику приведения типов .(int), так как middleware гарантирует наличие этого значения.
	userID := ctx.Value(middleware.UserIDKey).(int)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "bad_request", "Invalid ID",
			map[string]any{"id": idStr}) // NEW-TEACH
		return
	}

	// PUT валидируем через DTO, чтобы контракт был таким же строгим, как в POST.
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

	if req.AssignedTo == 0 {
		req.AssignedTo = userID // или task.AssignedTo = userID в зависимости от вашей структуры переменных
	}
	incoming := Task{
		ID:         id,
		Title:      req.Title,
		Done:       req.Done,
		Priority:   req.Priority,
		AssignedTo: req.AssignedTo,
	}

	err = h.svc.UpdateTask(ctx, &incoming, userID)
	if errors.Is(err, ErrTaskNotFound) {
		// Если задача с запрашиваемым ID не найдена
		appMiddleware.WriteError(w, r, http.StatusNotFound, "not_found", "Task not found",
			map[string]any{"id": id}) // NEW-TEACH
		return
	}
	if err != nil {
		if h.handleContextError(w, r, err) {
			return
		}
		log.Printf("request_id=%s updateTask error: %v", appMiddleware.GetRequestID(ctx), err)
		appMiddleware.WriteError(w, r, http.StatusInternalServerError, "internal", "Failed to save tasks", nil)
		return
	}

	_ = json.NewEncoder(w).Encode(incoming)
}

// deleteTask обрабатывает DELETE /api/v1/tasks/{id}
//
// Удаляет задачу, сохраняет список на диск, возвращает 204.
func (h *Handler) deleteTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Извлекаем ID авторизованного пользователя, записанный JWT-middleware.
	// Используем панику приведения типов .(int), так как middleware гарантирует наличие этого значения.
	userID := ctx.Value(middleware.UserIDKey).(int)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "bad_request", "Invalid ID",
			map[string]any{"id": idStr}) // NEW
		return
	}

	err = h.svc.DeleteTask(ctx, id, userID)
	if errors.Is(err, ErrTaskNotFound) {
		appMiddleware.WriteError(w, r, http.StatusNotFound, "not_found", "Task not found",
			map[string]any{"id": id}) // NEW
		return
	}
	if err != nil {
		if h.handleContextError(w, r, err) {
			return
		}
		log.Printf("request_id=%s deleteTask error: %v", appMiddleware.GetRequestID(ctx), err) // NEW-TEACH
		appMiddleware.WriteError(w, r, http.StatusInternalServerError, "internal", "Failed to save tasks", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createSubTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := ctx.Value(middleware.UserIDKey).(int)

	taskIDStr := chi.URLParam(r, "id")
	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "bad_request", "Invalid Task ID",
			map[string]any{"id": taskIDStr})
		return
	}

	var req CreateSubTaskRequest
	err = decodeJSONStrict(r, &req)
	if err != nil {
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "validation_error", "Validation failed",
			validationDetails(err))
		return
	}

	if err = h.validate.Struct(req); err != nil {
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "validation_error", "Validation failed",
			validationDetails(err))
		return
	}

	incoming := SubTask{
		TaskID: taskID, // Сюда передаем ID из URL
		Title:  req.Title,
		Done:   false, // Новая покупка всегда не выполнена по умолчанию
	}

	err = h.svc.CreateSubTask(ctx, &incoming, userID)

	if errors.Is(err, ErrTaskNotFound) {
		appMiddleware.WriteError(w, r, http.StatusNotFound, "not_found", "Task not found",
			map[string]any{"id": taskID})
		return
	}

	if err != nil {
		if h.handleContextError(w, r, err) {
			return
		}
		log.Printf("request_id=%s updateTask error: %v", appMiddleware.GetRequestID(ctx), err)
		appMiddleware.WriteError(w, r, http.StatusInternalServerError, "internal", "Failed to save subtask", nil)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/api/v1/tasks/%d", incoming.ID))
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(incoming)
}

func (h *Handler) registerUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. DTO и валидация
	var req RegisterRequest
	if err := decodeJSONStrict(r, &req); err != nil {
		h.writeDecodeError(w, r, err)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "validation_error", "valiation_failed",
			validationDetails(err))
		return
	}

	// Маппить DTO в доменную модель не нужно
	// т.к. DTO передается непосредственно в метод Register
	err := h.svc.Register(ctx, req)
	if errors.Is(err, ErrUserAlreadyExists) {
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "bad_request", "Такой Email уже занят",
			nil)
		return
	}

	if err != nil {
		if h.handleContextError(w, r, err) { // Проверка на таймаут контекста
			return
		}
		log.Printf("request_id=%s registerUser error: %v", appMiddleware.GetRequestID(ctx), err)
		appMiddleware.WriteError(w, r, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера", nil)
		return
	}

	w.WriteHeader(http.StatusCreated)
	data := map[string]string{"status": "ok"}
	_ = json.NewEncoder(w).Encode(data)
}

func (h *Handler) loginUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. DTO и валидация
	var req LoginRequest
	if err := decodeJSONStrict(r, &req); err != nil {
		h.writeDecodeError(w, r, err)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		appMiddleware.WriteError(w, r, http.StatusBadRequest, "validation_error", "valiation_failed",
			validationDetails(err))
		return
	}

	token, err := h.svc.Login(ctx, req)
	if errors.Is(err, ErrInvalidCredentials) {
		appMiddleware.WriteError(w, r, http.StatusUnauthorized, "Unauthorized", "Неверный логин или пароль",
			nil)
		return
	}

	if err != nil {
		if h.handleContextError(w, r, err) { // Проверка на таймаут контекста
			return
		}
		log.Printf("request_id=%s loginUser error: %v", appMiddleware.GetRequestID(ctx), err)
		appMiddleware.WriteError(w, r, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера", nil)
		return
	}

	w.WriteHeader(http.StatusOK)
	tokenData := map[string]string{"token": token}
	_ = json.NewEncoder(w).Encode(tokenData)
}

func (h *Handler) getAllUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	users, err := h.svc.GetAllUsers(ctx)
	if err != nil {
		if h.handleContextError(w, r, err) {
			return
		}
		log.Printf("request_id=%s getAllUsers error: %v", appMiddleware.GetRequestID(ctx), err)
		appMiddleware.WriteError(w, r, http.StatusInternalServerError, "internal", "Failed to load users", nil)
		return
	}

	// Отдаем массив пользователей фронтенду
	_ = json.NewEncoder(w).Encode(users)
}

// handleContextError делает понятную обработку ошибок отмены/таймаута.
func (h *Handler) handleContextError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, context.Canceled):
		// Запрос отменен: клиент ушел ИЛИ сервер делает graceful shutdown.
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
