package tasks

import "context"

type TaskRepository interface {
	// Создать задачу. Должен принимать указатель на Task,
	// чтобы внутри метода можно было присвоить задаче сгенерированный ID.
	Create(ctx context.Context, task *Task) error

	// Получить задачу по ID. Возвращает указатель на задачу и ошибку.
	GetByID(ctx context.Context, id int, userID int) (*Task, error)

	// Получить все задачи. Возвращает слайс.
	GetAll(ctx context.Context, userID int) ([]Task, error)

	// Обновить задачу.
	Update(ctx context.Context, task *Task, userID int) error

	// Удалить задачу по ID.
	Delete(ctx context.Context, id int, userID int) error

	// Создать нового пользователя
	CreateUser(ctx context.Context, user *User) error

	// Ищет пользователя по email и возвращает заполненную структуру.
	GetUserByEmail(ctx context.Context, email string) (*User, error)

	// CreateSubtask создает подзадачу, привязанную к задаче
	CreateSubtask(ctx context.Context, subtask *SubTask) error
}

// calcNextID — helper для корректного nextID после чтения из  JSON файла.
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
