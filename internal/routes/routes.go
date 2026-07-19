package routes

import (
	"github.com/KShekhurin/blog-go/config"
	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/db"
	"github.com/KShekhurin/blog-go/internal/handles"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/services"

	"github.com/gin-gonic/gin"
)

func CreateRouter(databaseConnect *db.Database, cfg *config.Config) *gin.Engine {
	router := gin.Default()

	query := database.New(databaseConnect.Db)

	userRepo := repositories.NewUserRepository(query)
	userService := services.NewUserService(userRepo, cfg.HashParams)
	authService := services.NewAuthService(userRepo, cfg.PrivateKey, cfg.SignMethod)

	userHandler := handles.NewAuthHandler(userService, authService)

	userGroup := router.Group("/api/v1")
	{
		userGroup.POST("/register", userHandler.Register)
		userGroup.POST("/login", userHandler.Login)
	}

	return router
}
