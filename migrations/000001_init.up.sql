-- 1. Создаем таблицу пользователей (независимая, родительская таблица)
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL
);


-- 2. Создаем таблицу задач (дочерняя таблица со связями)
CREATE TABLE IF NOT EXISTS tasks (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,     -- Идентификатор создателя (автора) задачи
    assigned_to INT NOT NULL, -- Идентификатор исполнителя (ответственного) задачи
    title VARCHAR(100) NOT NULL, 
    done BOOLEAN NOT NULL DEFAULT false,
    priority VARCHAR(10) NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high')),
    
    -- Констрейнт (связь) для Создателя задачи
    CONSTRAINT fk_task_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    
    -- Констрейнт (связь) для Исполнителя задачи
    CONSTRAINT fk_task_assignee FOREIGN KEY (assigned_to) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS subtasks (
    id SERIAL PRIMARY KEY,
    task_id INT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    title VARCHAR(100) NOT NULL,
    done BOOLEAN NOT NULL DEFAULT false,
    
    -- Указываем, что колонка task_id ссылается на id в таблице tasks
    CONSTRAINT fk_task_subtask FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

