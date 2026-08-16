package routes

import (
	"context"
	"time"

	"github.com/KShekhurin/blog-go/config"
	_ "github.com/KShekhurin/blog-go/docs"
	"github.com/KShekhurin/blog-go/internal/cache"
	"github.com/KShekhurin/blog-go/internal/database"
	"github.com/KShekhurin/blog-go/internal/db"
	"github.com/KShekhurin/blog-go/internal/handles"
	"github.com/KShekhurin/blog-go/internal/middleware"
	"github.com/KShekhurin/blog-go/internal/processors"
	"github.com/KShekhurin/blog-go/internal/repositories"
	"github.com/KShekhurin/blog-go/internal/services"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

//	@title			Blog API
//	@version		1.0.0
//	@description	This is the example of a twitter-like blog written on go

// @BasePath					/api/v1
// @securityDefinitions.apikey	Bearer
// @in							header
// @name						Authorization
// @description				Type "Bearer" followed by a space and JWT token.
func CreateRouter(databaseConnect *db.Database, cfg *config.Config) (*gin.Engine, func(context.Context)) {
	router := gin.Default()
	router.Use(otelgin.Middleware(""))

	query := database.New(databaseConnect.Db)

	feedCacher := cache.NewFeedCacher(
		databaseConnect.Cache,
	)

	userRepo := repositories.NewUserRepository(query)
	userRepoWrapper := repositories.NewUserCacheWrapper(
		userRepo,
		cache.NewSubsCacher(
			databaseConnect.Cache,
		),
	)

	postRepoWrapper := repositories.NewPostCacheWrapper(
		repositories.NewPostRepository(databaseConnect.Db),
		cache.NewPostCacher(
			databaseConnect.Cache,
			&cache.CacherParams{
				TTL: 10 * 24 * time.Hour,
			},
		),
	)
	tokenRepo := repositories.NewTokenRepository(query)

	userService := services.NewUserService(userRepoWrapper, cfg.HashParams)
	authService := services.NewAuthService(userRepoWrapper, tokenRepo, cfg.PrivateKey, cfg.SignMethod)
	postService := services.NewPostService(postRepoWrapper)
	feedService := services.NewFeedService(
		feedCacher,
		userRepo,
		postRepoWrapper,
	)

	postEventsProcessor := processors.NewPostEventsProcessor(
		query,
		feedCacher,
		userRepo,
		50,
	)
	postEventsProcessor.Start()

	authHandle := handles.NewAuthHandler(userService, authService)
	postHandle := handles.NewPostsHandle(postService, feedService)
	userHandle := handles.NewUserHandler(userService)
	feedHandle := handles.NewFeedHandle(feedService)

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
			postGroup.DELETE(":post_id", jwtMiddleware.Pass, postHandle.DeletePostById)
		}

		feedGroup := v1.Group("/feed")
		{
			feedGroup.GET("", jwtMiddleware.Pass, feedHandle.GetPosts)
		}

		userGroup := v1.Group("/user")
		{
			userGroup.GET("/:user_id/posts", postHandle.GetPostsByUserId)
			userGroup.POST("/:author_id/subs", jwtMiddleware.Pass, userHandle.SubscribeTo)
			userGroup.DELETE("/:author_id/subs", jwtMiddleware.Pass, userHandle.UnsubscribeFrom)
		}
	}
	return router, postEventsProcessor.Close
}
