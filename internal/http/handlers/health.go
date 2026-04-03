package handlers

import (
	"net/http"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/config"
	"github.com/DioSaputra28/vps-nat/internal/http/response"
	incusclient "github.com/DioSaputra28/vps-nat/internal/incus"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HealthHandler struct {
	config config.Config
	db     *gorm.DB
	incus  *incusclient.Client
}

func NewHealthHandler(cfg config.Config, db *gorm.DB, incus *incusclient.Client) HealthHandler {
	return HealthHandler{
		config: cfg,
		db:     db,
		incus:  incus,
	}
}

func (h HealthHandler) Get(c *gin.Context) {
	dbStatus := "connected"
	if h.db == nil {
		dbStatus = "disconnected"
	}

	response.Success(c, http.StatusOK, "health check successful", gin.H{
		"status":    "ok",
		"service":   h.config.App.Name,
		"env":       h.config.App.Env,
		"database":  dbStatus,
		"incus":     h.incusStatus(),
		"timestamp": time.Now().UTC(),
	})
}

func (h HealthHandler) incusStatus() gin.H {
	if h.incus == nil {
		return gin.H{
			"enabled": false,
		}
	}

	return gin.H{
		"enabled": true,
		"mode":    h.incus.Mode(),
	}
}
