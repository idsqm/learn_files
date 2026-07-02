package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andruho/files/internal/config"
	"github.com/andruho/files/internal/handler"
	"github.com/andruho/files/internal/repository"
	"github.com/andruho/files/internal/service"
	"github.com/andruho/files/internal/storage"
)

const cleanupInterval = 15 * time.Minute

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logLevel := slog.LevelInfo
	if cfg.Debug {
		logLevel = slog.LevelDebug
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	handler.SetLogger(log, cfg.Debug)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := connectWithRetry(ctx, cfg.DBURL, 10, log)
	if err != nil {
		log.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := runMigrations(ctx, pool); err != nil {
		log.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	store := storage.New(storage.Config{
		Endpoint:       cfg.S3Endpoint,
		PublicEndpoint: cfg.S3PublicEndpoint,
		Region:         cfg.S3Region,
		Bucket:         cfg.S3Bucket,
		AccessKey:      cfg.S3AccessKey,
		SecretKey:      cfg.S3SecretKey,
		UsePathStyle:   cfg.S3UsePathStyle,
	})

	files := repository.NewFileRepository(pool)
	fileSvc := service.NewFileService(files, store, cfg.PresignedTTL, cfg.PendingFileTTL, log)

	go service.RunCleanupLoop(ctx, fileSvc, cleanupInterval, log)

	router := handler.NewRouter(fileSvc, cfg.JWTSecret, cfg.CORSOrigins)

	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("server starting", "port", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	log.Info("server stopped")
}

func connectWithRetry(ctx context.Context, dbURL string, maxRetries int, log *slog.Logger) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error

	for i := 0; i < maxRetries; i++ {
		pool, err = pgxpool.New(ctx, dbURL)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				return pool, nil
			}
			pool.Close()
		}
		log.Warn("failed to connect to postgres, retrying", "attempt", i+1, "error", err)
		time.Sleep(2 * time.Second)
	}

	return nil, err
}

func gooseUp(raw string) string {
	if i := strings.Index(raw, "-- +goose Down"); i != -1 {
		return raw[:i]
	}
	return raw
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	for _, path := range []string{
		"migrations/20260630001_init.sql",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, gooseUp(string(raw))); err != nil {
			return err
		}
	}
	return nil
}
