package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"task-team-api/internal/app"
	"task-team-api/internal/auth"
	"task-team-api/internal/cache"
	"task-team-api/internal/config"
	"task-team-api/internal/db"
	"task-team-api/internal/email"
	"task-team-api/internal/handler"
	"task-team-api/internal/repository"
	"task-team-api/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx := context.Background()
	mysqlDB, err := db.NewMySQL(ctx, cfg.MySQL)
	if err != nil {
		log.Fatalf("connect mysql: %v", err)
	}
	defer mysqlDB.Close()

	redisClient, err := cache.NewRedis(ctx, cfg.Redis)
	if err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer redisClient.Close()

	jwtManager := auth.NewManager(cfg.JWT.Secret, cfg.JWT.TTL)
	repo := repository.New(mysqlDB)
	emailClient := email.NewMockClient(cfg.Email.MockLatency)
	svc := service.New(repo, jwtManager, redisClient, cfg.Redis.TTL, emailClient)
	h := handler.New(svc)
	router := app.NewRouter(h, jwtManager, cfg.RateLimit.RequestsPerMinute)

	server := &http.Server{Addr: cfg.App.Address, Handler: router}

	go func() {
		log.Printf("task api started on %s", cfg.App.Address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
	defer cancel()
	log.Println("graceful shutdown started")
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("server stopped")
}
