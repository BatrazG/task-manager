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
func (r *PostgresRepository) GetByID(ctx context.Context, id int, userID int) (*Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var t Task
	query := "SELECT id, user_id, assigned_to, title, done, priority FROM tasks WHERE id = $1 AND (user_id = $2 OR assigned_to = $2)"
	err := r.db.QueryRowContext(ctx, query, id, userID).Scan(&t.ID, &t.UserID, &t.AssignedTo, &t.Title, &t.Done, &t.Priority)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTaskNotFound
	}

	if err != nil {
		return nil, err
	}

	return &t, nil
}

// 3. Получить все задачи. Возвращает слайс.
func (r *PostgresRepository) GetAll(ctx context.Context, userID int) ([]Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, "SELECT id, user_id, assigned_to, title, done, priority FROM tasks WHERE user_id = $1 OR assigned_to = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		err := rows.Scan(&t.ID, &t.UserID, &t.AssignedTo, &t.Title, &t.Done, &t.Priority)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

// 4. Обновить задачу.
func (r *PostgresRepository) Update(ctx context.Context, task *Task, userID int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	query := "UPDATE tasks SET title=$1, done=$2, priority=$3 WHERE id = $4 AND (user_id = $5 OR assigned_to = $5)"
	result, err := r.db.ExecContext(ctx, query, task.Title, task.Done, task.Priority, task.ID, userID)
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

	query := "DELETE FROM tasks WHERE id = $1 AND (user_id = $2 OR assigned_to = $2)"
	result, err := r.db.ExecContext(ctx, query, id, userID)
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
