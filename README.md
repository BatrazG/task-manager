# Task Manager (Go) — учебный REST API

Учебный проект для отработки тем Web/HTTP на Go: роутинг, CRUD, middleware, context, graceful shutdown.

## Run

```bash
go run ./cmd/task-server

Config (ENV)
HTTP_PORT — порт (default: 8080)
STORAGE_PATH — путь к файлу (default: tasks.json)
API (for frontend)
Base URL
http://localhost:8080
Request ID
Сервис поддерживает X-Request-ID:
если клиент прислал X-Request-ID — сервис пробросит его дальше и вернёт в ответе
если нет — сервис сгенерирует и вернёт
Error format (единый)
Любая ошибка возвращается в JSON:
{
  "error": {
    "code": "validation_error",
    "message": "Validation failed",
    "request_id": "demo-req-123",
    "details": [
      {"field": "Priority", "rule": "oneof"}
    ]
  }
}

Endpoints
GET /api/v1/tasks
Опционально: ?delay=200ms или ?delay=2s (учебная симуляция медленного I/O)
Response 200:
[
  {"id":1,"title":"Write Unit Tests","done":false,"priority":"high"}
]

POST /api/v1/tasks
Request:
{"title":"Write Unit Tests","done":false,"priority":"high"}

Response 201 (header Location: /api/v1/tasks/{id}):
{"id":1,"title":"Write Unit Tests","done":false,"priority":"high"}

GET /api/v1/tasks/{id}
Response 200:
{"id":1,"title":"Write Unit Tests","done":false,"priority":"high"}

Response 404 (если нет задачи)
PUT /api/v1/tasks/{id}
Request:
{"title":"Updated","done":true,"priority":"low"}

Response 200:
{"id":1,"title":"Updated","done":true,"priority":"low"}

DELETE /api/v1/tasks/{id}
Защищено Basic Auth:
username: admin
password: secret
Response 204 (no content)
Manual testing
См. набор cURL команд в материалах урока / или используйте openapi.yaml.
Tests
go test ./... -v


