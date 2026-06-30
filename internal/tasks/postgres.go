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

	err := r.db.QueryRowContext(ctx, "INSERT INTO tasks (title, done, priority) VALUES ($1, $2, $3) RETURNING id",
		task.Title, task.Done, task.Priority).Scan(&task.ID)

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

	var t Task
	err := r.db.QueryRowContext(ctx, "SELECT id, title, done, priority FROM tasks WHERE id=$1", id).Scan(&t.ID, &t.Title, &t.Done, &t.Priority)

	if errors.Is(sql.ErrNoRows, err) {
		return nil, ErrTaskNotFound
	}

	if err != nil {
		return nil, err
	}

	return &t, nil
}

// 3. Получить все задачи. Возвращает слайс.
func (r *PostgresRepository) GetAll(ctx context.Context) ([]Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, "SELECT id, title, done, priority FROM tasks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		err := rows.Scan(&t.ID, &t.Title, &t.Done, &t.Priority)
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
func (r *PostgresRepository) Update(ctx context.Context, task *Task) error {
	return nil
}

// 5. Удалить задачу по ID.
func (r *PostgresRepository) Delete(ctx context.Context, id int) error {
	return nil
}
