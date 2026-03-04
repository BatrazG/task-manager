package tasks

// Task — модель задачи.
//
// Хранится в памяти (для скорости) и сериализуется в JSON (для API и файла).
type Task struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Priority string `json:"priority"` // [Validation] - добавили приоьритет в модель хранения и API
}

type CreateTaskRequest struct {
	Title    string `json:"title" validate:"required,max=100"` // [Валидация] правила входного контракта живут в DTO, а не в Task
	Done     bool   `json:"done"`
	Priority string `json:"priority" validate:"required,oneof=low medium high"`
}
