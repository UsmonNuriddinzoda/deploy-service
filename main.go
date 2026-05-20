package main

import (
	"context"
	"deploy-service/config"
	"deploy-service/db"
	"deploy-service/handler"
	"deploy-service/logger"
	"deploy-service/middleware"
	"deploy-service/registry"
	"deploy-service/runner"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	log := logger.New()

	cfg := config.Load()

	// Подключение к PostgreSQL
	conn, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database: %v", err)
	}
	defer conn.Close()
	log.Info("Connected to PostgreSQL")

	repo := db.NewServiceRepo(conn)
	reg := registry.New(repo)

	r := runner.New(cfg.ScriptTimeout)
	h := handler.NewHandler(log, r, reg, repo)

	mux := http.NewServeMux()

	// ── UI ───────────────────────────────────────────────────────
	// Страница логина — без авторизации
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/login.html")
	})
	// Обработчик логина/логаута
	mux.HandleFunc("/ui/login",  handler.LoginHandler(cfg.UIUsername, cfg.UIPassword))
	mux.HandleFunc("/ui/logout", handler.LogoutHandler())
	// Защищённый UI — только после входа
	mux.Handle("/", middleware.SessionAuth(http.FileServer(http.Dir("./static"))))

	// ── Сервисы (CRUD) ──────────────────────────────────────────
	// POST   /services           — создать сервис
	// GET    /services           — список всех сервисов
	// GET    /services/{name}    — получить сервис
	// PUT    /services/{name}    — обновить сервис
	// DELETE /services/{name}    — удалить сервис
	mux.Handle("/services", middleware.SessionOrTokenAuth(cfg.SecretToken, http.HandlerFunc(h.ServicesHandler)))
	mux.Handle("/services/", middleware.SessionOrTokenAuth(cfg.SecretToken, http.HandlerFunc(h.ServicesHandler)))

	// ── Деплой ──────────────────────────────────────────────────
	// POST /deploy/{service}          — запустить деплой (JSON-ответ целиком)
	// GET  /deploy/{service}/stream   — стриминг вывода построчно (SSE)
	mux.Handle("/deploy/", middleware.SessionOrTokenAuth(cfg.SecretToken, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/stream") {
			h.DeployStreamHandler(w, r)
		} else {
			h.DeployHandler(w, r)
		}
	})))

	// ── Health ──────────────────────────────────────────────────
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: cfg.ScriptTimeout + 30*time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("Deploy service started on :%s", cfg.Port)
		log.Info("Routes:")
		log.Info("  GET    /                  — web UI (protected)")
		log.Info("  GET    /login             — login page")
		log.Info("  POST   /ui/login          — authenticate")
		log.Info("  POST   /ui/logout         — logout")
		log.Info("  POST   /services           — create service")
		log.Info("  GET    /services           — list services")
		log.Info("  GET    /services/{name}    — get service")
		log.Info("  PUT    /services/{name}    — update service")
		log.Info("  DELETE /services/{name}    — delete service")
		log.Info("  POST   /deploy/{service}   — run deploy")
		log.Info("  GET    /health             — health check")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server error: %v", err)
		}
	}()

	<-quit
	log.Info("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Forced shutdown: %v", err)
	}
	log.Info("Server stopped")
}

