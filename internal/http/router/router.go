package router

import (
	"github.com/DioSaputra28/vps-nat/internal/auth"
	"github.com/DioSaputra28/vps-nat/internal/config"
	"github.com/DioSaputra28/vps-nat/internal/http/handlers"
	"github.com/DioSaputra28/vps-nat/internal/http/middleware"
	incusclient "github.com/DioSaputra28/vps-nat/internal/incus"
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

	router.GET("/healthz", healthHandler.Get)
	router.GET("/health", healthHandler.Get)

	authRoutes := router.Group("/auth")
	authRoutes.POST("/login", adminAuthHandler.Login)

	protectedAuthRoutes := router.Group("/auth")
	protectedAuthRoutes.Use(adminAuthMiddleware.Require())
	protectedAuthRoutes.GET("/me", adminAuthHandler.Me)
	protectedAuthRoutes.POST("/logout", adminAuthHandler.Logout)

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
