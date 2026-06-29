# Review: Task Manager (REST API)

## Summary
Проект хорошо структурирован (main/handler/service/store), есть context и graceful shutdown.
Для релизной готовности не хватало консистентного HTTP-контракта (ошибки), request tracing, устойчивости к плохим запросам и краткой документации для интеграции.

## Findings

### Major: Неконсистентный формат ошибок
- Where: internal/tasks/handler.go (http.Error + JSON местами)
- Risk: фронтенд/QA не могут стабильно парсить ошибки
- Fix: единый ErrorResponse JSON + helper WriteError
- Status: FIXED

### Major: Нет X-Request-ID
- Where: отсутствовал middleware
- Risk: сложно расследовать проблемы по логам
- Fix: RequestIDMiddleware, проброс/генерация, возврат в header
- Status: FIXED

### Major: Логи не содержат status code
- Where: internal/middleware/middleware.go (LoggingMiddleware)
- Risk: невозможно построить базовую наблюдаемость (сколько 500/400 и т.д.)
- Fix: response recorder
- Status: FIXED

### Major: Нет лимита request body
- Where: POST/PUT handlers
- Risk: DoS/случайные большие payload
- Fix: BodyLimitMiddleware (1 MiB) + обработка 413
- Status: FIXED

### Major: JSON decode не строгий (unknown fields, multiple JSON)
- Where: createTask/updateTask
- Risk: клиент может послать мусор, контракт “размывается”
- Fix: DisallowUnknownFields + проверка EOF
- Status: FIXED

### Major: PUT не обновлял priority
- Where: Service.UpdateTask
- Risk: контракт выглядит сломанным/неожиданным
- Fix: обновлять Priority тоже
- Status: FIXED

### Major: Запись в файл не защищена от частичной записи
- Where: TaskStore.SaveTasks (os.WriteFile)
- Risk: при сбое можно получить битый JSON
- Fix: temp file + rename (best-effort, с комментарием про Windows)
- Status: FIXED

### Minor: Ошибки чтения/парсинга файла не содержат контекста
- Where: TaskStore.LoadTasks
- Risk: сложнее дебажить
- Fix: fmt.Errorf(...) с filename
- Status: FIXED

### Minor: Нет “API for frontend” в README
- Risk: интеграция дороже по времени
- Fix: README API section + openapi.yaml
- Status: FIXED

### Nit: В тестах расширение файла test_db.jsn
- Where: internal/tasks/handler_test.go
- Risk: мелкая читаемость/опечатка
- Fix: можно переименовать в .json
- Status: NOT FIXED (не критично)
