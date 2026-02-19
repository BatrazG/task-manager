package tasks

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"

	// "task-manager/internal/middleware"
	appMiddleware "task-manager/internal/middleware" // [CHANGE] подключаем middleware-пакет (алиас, чтобы не путать с chi/middleware)

	"github.com/go-chi/chi/v5"
)

// Task — модель задачи.
//
// Хранится в памяти (для скорости) и сериализуется в JSON (для API и файла).
type Task struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

// Handler — HTTP-слой модуля задач.
//
// Здесь лежит всё, что относится к HTTP:
// роуты, парсинг JSON, выставление заголовков, коды ответов, middleware.

// Раньше состояние было в package-level переменных в main.
//
//	Теперь это поля структуры Handler, чтобы:
//	- отделить HTTP-слой от запуска приложения (cmd/.../main.go);
//	- убрать глобальные переменные;
//	- упростить тестирование и расширение.
type Handler struct {
	store  *TaskStore   // Хранилище (файловое)
	tasks  []Task       // Кэш задач в памяти
	nextID int          // Следующий ID для новых задач
	mu     sync.RWMutex // Защищает tasks + nextID при параллельных запросах
}

// NewHandler создаёт Handler и загружает данные из хранилища.
//
//	 Логика загрузки/инициализации переехала из main в конструктор,
//		чтобы main оставался "только запуском"
func NewHandler(store *TaskStore) *Handler {
	h := &Handler{
		store:  store,
		tasks:  []Task{},
		nextID: 1,
	}

	loaded, err := store.LoadTasks()
	if err == nil {
		h.tasks = loaded
		h.nextID = calcNextID(h.tasks)
		//  В исходнике при ошибке чтения печаталось предупреждение.
		//         Здесь намеренно "молча" стартуем с пустым списком при ошибке.
		//         Почему: на предыдущих парах мы с вами оговаривали моменты, когда общаемся с клиентом,
		//         когда передаем ошибку или лог выше
		// 		   пакет internal/tasks не должен напрямую печатать в stdout;
		//         логирование — ответственность cmd/task-server/main.go или отдельного логгера.
	} else {
		// Стартуем с пустым списком, nextID остаётся 1.
	}

	return h
}

// Router собирает HTTP-роутер для задач.
//
// Здесь размещаем всё связывание путей с обработчиками.
// Эта часть переезжает практически без изменений
func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()

	r.Route("/api/v1/tasks", func(r chi.Router) {
		// [CHANGE] JSONHeaderMiddleware вешаем на весь tasks API,
		// чтобы убрать дублирующиеся Content-Type из хендлеров.
		r.Use(appMiddleware.JSONHeaderMiddleware)

		// MVP: GET / (список), POST / (создание)
		r.Get("/", h.getAllTasks)
		r.Post("/", h.createTask)

		// Advanced: GET /{id}
		r.Get("/{id}", h.getTaskByID)

		// PUT: обновление
		r.Put("/{id}", h.updateTask)

		// [CHANGE] Вместо AdminOnly используем BasicAuthMiddleware только на DELETE.
		r.With(appMiddleware.BasicAuthMiddleware).Delete("/{id}", h.deleteTask)
		//r.With(AdminOnly).Delete("/{id}", h.deleteTask)
	})

	return r
}

// Убираем, так как заменили на BasicAuth из middleware
// AdminOnly — middleware: проверяет доступ по заголовку.
//
// Работает как обёртка над handler.
// При неверном ключе отвечает 403 и не передаёт управление дальше.
/*func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// В будущем заменить на реальную проверку админа.
		// Пока оставим как заглушку.
		if r.Header.Get("X-Admin-Key") != "secret123" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}*/

// getAllTasks обрабатывает GET /api/v1/tasks/
//
// Возвращает полный список задач в JSON.
func (h *Handler) getAllTasks(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()         // Потокобезопасное чтение
	defer h.mu.RUnlock() // Разблокируем при выходе

	// [CHANGE] Content-Type выставляет JSONHeaderMiddleware
	//w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.tasks)
}

// createTask обрабатывает POST /api/v1/tasks/
//
// Создаёт задачу, выдаёт ID, сохраняет список на диск, возвращает созданную задачу.
func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
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

	h.mu.Lock()         // Потокобезопасная запись
	defer h.mu.Unlock() // Разблокируем при выходе

	task.ID = h.nextID
	h.nextID++

	h.tasks = append(h.tasks, task)

	// Сохраняем на диск
	if err := h.store.SaveTasks(h.tasks); err != nil {
		http.Error(w, "Failed to save tasks", http.StatusInternalServerError)
		return
	}

	// Возвращаем JSON созданной задачи.
	// [CHANGE] Content-Type выставляет JSONHeaderMiddleware
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(task)
}

// getTaskByID обрабатывает GET /api/v1/tasks/{id}
//
// Находит задачу по ID и возвращает её.
func (h *Handler) getTaskByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, task := range h.tasks {
		if task.ID == id {
			// [CHANGE] Content-Type выставляет JSONHeaderMiddleware
			_ = json.NewEncoder(w).Encode(task)
			return
		}
	}

	http.Error(w, "Task not found", http.StatusNotFound)
}

// updateTask обрабатывает PUT /api/v1/tasks/{id}
//
// Обновляет Title/Done у задачи, сохраняет список на диск, возвращает обновлённую задачу.
func (h *Handler) updateTask(w http.ResponseWriter, r *http.Request) {
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

	h.mu.Lock()
	defer h.mu.Unlock()

	for i := range h.tasks {
		if h.tasks[i].ID == id {
			// Обновляем поля (ID фиксируем по URL).
			h.tasks[i].Title = incoming.Title
			h.tasks[i].Done = incoming.Done

			// Сохраняем после PUT
			if err := h.store.SaveTasks(h.tasks); err != nil {
				http.Error(w, "Failed to save tasks", http.StatusInternalServerError)
				return
			}

			// [CHANGE] Content-Type выставляет JSONHeaderMiddleware
			_ = json.NewEncoder(w).Encode(h.tasks[i])
			return
		}
	}

	// Если задача с запрашиваемым ID не найдена
	http.Error(w, "Task not found", http.StatusNotFound)
}

// deleteTask обрабатывает DELETE /api/v1/tasks/{id}
//
// Удаляет задачу, сохраняет список на диск, возвращает 204.
func (h *Handler) deleteTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for i, task := range h.tasks {
		if task.ID == id {
			h.tasks = append(h.tasks[:i], h.tasks[i+1:]...)

			// Сохраняем после DELETE.
			if err := h.store.SaveTasks(h.tasks); err != nil {
				http.Error(w, "Failed to save tasks", http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	http.Error(w, "Task not found", http.StatusNotFound)
}

// calcNextID — helper для корректного nextID после чтения из файла.
//
// Вычисляет следующий свободный ID как maxID+1.
func calcNextID(ts []Task) int {
	maxID := 0
	for _, t := range ts {
		if t.ID > maxID {
			maxID = t.ID
		}
	}
	return maxID + 1
}
