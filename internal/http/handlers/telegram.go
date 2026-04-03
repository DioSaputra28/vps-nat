package handlers

import (
	"errors"
	"net/http"

	"github.com/DioSaputra28/vps-nat/internal/http/response"
	"github.com/DioSaputra28/vps-nat/internal/telegram"
	"github.com/gin-gonic/gin"
)

const telegramBotSecretHeader = "X-Telegram-Bot-Secret"

type TelegramHandler struct {
	service   *telegram.Service
	botSecret string
}

type telegramStartRequest struct {
	TelegramID       int64   `json:"telegram_id" binding:"required"`
	TelegramUsername *string `json:"telegram_username"`
	DisplayName      string  `json:"display_name"`
	FirstName        *string `json:"first_name"`
	LastName         *string `json:"last_name"`
}

func NewTelegramHandler(service *telegram.Service, botSecret string) TelegramHandler {
	return TelegramHandler{
		service:   service,
		botSecret: botSecret,
	}
}

func (h TelegramHandler) Start(c *gin.Context) {
	if h.botSecret != "" && c.GetHeader(telegramBotSecretHeader) != h.botSecret {
		response.Fail(c, http.StatusUnauthorized, "invalid telegram bot secret", "unauthorized", nil)
		return
	}

	var req telegramStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.SyncStart(c.Request.Context(), telegram.SyncStartInput{
		TelegramID:       req.TelegramID,
		TelegramUsername: req.TelegramUsername,
		DisplayName:      req.DisplayName,
		FirstName:        req.FirstName,
		LastName:         req.LastName,
	})
	if err != nil {
		switch {
		case errors.Is(err, telegram.ErrInvalidTelegramUser):
			response.Fail(c, http.StatusBadRequest, "invalid telegram user data", "bad_request", nil)
		default:
			response.Fail(c, http.StatusInternalServerError, "failed to sync telegram user", "internal_server_error", nil)
		}
		return
	}

	message := "telegram user synced successfully"
	if result.Created {
		message = "telegram user created successfully"
	}

	response.Success(c, http.StatusOK, message, gin.H{
		"user": gin.H{
			"id":                result.User.ID,
			"telegram_id":       result.User.TelegramID,
			"telegram_username": result.User.TelegramUsername,
			"display_name":      result.User.DisplayName,
			"status":            result.User.Status,
			"created_at":        result.User.CreatedAt,
			"updated_at":        result.User.UpdatedAt,
		},
		"wallet": gin.H{
			"id":         result.User.Wallet.ID,
			"balance":    result.User.Wallet.Balance,
			"created_at": result.User.Wallet.CreatedAt,
			"updated_at": result.User.Wallet.UpdatedAt,
		},
		"created": result.Created,
	})
}
