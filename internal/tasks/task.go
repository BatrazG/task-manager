package tasks

// Task — модель задачи.
//
// Хранится в памяти (для скорости) и сериализуется в JSON (для API и файла).
type Task struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
	// [CHANGE-VALIDATION]: [Добавлено поле приоритета в модель данных]
	Priority string `json:"priority"`
}

// [CHANGE-VALIDATION]: [Вводим DTO структуру для защиты входных данных (теги validate)]
// CreateRaskRequest описывает контракт входящего JSON для создания задачи
type CreateRaskRequest struct {
	Title string `json:"title" validate:"required,max=100"`
	// В данном случае поле не обязательное - это bool
	//Состояние задачи по умолчанию станет false
	// Это норм, делаем для того, чтобы вы увидели, как быть с невалидируемыми полями
	Done     bool   `json:"done"` // Не валидируем.
	Priority string `json:"priority" validate:"required,oneof=low medium high"`
}

type UpdateTaskRequest struct {
	// [CHANGE-VALIDATION]: [Добавлен тег omitempty, чтобы при отсутствии поля валидатор не ругался на max=100]
	Title *string `json:"title,omitempty" validate:"omitempty,max=100"`
	Done  *bool   `json:"done,omitempty"`
	// [CHANGE-VALIDATION]: [Добавлен указатель на поле Priority для поддержки частичного обновления с тегами валидации]
	Priority *string `json:"priority,omitempty" validate:"omitempty,oneof=low medium high"`
}
