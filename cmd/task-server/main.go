package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// Импорт пакета config: Подключаем наш новый модуль настроек
	"task-manager/internal/config"
	"task-manager/internal/middleware" // Подключаем наш пакет middleware
	"task-manager/internal/tasks"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware" // Алиас, чтобы не конфликтовать с internal/middleware
)

// Здесь только:
// - создание зависимостей;
// - настройка middleware;
// - запуск HTTP-сервера.
func main() {
	// Инициализация конфига: Читаем переменные окружения при старте
	cfg := config.Load()

	// Создаем основной контекст приложения.
	// Его отмена должна "доезжать" до всех in-flight запросов
	// через http.Server.BaseContext.
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// Наш контекст, который отменяется при Ctrl+C / SIGTERM
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Инициализируем файловое хранилище.
	// Использование конфига: передаем путь до файла БД из настроек, а не хардкодом
	store := tasks.NewTaskStore(cfg.StoragePath)

	// Инициализируем сервис (слой business logic) и грузим данные с учетом контекста.
	svc, err := tasks.NewService(appCtx, store)
	if err != nil {
		log.Fatalf("service init error: %v\n", err) // Логирование - ответственность main
	}

	// Инициализируем HTTP-обработчики задач.
	handler := tasks.NewHandler(svc)

	// Собираем роутер.
	// Роуты переехали в internal/tasks (HTTP-слой), main только подключает.
	r := chiWithMiddleware(handler.Router())

	// Запускаем сервер через http.Server (а не http.ListenAndServe),
	// чтобы поддержать graceful shutdown + таймауты сервера.
	srv := &http.Server{
		// Использование конфига: подставляем порт из переменных окружения
		Addr:    ":" + cfg.Port,
		Handler: r,

		// Понятные таймауты сервера (без усложнений).
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,

		// Корневой контекст для всех соединений/запросов.
		// Если мы отменим appCtx при shutdown, r.Context() у in-flight запросов тоже отменится.
		BaseContext: func(net.Listener) context.Context {
			return appCtx
		},
	}

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}

	// Логирование конфига: визуализируем настройки для удобства DevOps
	log.Printf("Server running on port %s (Storage %s)", cfg.Port, cfg.StoragePath)

	serverErrCh := make(chan error, 1)
	go func() {
		err := srv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
			return
		}
		serverErrCh <- nil
	}()

	// Ждём либо сигнал, либо фатальную ошибку сервера.
	select {
	case <-sigCtx.Done():
		log.Printf("shutdown signal received")
	case err := <-serverErrCh:
		if err != nil {
			log.Printf("server error: %v", err)
		}
		// Если сервер неожиданно остановился без ошибки -- просто выходим.
		if err == nil {
			return
		}
	}

	// ВАЖНО: отменяем корневой контекст приложения.
	// Это "протекает" сверху вниз: handler -> service -> store,
	// и позволяет in-flight запросам корректно завершиться по ctx.Done().
	appCancel()

	// Graceful shutdown с таймаутом.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Если graceful не успел -- закрываем жёстко.
		log.Printf("shutdown error: %v", err)
		_ = srv.Close()
	}

	log.Printf("server stopped")

}

// !У нас получился не классический Graceful shutdown
// Для это стоило сделать так:
// 1. Сначала сигнализируем серверу о завершении (он перестанет принимать новые запросы)
/*if err := srv.Shutdown(shutdownCtx); err != nil {
    log.Printf("shutdown error: %v", err)
}

// 2. И только потом отменяем корневой контекст, если кто-то еще его слушает
appCancel()*/
// В нашем случае получается приоритет на скорость и предсказуемость
// Но есть вероятность того, что некоторые процессы, которые могли бы успешно завершиться, будут прерваны
// Уточняю, потому что мы сделали полезно для учебных целей
// Но в реальной практике могут быть другие приоритеты

// chiWithMiddleware навешивает базовые middleware на уже собранный роутер.
//
//	Вынесено в отдельную функцию, чтобы main был читаемым и "про запуск".
func chiWithMiddleware(h http.Handler) http.Handler {
	// Используем chi.Router, чтобы навесить middleware, не меняя роуты модуля.
	// Это позволяет internal/tasks оставаться независимым от общесервисных middleware.
	r := chi.NewRouter()

	// middleware.Logger и middleware.Recoverer.
	// r.Use(chiMiddleware.Logger)
	// r.Use(chiMiddleware.Recoverer)

	// Подключаем кастомный логгер на весь сервис
	// r.Use(middleware.LoggingMiddleware)

	// request-id должен быть доступен всем нижним слоям и логам (проброс через context + header)
	r.Use(middleware.RequestIDMiddleware)

	// Recoverer ставим "внутрь" логгера, чтобы паника превращалась в 500 ДО логирования статуса
	r.Use(chiMiddleware.Recoverer)

	r.Mount("/", h)
	return r
}
