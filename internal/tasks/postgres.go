package tasks

import (
	"context"
	"database/sql"
	"errors"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{
		db: db,
	}
}

// 1. Создать задачу. Должен принимать указатель на Task,
// чтобы внутри метода можно было присвоить задаче сгенерированный ID.
func (r *PostgresRepository) Create(ctx context.Context, task *Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := r.db.QueryRowContext(ctx, "INSERT INTO tasks (user_id, assigned_to, title, done, priority) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		task.UserID, task.AssignedTo, task.Title, task.Done, task.Priority).Scan(&task.ID)

	if err != nil {
		return err
	}
	return nil
}

// 2. Получить задачу по ID. Возвращает указатель на задачу и ошибку.
func (r *PostgresRepository) GetByID(ctx context.Context, id int) (*Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	query := `
		SELECT t.id, t.user_id, t.assigned_to, t.title, t.done, t.priority,
		       s.id, s.task_id, s.title, s.done
		FROM tasks t
		LEFT JOIN subtasks s ON t.id = s.task_id
		WHERE t.id = $1`

	rows, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var task *Task
	for rows.Next() {
		// Создаем временные переменные для полей подзадачи на случай, если у задачи нет подзадач.
		// Зачем: если у задачи НЕТ подзадач, LEFT JOIN вернет в этих полях NULL.
		// Обычные типы int и string упадут с ошибкой при сканировании NULL.
		var sID sql.NullInt64
		var sTaskID sql.NullInt64
		var sTitle sql.NullString
		var sDone sql.NullBool

		var t Task

		err := rows.Scan(
			&t.ID, &t.UserID, &t.AssignedTo, &t.Title, &t.Done, &t.Priority,
			&sID, &sTaskID, &sTitle, &sDone,
		)
		if err != nil {
			return nil, err
		}

		// Если это САМАЯ ПЕРВАЯ строка, инициализируем наш итоговый объект task.
		// Во всех следующих строках данные задачи будут дублироваться, мы их просто игнорируем.
		if task == nil {
			task = &t
			// Инициализируем слайс подзадач, чтобы фронтенд получил [] вместо null, если чек-лист пуст
			task.SubTasks = make([]SubTask, 0)
		}

		// Проверяем: если sID.Valid == true, значит в этой строке РЕАЛЬНО есть подзадача (не NULL)
		if sID.Valid {
			sub := SubTask{
				ID:     int(sID.Int64),
				TaskID: int(sTaskID.Int64),
				Title:  sTitle.String,
				Done:   sDone.Bool,
			}
			// Добавляем подзадачу в слайс нашей основной задачи
			task.SubTasks = append(task.SubTasks, sub)
		}
	}

	// Проверяем, не прервался ли цикл из-за ошибки базы
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Если после цикла переменная task осталась nil — значит, база не вернула ни одной строки (задача не найдена)
	if task == nil {
		return nil, ErrTaskNotFound
	}

	return task, nil
}

// 3. Получить все задачи. Возвращает слайс.
func (r *PostgresRepository) GetAll(ctx context.Context, userID int) ([]Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Выбираем задачи семьи вместе со всеми их подзадачами через LEFT JOIN
	query := `
		SELECT t.id, t.user_id, t.assigned_to, t.title, t.done, t.priority,
		       s.id, s.task_id, s.title, s.done
		FROM tasks t
		LEFT JOIN subtasks s ON t.id = s.task_id
		ORDER BY t.id DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Используем карту (map), чтобы склеивать строки подзадач с нужной задачей по её ID
	taskMap := make(map[int]*Task)
	var taskOrder []int // Чтобы сохранить правильный порядок сортировки задач

	for rows.Next() {
		var t Task
		var sID, sTaskID sql.NullInt64
		var sTitle sql.NullString
		var sDone sql.NullBool

		err := rows.Scan(
			&t.ID, &t.UserID, &t.AssignedTo, &t.Title, &t.Done, &t.Priority,
			&sID, &sTaskID, &sTitle, &sDone,
		)
		if err != nil {
			return nil, err
		}

		// Если такой задачи еще нет в карте, добавляем её
		if _, exists := taskMap[t.ID]; !exists {
			t.SubTasks = make([]SubTask, 0) // Инициализируем слайс, чтобы в JSON не было null
			taskMap[t.ID] = &t
			taskOrder = append(taskOrder, t.ID)
		}

		// Если в этой строке прилетела реальная подзадача, добавляем её к родителю
		if sID.Valid {
			sub := SubTask{
				ID:     int(sID.Int64),
				TaskID: int(sTaskID.Int64),
				Title:  sTitle.String,
				Done:   sDone.Bool,
			}
			taskMap[t.ID].SubTasks = append(taskMap[t.ID].SubTasks, sub)
		}
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Переводим нашу карту обратно в плоский слайс для отправки фронтенду
	tasks := make([]Task, 0, len(taskMap))
	for _, id := range taskOrder {
		tasks = append(tasks, *taskMap[id])
	}

	return tasks, nil
}

// 4. Обновить задачу.
func (r *PostgresRepository) Update(ctx context.Context, task *Task, userID int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	query := "UPDATE tasks SET title=$1, done=$2, priority=$3, assigned_to=$4 WHERE id = $5"
	result, err := r.db.ExecContext(ctx, query, task.Title, task.Done, task.Priority, task.AssignedTo, task.ID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrTaskNotFound
	}

	return nil
}

// 5. Удалить задачу по ID.
func (r *PostgresRepository) Delete(ctx context.Context, id int, userID int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	query := "DELETE FROM tasks WHERE id = $1"
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrTaskNotFound
	}

	return nil
}

// Добавить нового пользователя
func (r *PostgresRepository) CreateUser(ctx context.Context, user *User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	query := "INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id"
	return r.db.QueryRowContext(ctx, query, user.Email, user.PasswordHash).Scan(&user.ID)
}

// Найти пользователя по email
func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var u User
	query := "SELECT id, email, password_hash FROM users WHERE email = $1"
	err := r.db.QueryRowContext(ctx, query, email).Scan(&u.ID, &u.Email, &u.PasswordHash)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

// Создать подзадачу
func (r *PostgresRepository) CreateSubtask(ctx context.Context, subtask *SubTask) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := r.db.QueryRowContext(ctx, "INSERT INTO subtasks (task_id, title, done) VALUES ($1, $2, $3) RETURNING id",
		subtask.TaskID, subtask.Title, subtask.Done).Scan(&subtask.ID)

	if err != nil {
		return err
	}
	return nil

}

// GetAllUsers возвращает список всех зарегистрированных пользователей системы.
func (r *PostgresRepository) GetAllUsers(ctx context.Context) ([]User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Запрашиваем только ID и Email, хэши паролей фронтенду знать нельзя
	rows, err := r.db.QueryContext(ctx, "SELECT id, email FROM users ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		// Сканируем только два поля
		err := rows.Scan(&u.ID, &u.Email)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}
