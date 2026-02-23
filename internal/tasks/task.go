package tasks

// Task — модель задачи.
//
// Хранится в памяти (для скорости) и сериализуется в JSON (для API и файла).
type Task struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}
