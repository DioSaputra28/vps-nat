package app

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/config"
	"github.com/DioSaputra28/vps-nat/internal/database"
	httprouter "github.com/DioSaputra28/vps-nat/internal/http/router"
	incusclient "github.com/DioSaputra28/vps-nat/internal/incus"
	"gorm.io/gorm"
)

type App struct {
	Config config.Config
	DB     *gorm.DB
	Incus  *incusclient.Client
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

	router := httprouter.New(cfg, httprouter.Dependencies{
		DB:    db,
		Incus: incus,
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
	if a.DB == nil {
		return nil
	}

	sqlDB, err := a.DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}
