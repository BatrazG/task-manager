// Модуль конфигурации: вынесли хардкод из main.go
// Теперь приложение можно настраивать через переменные окружения, не меняя код.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config содержит базовые настройки приложения
type Config struct {
	Port        string
	StoragePath string

	// Поля для SQL:
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
}

// DSN возвращает строку подключения к PostgreSQL.
func (cfg *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
}

// Load считывает конфигурацию.
// Сначала ставим дефолтные значения, затем перезаписываем их, если в ENV что-то есть.
func Load() *Config {
	cfg := &Config{
		Port:        "8080",
		StoragePath: "tasks.json",
		// Ставим разумные дефолты для Postgres на случай локального запуска:
		DBHost: "localhost",
		DBPort: 5432,
		DBUser: "postgres",
		DBName: "taskmanager",
	}

	if port := os.Getenv("HTTP_PORT"); port != "" {
		cfg.Port = port
	}

	if path := os.Getenv("STORAGE_PATH"); path != "" {
		cfg.StoragePath = path
	}

	// Считываем новые переменные для работы с PostgreSQL
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		cfg.DBHost = dbHost
	}

	if dbPortStr := os.Getenv("DB_PORT"); dbPortStr != "" {
		// Переводим строку в число
		if val, err := strconv.Atoi(dbPortStr); err == nil {
			cfg.DBPort = val
		}
	}

	if dbUser := os.Getenv("DB_USER"); dbUser != "" {
		cfg.DBUser = dbUser
	}

	if dbPassword := os.Getenv("DB_PASSWORD"); dbPassword != "" {
		cfg.DBPassword = dbPassword
	}

	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		cfg.DBName = dbName
	}

	return cfg
}
