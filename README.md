# Семейный Таск-Менеджер (Family Task Manager API)

Бэкенд-сервер упакован в Docker-контейнер и по умолчанию доступен по адресу: `http://localhost:8080`
Все запросы должны обязательно содержать заголовок: `Content-Type: application/json`

---

## 1. АВТОРИЗАЦИЯ (Открытая группа)

### Регистрация нового пользователя
* **URL:** `POST /api/v1/auth/register`
* **Тело запроса (JSON):**
```json
{
  "email": "user@family.com",
  "password": "StrongPassword123",
  "invite_code": "CheshikKesha"
}
```
* **Ответ (201 Created):** Пользователь успешно создан.

### Вход в систему (Получение токена)
* **URL:** `POST /api/v1/auth/login`
* **Тело запроса (JSON):**
```json
{
  "email": "user@family.com",
  "password": "StrongPassword123"
}
```
* **Ответ (200 OK):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

---

## 2. ЗАДАЧИ (Закрытая группа)

> **ВАЖНО:** Все запросы к методам ниже обязаны содержать заголовок авторизации:
> `Authorization: Bearer <ваш_токен>`

### Получить список всех задач семьи
* **URL:** `GET /api/v1/tasks`
* **Ответ (200 OK):** Возвращает массив задач, где текущий пользователь является автором или исполнителем.
```json
[
  {
    "id": 1,
    "user_id": 42,
    "assigned_to": 42,
    "title": "Купить продукты на рынке",
    "done": false,
    "priority": "high",
    "subtasks": []
  }
]
```

### Создать новую задачу
* **URL:** `POST /api/v1/tasks`
* **Тело запроса (JSON):** Поле `assigned_to` необязательное (если пустое — исполнителем станет сам создатель).
```json
{
  "title": "Починить кран в ванной",
  "assigned_to": 0,
  "done": false,
  "priority": "medium"
}
```
* **Ответ (201 Created):** Возвращает созданную задачу с присвоенным ID.

### Добавить пункт чек-листа (Подзадачу)
* **URL:** `POST /api/v1/tasks/{id}/subtasks` (где `{id}` — это ID большой задачи)
* **Тело запроса (JSON):**
```json
{
  "title": "Купить прокладку для смесителя"
}
```
* **Ответ (201 Created):** Возвращает созданную подзадачу.
```json
{
  "id": 12,
  "task_id": 5,
  "title": "Купить прокладку для смесителя",
  "done": false
}
```
