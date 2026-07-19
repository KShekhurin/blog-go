package main

import (
	"fmt"
	"log"

	"github.com/KShekhurin/blog-go/config"
	"github.com/KShekhurin/blog-go/internal/db"
	"github.com/KShekhurin/blog-go/internal/routes"
	"github.com/KShekhurin/blog-go/internal/webModels"
)

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

	webModels.RegisterValidators()

	r := routes.CreateRouter(database, cfg)

	if err = r.Run(fmt.Sprintf(":%s", cfg.ServerPort)); err != nil {

		log.Fatal(err)
	}
}
