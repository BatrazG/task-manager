package tasks

// Task описывает доменную модель задачи в системе.
type Task struct {
	// ID — уникальный идентификатор задачи, автоматически генерируемый базой данных.
	ID int `json:"id"`

	// UserID — идентификатор пользователя (владельца), которому принадлежит эта задача.
	UserID int `json:"user_id"`

	// AssignedTo — идентификатор пользователя (исполнителя), который должен выполнить задачу.
	AssignedTo int `json:"assigned_to"`

	// Title — краткое описание или название задачи.
	Title string `json:"title"`

	// Done — флаг текущего состояния задачи (true — выполнена, false — в работе).
	Done bool `json:"done"`

	// Priority — уровень важности задачи (принимает значения: low, medium, high).
	Priority string `json:"priority"`

	// SubTasks - список подзадач(пунктов чек-листа), привязанных к этой задаче
	SubTasks []SubTask `json:"subtasks"`
}

// Subtask описывает доменную модель подзадачи в системе.
type SubTask struct {
	ID     int    `json:"id"`
	TaskID int    `json:"task_id"`
	Title  string `json:"title"`
	Done   bool   `json:"done"`
}

type CreateTaskRequest struct {
	Title      string `json:"title" validate:"required,max=100"` // [Валидация] правила входного контракта живут в DTO, а не в Task
	AssignedTo int    `json:"assigned_to"`
	Done       bool   `json:"done"`
	Priority   string `json:"priority" validate:"required,oneof=low medium high"`
}

// Отдельный DTO для PUT -- фиксируем контракт входных данных + включаем валидацию.
type UpdateTaskRequest struct {
	Title    string `json:"title" validate:"required,max=100"`
	Done     bool   `json:"done"`
	Priority string `json:"priority" validate:"required,oneof=low medium high"`
}

type CreateSubTaskRequest struct {
	Title string `json:"title" validate:"required,max=100"`
}

// Делать разные DTO для разных эндпоинтов - правильная практика
// Для обновления задачи можно не все поля сделать обязательными
// Не усложняем в рамках учебного проекта, в реальном стоило бы пересмотреть
// Например еще логично сделать Done *bool

// Хорошо делать со встраиванием, что-то вроде этого:
/*type BaseTask struct {
    Title    string `json:"title" validate:"required"`
    Priority string `json:"priority"`
}
type CreateTaskRequest struct { BaseTask }
type UpdateTaskRequest struct {
    BaseTask
    Done bool `json:"done"`
}*/
