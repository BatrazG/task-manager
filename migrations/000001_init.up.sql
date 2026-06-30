CREATE TABLE IF NOT EXISTS tasks (
    id SERIAL PRIMARY KEY,
    title VARCHAR(100) NOT NULL,
    done BOOLEAN NOT NULL DEFAULT false,
    priority VARCHAR(10) NOT NULL CHECK (priority IN ('low', 'medium', 'high'))
);
