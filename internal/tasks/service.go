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

	return s.repo.GetByID(ctx, id)
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
	_, err := s.repo.GetByID(ctx, subtask.TaskID)
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

	_, err := s.repo.GetUserByUsername(ctx, req.Username)

	if err == nil {
		// Пользователь нашелся без ошибок -> username точно занят!
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
		Username:     req.Username,
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

	u, err := s.repo.GetUserByUsername(ctx, req.Username)

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

func (s *Service) GetAllUsers(ctx context.Context) ([]User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.repo.GetAllUsers(ctx)
}

// UpdateSubTaskStatus передает команду обновления статуса пункта чек-листа в базу данных.
func (s *Service) UpdateSubTaskStatus(ctx context.Context, subID int, done bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.repo.UpdateSubTaskStatus(ctx, subID, done)
}
