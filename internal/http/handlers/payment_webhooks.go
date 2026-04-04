package handlers

import (
	"errors"
	"net/http"

	"github.com/DioSaputra28/vps-nat/internal/http/response"
	"github.com/DioSaputra28/vps-nat/internal/telegram"
	"github.com/gin-gonic/gin"
)

type PaymentWebhookHandler struct {
	service *telegram.Service
}

func NewPaymentWebhookHandler(service *telegram.Service) PaymentWebhookHandler {
	return PaymentWebhookHandler{service: service}
}

func (h PaymentWebhookHandler) Pakasir(c *gin.Context) {
	var req telegram.PakasirWebhookInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	if err := h.service.HandlePakasirWebhook(c.Request.Context(), req); err != nil {
		switch {
		case errors.Is(err, telegram.ErrInvalidActionRequest), errors.Is(err, telegram.ErrPaymentVerification):
			response.Fail(c, http.StatusBadRequest, "invalid pakasir webhook", "bad_request", nil)
		case errors.Is(err, telegram.ErrOrderNotFound):
			response.Fail(c, http.StatusNotFound, "order not found", "not_found", nil)
		default:
			response.Fail(c, http.StatusInternalServerError, "failed to handle pakasir webhook", "internal_server_error", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, "pakasir webhook handled successfully", nil)
}
