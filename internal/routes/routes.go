package routes

import (
	"github.com/KShekhurin/blog-go/config"
	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/db"
	"github.com/KShekhurin/blog-go/internal/handles"
	"github.com/KShekhurin/blog-go/internal/middleware"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/services"

	"github.com/gin-gonic/gin"
)

func CreateRouter(databaseConnect *db.Database, cfg *config.Config) *gin.Engine {
	router := gin.Default()

	query := database.New(databaseConnect.Db)

	userRepo := repositories.NewUserRepository(query)
	postRepo := repositories.NewPostRepository(databaseConnect.Db)

	userService := services.NewUserService(userRepo, cfg.HashParams)
	authService := services.NewAuthService(userRepo, cfg.PrivateKey, cfg.SignMethod)
	postService := services.NewPostService(postRepo)

	userHandle := handles.NewAuthHandler(userService, authService)
	postHandle := handles.NewPostsHandle(postService)

	jwtMiddleware := middleware.NewJwtMiddleware(cfg.PublicKey)

	v1 := router.Group("/api/v1")
	v1.Use(middleware.ErrorMiddleware())
	{
		v1.POST("/register", userHandle.Register)
		v1.POST("/login", userHandle.Login)

		postGroup := v1.Group("/post")
		{
			postGroup.POST("", jwtMiddleware.Pass, postHandle.SendPost)
		}
	}

	return router
}
