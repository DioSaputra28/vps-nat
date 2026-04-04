package router

import (
	"github.com/DioSaputra28/vps-nat/internal/auth"
	"github.com/DioSaputra28/vps-nat/internal/config"
	"github.com/DioSaputra28/vps-nat/internal/containers"
	"github.com/DioSaputra28/vps-nat/internal/http/handlers"
	"github.com/DioSaputra28/vps-nat/internal/http/middleware"
	incusclient "github.com/DioSaputra28/vps-nat/internal/incus"
	"github.com/DioSaputra28/vps-nat/internal/packages"
	"github.com/DioSaputra28/vps-nat/internal/telegram"
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
	telegramRepository := telegram.NewRepository(deps.DB)
	telegramService := telegram.NewService(telegramRepository, deps.Incus)
	telegramService.ConfigurePurchase(
		telegram.NewIncusPurchaseProvisioner(deps.Incus, cfg.Incus.NetworkName),
		telegram.NewPakasirGateway(cfg.Pakasir.BaseURL, cfg.Pakasir.ProjectSlug, cfg.Pakasir.APIKey),
		telegram.NewTelegramAdminNotifier(cfg.Alerts.TelegramBotToken, cfg.Alerts.TelegramChatID),
	)
	telegramHandler := handlers.NewTelegramHandler(telegramService, cfg.Telegram.BotSecret)
	paymentWebhookHandler := handlers.NewPaymentWebhookHandler(telegramService)
	containerRepository := containers.NewRepository(deps.DB)
	containerService := containers.NewService(containerRepository, deps.Incus)
	containerHandler := handlers.NewContainerHandler(containerService)

	router.GET("/healthz", healthHandler.Get)
	router.GET("/health", healthHandler.Get)

	telegramRoutes := router.Group("/telegram")
	telegramRoutes.POST("/start", telegramHandler.Start)
	telegramRoutes.POST("/home", telegramHandler.Home)
	telegramRoutes.POST("/buy-vps", telegramHandler.BuyVPS)
	telegramRoutes.POST("/buy-vps/submit", telegramHandler.BuyVPSSubmit)
	telegramRoutes.POST("/buy-vps/status", telegramHandler.BuyVPSStatus)
	telegramRoutes.POST("/my-vps", telegramHandler.MyVPS)
	telegramRoutes.POST("/my-vps/detail", telegramHandler.MyVPSDetail)
	telegramRoutes.POST("/my-vps/start", telegramHandler.StartAction)
	telegramRoutes.POST("/my-vps/stop", telegramHandler.StopAction)
	telegramRoutes.POST("/my-vps/reboot", telegramHandler.RebootAction)
	telegramRoutes.POST("/my-vps/password/change", telegramHandler.ChangePassword)
	telegramRoutes.POST("/my-vps/ssh/reset", telegramHandler.ResetSSH)
	telegramRoutes.POST("/my-vps/renew/preview", telegramHandler.RenewPreview)
	telegramRoutes.POST("/my-vps/renew/submit", telegramHandler.RenewSubmit)
	telegramRoutes.POST("/my-vps/upgrade/options", telegramHandler.UpgradeOptions)
	telegramRoutes.POST("/my-vps/upgrade/preview", telegramHandler.UpgradePreview)
	telegramRoutes.POST("/my-vps/upgrade/submit", telegramHandler.UpgradeSubmit)
	telegramRoutes.POST("/my-vps/reinstall/options", telegramHandler.ReinstallOptions)
	telegramRoutes.POST("/my-vps/reinstall/preview", telegramHandler.ReinstallPreview)
	telegramRoutes.POST("/my-vps/reinstall/submit", telegramHandler.ReinstallSubmit)
	telegramRoutes.POST("/my-vps/domain/preview", telegramHandler.DomainPreview)
	telegramRoutes.POST("/my-vps/domain/submit", telegramHandler.DomainSubmit)
	telegramRoutes.POST("/my-vps/reconfig-ip/preview", telegramHandler.ReconfigIPPreview)
	telegramRoutes.POST("/my-vps/reconfig-ip/submit", telegramHandler.ReconfigIPSubmit)
	telegramRoutes.POST("/my-vps/transfer/preview", telegramHandler.TransferPreview)
	telegramRoutes.POST("/my-vps/transfer/submit", telegramHandler.TransferSubmit)
	telegramRoutes.POST("/my-vps/cancel/preview", telegramHandler.CancelPreview)
	telegramRoutes.POST("/my-vps/cancel/submit", telegramHandler.CancelSubmit)

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

	paymentRoutes := router.Group("/payments")
	paymentRoutes.POST("/pakasir/webhook", paymentWebhookHandler.Pakasir)

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
