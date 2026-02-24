package main

import (
	"context" // [CHANGE-CONTEXT]
	"errors"  // [CHANGE-CONTEXT]
	"log"     // [CHANGE-CONTEXT]
	"net"     // [CHANGE-CONTEXT]
	"net/http"
	"os"        // [CHANGE-CONTEXT]
	"os/signal" // [CHANGE-CONTEXT]
	"syscall"   // [CHANGE-CONTEXT]
	"time"      // [CHANGE-CONTEXT]

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
	// Создаем основной контекст приложения.
	// Его отмена должна "доезжать" до всех in-flight запросов
	// через http.Server.BaseContext.
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// Наш контекст, который отменяется при Ctrl+C / SIGTERM
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Инициализируем файловое хранилище.
	store := tasks.NewTaskStore("tasks.json")

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

	// [CHANGE-CONTEXT] Запускаем сервер через http.Server (а не http.ListenAndServe),
	// чтобы поддержать graceful shutdown + таймауты сервера.
	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,

		// [CHANGE-CONTEXT] Понятные таймауты сервера (без усложнений).
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,

		// [CHANGE-CONTEXT] Корневой контекст для всех соединений/запросов.
		// Если мы отменим appCtx при shutdown, r.Context() у in-flight запросов тоже отменится.
		BaseContext: func(net.Listener) context.Context {
			return appCtx
		},
	}

	ln, err := net.Listen("tcp", srv.Addr) // [CHANGE-CONTEXT]
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}

	log.Printf("Server running on %s", srv.Addr) // [CHANGE-CONTEXT]

	serverErrCh := make(chan error, 1) // [CHANGE-CONTEXT]
	go func() {
		err := srv.Serve(ln) // [CHANGE-CONTEXT]
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
			return
		}
		serverErrCh <- nil
	}()

	// [CHANGE-CONTEXT] Ждём либо сигнал, либо фатальную ошибку сервера.
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

	// [CHANGE-CONTEXT] ВАЖНО: отменяем корневой контекст приложения.
	// Это "протекает" сверху вниз: handler -> service -> store,
	// и позволяет in-flight запросам корректно завершиться по ctx.Done().
	appCancel()

	// [CHANGE-CONTEXT] Graceful shutdown с таймаутом.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Если graceful не успел -- закрываем жёстко.
		log.Printf("shutdown error: %v", err)
		_ = srv.Close()
	}

	log.Printf("server stopped")

}

// chiWithMiddleware навешивает базовые middleware на уже собранный роутер.
//
//	Вынесено в отдельную функцию, чтобы main был читаемым и "про запуск".
func chiWithMiddleware(h http.Handler) http.Handler {
	// Используем chi.Router, чтобы навесить middleware, не меняя роуты модуля.
	// Это позволяет internal/tasks оставаться независимым от общесервисных middleware.
	r := chi.NewRouter()

	// middleware.Logger и middleware.Recoverer.
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)

	// [CHANGE] подключаем кастомный логгер на весь сервис
	r.Use(middleware.LoggingMiddleware)

	r.Mount("/", h)
	return r
}
