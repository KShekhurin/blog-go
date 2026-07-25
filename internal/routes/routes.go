package routes

import (
	"github.com/KShekhurin/blog-go/config"
	_ "github.com/KShekhurin/blog-go/docs"
	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/db"
	"github.com/KShekhurin/blog-go/internal/handles"
	"github.com/KShekhurin/blog-go/internal/middleware"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/services"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

//	@title			Blog API
//	@version		1.0.0
//	@description	This is the example of a twitter-like blog written on go

// @host						localhost:9090
// @BasePath					/api/v1
// @securityDefinitions.apikey	Bearer
// @in							header
// @name						Authorization
// @description				Type "Bearer" followed by a space and JWT token.
func CreateRouter(databaseConnect *db.Database, cfg *config.Config) *gin.Engine {
	router := gin.Default()

	query := database.New(databaseConnect.Db)

	userRepo := repositories.NewUserRepository(query)
	postRepo := repositories.NewPostRepository(databaseConnect.Db)
	tokenRepo := repositories.NewTokenRepository(query)

	userService := services.NewUserService(userRepo, cfg.HashParams)
	authService := services.NewAuthService(userRepo, tokenRepo, cfg.PrivateKey, cfg.SignMethod)
	postService := services.NewPostService(postRepo)

	authHandle := handles.NewAuthHandler(userService, authService)
	postHandle := handles.NewPostsHandle(postService)
	userHandle := handles.NewUserHandler(userService)

	jwtMiddleware := middleware.NewJwtMiddleware(cfg.PublicKey)

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := router.Group("/api/v1")
	v1.Use(middleware.ErrorMiddleware())
	{
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", authHandle.Register)
			authGroup.POST("/login", authHandle.Login)
			authGroup.POST("/refresh", jwtMiddleware.PassRefresh, authHandle.Refresh)
			authGroup.POST("/logout", jwtMiddleware.PassRefresh, authHandle.Logout)
		}

		postGroup := v1.Group("/post")
		{
			postGroup.POST("", jwtMiddleware.Pass, postHandle.SendPost)
		}

		userGroup := v1.Group("/user")
		{
			userGroup.GET("/:user_id/posts", postHandle.GetPostsByUserId)
			userGroup.POST("/:author_id/subscriptions", jwtMiddleware.Pass, userHandle.SubscribeTo)
			userGroup.DELETE("/:author_id/subscriptions", jwtMiddleware.Pass, userHandle.UnsubscribeFrom)
		}
	}
	return router
}
