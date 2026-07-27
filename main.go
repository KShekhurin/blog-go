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
	"github.com/KShekhurin/blog-go/internal/webModels"
	"github.com/gin-gonic/gin"
)

func launchServer(cfg *config.Config, r *gin.Engine) {
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

	go func() {
		log.Printf("[INFO] server was launched on %s:%s", address, cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[FATAL] fatal error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[INFO] termination signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("[FATAL] fatal error: %v", err)
	}

	log.Println("[INFO] server gracefully stopped.")
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	database, err := db.Load(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	if cfg.IsRelease == "1" {
		gin.SetMode(gin.ReleaseMode)
	}

	webModels.RegisterValidators()

	r := routes.CreateRouter(database, cfg)

	launchServer(cfg, r)
}
