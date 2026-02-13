package tasks

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
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
func (ts *TaskStore) SaveTasks(tasks []Task) error {
	ts.mu.Lock()         // Блокируем на запись
	defer ts.mu.Unlock() // Разблокируем при выходе из функции

	data, err := json.MarshalIndent(tasks, "", "   ")
	if err != nil {
		return err
	}

	// 0644 - права доступа (rw-r--r--)
	return os.WriteFile(ts.filename, data, 0644)
}

// LoadTasks загружает задачи из файла.
//
// Использует RLock (разделяемая блокировка): безопасно для сценариев,
// где чтение может происходить параллельно с другими чтениями.
func (ts *TaskStore) LoadTasks() ([]Task, error) {
	ts.mu.RLock()         // Блокируем только на чтение
	defer ts.mu.RUnlock() // Разблокируем при выходе

	data, err := os.ReadFile(ts.filename)
	if err != nil {
		if os.IsNotExist(err) {
			// Если файла нет — это нормальная ситуация для первого запуска.
			return []Task{}, nil
		}
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

	return tasks, nil
}
