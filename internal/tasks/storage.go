package tasks

import (
	"context" // [CHANGE-CONTEXT]
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time" // [CHANGE-CONTEXT]
)

// TaskStore отвечает за хранение задач в файле.
//
// Хранилище потокобезопасно: операции чтения/записи защищены RWMutex.
// Это важно, если в будущем появятся параллельные операции бэкапа/перечитывания.
type TaskStore struct {
	mu       sync.RWMutex // Мьютекс для защиты доступа к файлу и данным при I/O операциях
	filename string       // Имя файла базы данных (например, tasks.json)
}

// NewTaskStore создаёт новое файловое хранилище задач.
func NewTaskStore(filename string) *TaskStore {
	return &TaskStore{filename: filename}
}

// SaveTasks сохраняет задачи в файл JSON.
//
// Важно: метод берёт Lock, потому что идёт запись на диск.
// Форматирование JSON (MarshalIndent) используется для читаемости файла.
//
// [CHANGE-CONTEXT] Добавляется ctx -- первый аргумент. Уважаем отмену/таймаут до/после потенциально долгих шагов.
func (ts *TaskStore) SaveTasks(ctx context.Context, tasks []Task) error {
	// [CHANGE-CONTEXT]
	if err := ctx.Err(); err != nil {
		return err
	}

	ts.mu.Lock()         // Блокируем на запись
	defer ts.mu.Unlock() // Разблокируем при выходе из функции

	if err := ctx.Err(); err != nil { // [CHANGE-CONTEXT]
		return err
	}

	data, err := json.MarshalIndent(tasks, "", "   ")
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil { // [CHANGE-CONTEXT]
		return err
	}

	// 0644 - права доступа (rw-r--r--)
	return os.WriteFile(ts.filename, data, 0644)
}

// LoadTasks загружает задачи из файла.
//
// Использует RLock (разделяемая блокировка): безопасно для сценариев,
// где чтение может происходить параллельно с другими чтениями.
//
// [CHANGE-CONTEXT] Добавляется ctx -- первый аргумент. Уважаем отмену/таймаут до/после потенциально долгих шагов.
func (ts *TaskStore) LoadTasks(ctx context.Context) ([]Task, error) {
	if err := ctx.Err(); err != nil { // [CHANGE-CONTEXT]
		return nil, err
	}

	ts.mu.RLock()         // Блокируем только на чтение
	defer ts.mu.RUnlock() // Разблокируем при выходе

	if err := ctx.Err(); err != nil { // [CHANGE-CONTEXT]
		return nil, err
	}

	data, err := os.ReadFile(ts.filename)
	if err != nil {
		if os.IsNotExist(err) {
			// Если файла нет — это нормальная ситуация для первого запуска.
			return []Task{}, nil
		}
		return nil, err
	}

	if err := ctx.Err(); err != nil { // [CHANGE-CONTEXT]
		return nil, err
	}

	// Пустой файл — не ошибка, просто нет задач.
	if len(strings.TrimSpace(string(data))) == 0 {
		return []Task{}, nil
	}

	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil { // [CHANGE-CONTEXT]
		return nil, err
	}

	return tasks, nil
}

// SimulateSlowIO симулирует "медленное I/O", которое можно прервать через ctx.Done().
//
// [CHANGE-CONTEXT] Это учебная имитация "медленной БД/файла":
// select { time.After(d) vs ctx.Done() } -- минимальный, но важный паттерн.
func (ts *TaskStore) SimulateSlowIO(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err() // nil, если всё ок; ошибка, если уже отменено
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return ctx.Err() // nil, если не отменено
	case <-ctx.Done():
		return ctx.Err()
	}
}
