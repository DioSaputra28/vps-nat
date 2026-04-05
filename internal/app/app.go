package app

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/adminops"
	"github.com/DioSaputra28/vps-nat/internal/config"
	"github.com/DioSaputra28/vps-nat/internal/database"
	httprouter "github.com/DioSaputra28/vps-nat/internal/http/router"
	incusclient "github.com/DioSaputra28/vps-nat/internal/incus"
	"github.com/DioSaputra28/vps-nat/internal/telegram"
	"gorm.io/gorm"
)

type App struct {
	Config config.Config
	DB     *gorm.DB
	Incus  *incusclient.Client
	Cache  *adminops.DashboardCache
	Alerts *adminops.AlertMonitor
	Admin  *adminops.Service
	Server *http.Server
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	db, err := database.New(cfg.Database)
	if err != nil {
		return nil, err
	}

	incus, err := incusclient.New(cfg.Incus)
	if err != nil {
		return nil, err
	}

	adminOpsRepo := adminops.NewRepository(db)
	var dashboardServer adminops.IncusServer
	if incus != nil {
		dashboardServer = incus.Server()
	}
	adminOpsCache := adminops.NewDashboardCache(dashboardServer, 30*time.Second)
	adminOpsService := adminops.NewService(adminOpsRepo, adminOpsCache)
	var alertNotifier adminops.AlertNotifier
	if notifier := telegram.NewTelegramAdminNotifier(cfg.Alerts.TelegramBotToken, cfg.Alerts.TelegramChatID); notifier != nil {
		alertNotifier = adminops.AlertNotifierFunc(func(ctx context.Context, message string) error {
			return notifier.NotifyProvisionFailure(ctx, message)
		})
	}
	alertMonitor := adminops.NewAlertMonitor(adminOpsRepo, dashboardServer, alertNotifier, adminops.AlertMonitorConfig{
		Interval:         30 * time.Second,
		ThresholdPercent: 95,
		Duration:         10 * time.Minute,
	})
	adminOpsCache.Start()
	alertMonitor.Start()

	router := httprouter.New(cfg, httprouter.Dependencies{
		DB:       db,
		Incus:    incus,
		AdminOps: adminOpsService,
	})

	server := &http.Server{
		Addr:              cfg.HTTP.Host + ":" + strconv.Itoa(cfg.HTTP.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &App{
		Config: cfg,
		DB:     db,
		Incus:  incus,
		Cache:  adminOpsCache,
		Alerts: alertMonitor,
		Admin:  adminOpsService,
		Server: server,
	}, nil
}

func (a *App) Run() error {
	err := a.Server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func (a *App) Close() error {
	if a.Cache != nil {
		a.Cache.Stop()
	}
	if a.Alerts != nil {
		a.Alerts.Stop()
	}

	if a.DB == nil {
		return nil
	}

	sqlDB, err := a.DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}
