package tasks

// Task — модель задачи.
//
// Хранится в памяти (для скорости) и сериализуется в JSON (для API и файла).
type Task struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Priority string `json:"priority"` // Добавили приоритет в модель хранения и API
}

type CreateTaskRequest struct {
	Title    string `json:"title" validate:"required,max=100"` // [Валидация] правила входного контракта живут в DTO, а не в Task
	Done     bool   `json:"done"`
	Priority string `json:"priority" validate:"required,oneof=low medium high"`
}

// Отдельный DTO для PUT -- фиксируем контракт входных данных + включаем валидацию.
type UpdateTaskRequest struct {
	Title    string `json:"title" validate:"required,max=100"`
	Done     bool   `json:"done"`
	Priority string `json:"priority" validate:"required,oneof=low medium high"`
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
