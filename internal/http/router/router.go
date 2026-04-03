package router

import (
	"github.com/DioSaputra28/vps-nat/internal/auth"
	"github.com/DioSaputra28/vps-nat/internal/config"
	"github.com/DioSaputra28/vps-nat/internal/containers"
	"github.com/DioSaputra28/vps-nat/internal/http/handlers"
	"github.com/DioSaputra28/vps-nat/internal/http/middleware"
	incusclient "github.com/DioSaputra28/vps-nat/internal/incus"
	"github.com/DioSaputra28/vps-nat/internal/packages"
	"github.com/DioSaputra28/vps-nat/internal/users"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB    *gorm.DB
	Incus *incusclient.Client
}

func New(cfg config.Config, deps Dependencies) *gin.Engine {
	setGinMode(cfg.App.Env)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	healthHandler := handlers.NewHealthHandler(cfg, deps.DB, deps.Incus)
	authService := auth.NewService(deps.DB, cfg.Auth)
	adminAuthHandler := handlers.NewAdminAuthHandler(authService)
	adminAuthMiddleware := middleware.NewAdminAuth(authService)
	packageRepository := packages.NewRepository(deps.DB)
	packageService := packages.NewService(packageRepository)
	packageHandler := handlers.NewPackageHandler(packageService)
	userRepository := users.NewRepository(deps.DB)
	userService := users.NewService(userRepository)
	userHandler := handlers.NewUserHandler(userService)
	containerRepository := containers.NewRepository(deps.DB)
	containerService := containers.NewService(containerRepository, deps.Incus)
	containerHandler := handlers.NewContainerHandler(containerService)

	router.GET("/healthz", healthHandler.Get)
	router.GET("/health", healthHandler.Get)

	authRoutes := router.Group("/auth")
	authRoutes.POST("/login", adminAuthHandler.Login)

	protectedAuthRoutes := router.Group("/auth")
	protectedAuthRoutes.Use(adminAuthMiddleware.Require())
	protectedAuthRoutes.GET("/me", adminAuthHandler.Me)
	protectedAuthRoutes.POST("/logout", adminAuthHandler.Logout)

	packageRoutes := router.Group("/packages")
	packageRoutes.Use(adminAuthMiddleware.Require())
	packageRoutes.POST("", packageHandler.Create)
	packageRoutes.GET("", packageHandler.List)
	packageRoutes.GET("/:id", packageHandler.GetByID)
	packageRoutes.PATCH("/:id", packageHandler.Update)
	packageRoutes.DELETE("/:id", packageHandler.Delete)

	userRoutes := router.Group("/users")
	userRoutes.Use(adminAuthMiddleware.Require())
	userRoutes.GET("", userHandler.List)
	userRoutes.GET("/:id", userHandler.GetByID)

	containerRoutes := router.Group("/containers")
	containerRoutes.Use(adminAuthMiddleware.Require())
	containerRoutes.GET("", containerHandler.List)
	containerRoutes.GET("/:id", containerHandler.GetByID)
	containerRoutes.POST("/:id/actions/start", containerHandler.Start)
	containerRoutes.POST("/:id/actions/stop", containerHandler.Stop)
	containerRoutes.POST("/:id/actions/suspend", containerHandler.Suspend)

	return router
}

func setGinMode(env string) {
	switch env {
	case "production":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}
}
