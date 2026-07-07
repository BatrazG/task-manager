// Слой бизнес-логики
package tasks

import (
	"context"
	"errors"

	"time"

	"os"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Service - слой бизнес-логики
// В нашем учебном проекте он минимальный, но нужен, чтобы было видно
// "протекание" контекста по слоям: handler -> service -> store
type Service struct {
	repo TaskRepository
}

// NewService создает сервис и загружает задачи из хранилища

// Принимаем ctx, чтобы даже инициализация уважала отмену
func NewService(repo TaskRepository) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) CreateTask(ctx context.Context, task *Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Просто делегируем задачу репозиторию
	return s.repo.Create(ctx, task)
}

func (s *Service) GetTaskByID(ctx context.Context, id int, userID int) (*Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, id, userID)
}

func (s *Service) GetAllTasks(ctx context.Context, userID int) ([]Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return s.repo.GetAll(ctx, userID)
}

func (s *Service) UpdateTask(ctx context.Context, task *Task, userID int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Защита от возможной подмены
	task.UserID = userID

	return s.repo.Update(ctx, task, userID)
}

func (s *Service) DeleteTask(ctx context.Context, id int, userID int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return s.repo.Delete(ctx, id, userID)
}

func (s *Service) CreateSubTask(ctx context.Context, subtask *SubTask, userID int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := s.repo.GetByID(ctx, subtask.TaskID, userID)
	if errors.Is(err, ErrTaskNotFound) {
		return err
	}
	if err != nil {
		return err
	}

	return s.repo.CreateSubtask(ctx, subtask)
}

// Register - бизнес-логика регистрации пользователя
func (s *Service) Register(ctx context.Context, req RegisterRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if req.InviteCode != os.Getenv("REGISTRATION_INVITE_CODE") {
		return errors.New("invalid invite code") // или кастомная ошибка
	}

	_, err := s.repo.GetUserByEmail(ctx, req.Email)

	if err == nil {
		// Пользователь нашелся без ошибок -> email точно занят!
		return ErrUserAlreadyExists
	}
	// Если ошибка НЕ связана с тем, что юзер не найден — значит это фатальная ошибка БД
	if !errors.Is(err, ErrUserNotFound) {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	hash := string(hashedPassword)
	u := User{
		Email:        req.Email,
		PasswordHash: hash,
	}
	err = s.repo.CreateUser(ctx, &u)
	if err != nil {
		return err
	}
	return nil
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	u, err := s.repo.GetUserByEmail(ctx, req.Email)
	if errors.Is(err, ErrUserNotFound) {
		return "", ErrInvalidCredentials
	}

	if err != nil {
		return "", err
	}

	err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password))
	if err != nil {
		return " ", ErrInvalidCredentials
	}

	claims := jwt.MapClaims{
		"user_id": u.ID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(), // Токен сгорит через сутки
	}

	// Создаем и подписываеем токен
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Забираем секретный ключ из окружения
	secret := os.Getenv("JWT_SECRET")

	// Превращаем токен в финальную строку
	tokenString, err := token.SignedString([]byte(secret))

	return tokenString, nil
}

// Устаревшие методы из бизнес-логики,
// когда хранилище было только в JSON
/*
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
		ID:       s.nextID,
		Title:    incoming.Title,
		Done:     incoming.Done,
		Priority: incoming.Priority,
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

// UpdateTask обновляет задачу по id и сохраняет в файл.
func (s *Service) UpdateTask(ctx context.Context, id int, incoming Task) (Task, bool, error) {
	if err := ctx.Err(); err != nil {
		return Task{}, false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return Task{}, false, nil
	}

	updated := s.tasks[idx]
	updated.Title = incoming.Title
	updated.Done = incoming.Done
	// PUT должен обновлять priority тоже -- иначе контракт "полуработающий".
	updated.Priority = incoming.Priority

	candidate := make([]Task, len(s.tasks))
	copy(candidate, s.tasks)
	candidate[idx] = updated

	if err := s.store.SaveTasks(ctx, candidate); err != nil {
		return Task{}, false, err
	}

	s.tasks = candidate
	return updated, true, nil
}

// DeleteTask удаляет задачу по id и сохраняет в файл.
func (s *Service) DeleteTask(ctx context.Context, id int) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false, nil
	}

	candidate := make([]Task, 0, len(s.tasks)-1)
	candidate = append(candidate, s.tasks[:idx]...)
	candidate = append(candidate, s.tasks[idx+1:]...)

	if err := s.store.SaveTasks(ctx, candidate); err != nil {
		return false, err
	}

	s.tasks = candidate
	return true, nil
}
*/
