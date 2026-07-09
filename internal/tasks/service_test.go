package tasks

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Пародийный стор (заглушка) для тестов, чтобы не ходить в реальный Postgres
type mockRepository struct {
	task *Task
}

func (m *mockRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	if email == "test@family.com" {
		// Генерируем честный хэш от пароля "Secret123" прямо в памяти во время запуска теста
		hash, _ := bcrypt.GenerateFromPassword([]byte("Secret123"), bcrypt.DefaultCost)

		return &User{
			ID:           42,
			Email:        "test@family.com",
			PasswordHash: string(hash), // Передаем живой, гарантированный хэш
		}, nil
	}
	return nil, ErrUserNotFound
}

func (m *mockRepository) GetByID(ctx context.Context, id int, userID int) (*Task, error) {
	// Симулируем логику: если userID не совпадает с хозяином задачи, возвращаем ошибку
	if m.task == nil || (m.task.UserID != userID && m.task.AssignedTo != userID) {
		return nil, ErrTaskNotFound
	}
	return m.task, nil
}

func (m *mockRepository) CreateSubtask(ctx context.Context, subtask *SubTask) error {
	subtask.ID = 999 // Имитируем, что база выдала ID
	return nil
}

// Пустые заглушки для остальных методов интерфейса, чтобы компилятор не ругался
func (m *mockRepository) Create(ctx context.Context, task *Task) error             { return nil }
func (m *mockRepository) GetAll(ctx context.Context, userID int) ([]Task, error)   { return nil, nil }
func (m *mockRepository) Update(ctx context.Context, task *Task, userID int) error { return nil }
func (m *mockRepository) Delete(ctx context.Context, id int, userID int) error     { return nil }
func (m *mockRepository) CreateUser(ctx context.Context, user *User) error         { return nil }

// САМ ТЕСТ БИЗНЕС-ЛОГИКИ
func TestCreateSubTask_Security(t *testing.T) {
	ctx := context.Background()

	// Создаем тестовую задачу, которая принадлежит Пользователю №1 (Папе)
	testTask := &Task{
		ID:         5,
		UserID:     1,
		AssignedTo: 1,
		Title:      "Починить забор",
	}

	// Инициализируем наш тестовый репозиторий-заглушку
	repo := &mockRepository{task: testTask}

	// Создаем настоящий Сервис, но подсовываем ему нашу заглушку вместо Postgres!
	service := &Service{repo: repo}

	// -------------------------------------------------------------------------
	// СЦЕНАРИЙ 1: Создать подзадачу пытается законный владелец (Папа, ID=1)
	// -------------------------------------------------------------------------
	sub1 := &SubTask{TaskID: 5, Title: "Купить гвозди"}
	err := service.CreateSubTask(ctx, sub1, 1) // Передаем userID = 1
	if err != nil {
		t.Errorf("Ожидался успешный успех для владельца, но получена ошибка: %v", err)
	}
	if sub1.ID != 999 {
		t.Errorf("Ожидалось, что подзадаче присвоится ID 999, но получено: %d", sub1.ID)
	}

	// -------------------------------------------------------------------------
	// СЦЕНАРИЙ 2: Создать подзадачу тайно пытается чужой хакер (ID=3)
	// -------------------------------------------------------------------------
	sub2 := &SubTask{TaskID: 5, Title: "Хакерская подзадача"}
	err = service.CreateSubTask(ctx, sub2, 3) // Передаем чужой userID = 3

	// Мы ОЖИДАЕМ, что сервис вернет ошибку ErrTaskNotFound и заблокирует хакера
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Система безопасности пробита! Ожидалась ошибка ErrTaskNotFound для чужого пользователя, но получено: %v", err)
	}
}

func TestService_Login_SuccessAndFailure(t *testing.T) {
	ctx := context.Background()

	// Задаем временный секретный ключ в окружение для генерации JWT внутри сервиса
	secret := "test_secret_key_2026"
	os.Setenv("JWT_SECRET", secret)
	defer os.Unsetenv("JWT_SECRET")

	// Инициализируем наш мок-репозиторий и Сервис
	repo := &mockRepository{}
	service := &Service{repo: repo}

	// -------------------------------------------------------------------------
	// СЦЕНАРИЙ 1: Успешный вход с правильным паролем
	// -------------------------------------------------------------------------
	reqSuccess := LoginRequest{
		Email:    "test@family.com",
		Password: "Secret123", // Пароль совпадает с хэшем
	}

	tokenString, err := service.Login(ctx, reqSuccess)
	if err != nil {
		t.Fatalf("Ожидался успешный вход, но получена ошибка: %v", err)
	}

	// Проверяем криптографическую валидность токена, который сгенерировал Сервис
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		t.Fatalf("Сервер выдал поврежденный токен: %v", err)
	}

	// Извлекаем claims и проверяем, что сервис зашил туда правильный ID пользователя (42)
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		t.Fatal("Токен невалиден илиClaims повреждены")
	}

	if int(claims["user_id"].(float64)) != 42 {
		t.Errorf("В токен записан неверный user_id. Ожидался 42, получено: %v", claims["user_id"])
	}

	// -------------------------------------------------------------------------
	// СЦЕНАРИЙ 2: Ошибка входа (Неверный пароль)
	// -------------------------------------------------------------------------
	reqFailure := LoginRequest{
		Email:    "test@family.com",
		Password: "WrongPassword", // Неверный пароль
	}

	_, err = service.Login(ctx, reqFailure)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Ожидалась ошибка ErrInvalidCredentials, но получено: %v", err)
	}
}
