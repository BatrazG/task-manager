package tasks

import "context"

type TaskRepository interface {
	// 1. Создать задачу. Должен принимать указатель на Task,
	// чтобы внутри метода можно было присвоить задаче сгенерированный ID.
	Create(ctx context.Context, task *Task) error

	// 2. Получить задачу по ID. Возвращает указатель на задачу и ошибку.
	GetByID(ctx context.Context, id int, userID int) (*Task, error)

	// 3. Получить все задачи. Возвращает слайс.
	GetAll(ctx context.Context, userID int) ([]Task, error)

	// 4. Обновить задачу.
	Update(ctx context.Context, task *Task, userID int) error

	// 5. Удалить задачу по ID.
	Delete(ctx context.Context, id int, userID int) error

	// 6. Создать нового пользователя
	CreateUser(ctx context.Context, user *User) error

	// 7. Ищет пользователя по email и возвращает заполненную структуру.
	GetUserByEmail(ctx context.Context, email string) (*User, error)
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
