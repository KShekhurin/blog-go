package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/KShekhurin/blog-go/config"
	"github.com/KShekhurin/blog-go/internal/db"
	"github.com/KShekhurin/blog-go/internal/routes"
	"github.com/KShekhurin/blog-go/internal/telemetry"
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/gin-gonic/gin"
)

func launchServer(cfg *config.Config, r *gin.Engine) (*http.Server, <-chan error) {
	address := cfg.ServerDevAddress

	if cfg.IsRelease == "1" {
		address = "0.0.0.0"
	}

	srv := &http.Server{
		Addr:           fmt.Sprintf("%s:%s", address, cfg.ServerPort),
		Handler:        r,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	srvErr := make(chan error, 1)

	go func() {
		log.Printf("[INFO] server was launched on %s:%s", address, cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErr <- err
		}
	}()

	return srv, srvErr
}

func closeServer(srv *http.Server) error {
	log.Println("[INFO] closing server..")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}
	return nil
}

func closeOTel(shutdown func(ctx context.Context) error) error {
	log.Println("[INFO] closing telemetry..")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := shutdown(ctx); err != nil {
		return fmt.Errorf("telemetry shutdown: %w", err)
	}
	return nil
}

func closeDb(database *db.Database) error {
	log.Println("[INFO] closing database..")

	if err := database.Close(); err != nil {
		return fmt.Errorf("database close: %w", err)
	}
	return nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	otelShutdown, err := telemetry.SetupOTelSDK(cfg)
	if err != nil {
		log.Fatal(err)
	}

	database, err := db.Load(cfg)
	if err != nil {
		_ = closeOTel(otelShutdown)
		log.Fatal(err)
	}

	if cfg.IsRelease == "1" {
		gin.SetMode(gin.ReleaseMode)
	}

	webModels.RegisterValidators()

	r := routes.CreateRouter(database, cfg)
	srv, srvErr := launchServer(cfg, r)

	var shutdownErr error

	select {
	case <-ctx.Done():
		stop()
	case err := <-srvErr:
		log.Printf("[ERROR] server died unexpectedly: %v", err)
		shutdownErr = err
	}

	if err := closeServer(srv); err != nil {
		log.Printf("[ERROR] %v", err)
		shutdownErr = err
	}

	if err := closeOTel(otelShutdown); err != nil {
		log.Printf("[ERROR] %v", err)
		shutdownErr = errors.Join(shutdownErr, err)
	}

	if err := closeDb(database); err != nil {
		log.Printf("[ERROR] %v", err)
		shutdownErr = errors.Join(shutdownErr, err)
	}

	if shutdownErr != nil {
		log.Printf("[WARN] shutdown completed with errors: %v", shutdownErr)
		os.Exit(1)
	}

	log.Println("[INFO] shutdown complete")
}
