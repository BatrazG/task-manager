// [CHANGE-CONTEXT] создадим минимальный слой бизнес-логики
// чтобы лучше разобраться с темой
package tasks

import (
	"context"
	"sync"
	"time"
)

// Service - слой бизнес-логики
// В нашем учебном проекте он минимальный, но нужен, чтобы было видно
// "протекание" контекста по слоям: handler -> service -> store
type Service struct {
	store *TaskStore

	mu     sync.RWMutex
	tasks  []Task
	nextID int
}

// NewService создает сервис и загружает задачи из ранилища

// Принимаем ctx, чтобы даже инициализация уважала отмену
func NewService(ctx context.Context, store *TaskStore) (*Service, error) {
	loaded, err := store.LoadTasks(ctx)
	if err != nil {
		return nil, err
	}

	return &Service{
		store:  store,
		tasks:  loaded,
		nextID: calcNextID(loaded),
	}, nil
}

// ListTasks возвращает список задач.
// Если delay > 0, симулируем "медленное I/O" в нижнем слое (store),
// чтобы можно было демонстрировать cancel/timeout.
//
// delay прерывается по ctx.Done().
func (s *Service) ListTasks(ctx context.Context, delay time.Duration) ([]Task, error) {
	// Зачем начинать долгую операцию,
	// если контекст уже отменен
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if delay > 0 {
		if err := s.store.SimulateSlowIO(ctx, delay); err != nil {
			return nil, err
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock() // Блокируе только на запись
	defer s.mu.RUnlock()

	out := make([]Task, len(s.tasks))
	copy(out, s.tasks)
	return out, nil
}

// GetTask возвращает задачу по id.
func (s *Service) GetTask(ctx context.Context, id int) (Task, bool, error) {
	if err := ctx.Err(); err != nil {
		return Task{}, false, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, t := range s.tasks {
		if t.ID == id {
			return t, true, nil
		}
	}
	return Task{}, false, nil
}

// CreateTask создаёт задачу и сохраняет в файл.
func (s *Service) CreateTask(ctx context.Context, incoming Task) (Task, error) {
	if err := ctx.Err(); err != nil {
		return Task{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	created := Task{
		ID:    s.nextID,
		Title: incoming.Title,
		Done:  incoming.Done,
	}

	// Готовим новый список, но НЕ коммитим в память, пока не сохранили на диск.
	candidate := make([]Task, 0, len(s.tasks)+1)
	candidate = append(candidate, s.tasks...)
	candidate = append(candidate, created)

	if err := s.store.SaveTasks(ctx, candidate); err != nil {
		return Task{}, err
	}

	s.tasks = candidate
	s.nextID++
	return created, nil
}
