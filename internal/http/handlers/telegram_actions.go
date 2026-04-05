package handlers

import (
	"errors"
	"net/http"

	"github.com/DioSaputra28/vps-nat/internal/http/response"
	"github.com/DioSaputra28/vps-nat/internal/telegram"
	"github.com/gin-gonic/gin"
)

type telegramContainerRequest struct {
	TelegramID  int64  `json:"telegram_id" binding:"required"`
	ContainerID string `json:"container_id" binding:"required"`
}

type telegramRuntimeActionRequest struct {
	TelegramID  int64  `json:"telegram_id" binding:"required"`
	ContainerID string `json:"container_id" binding:"required"`
}

type telegramChangePasswordRequest struct {
	TelegramID  int64  `json:"telegram_id" binding:"required"`
	ContainerID string `json:"container_id" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type telegramRenewSubmitRequest struct {
	TelegramID    int64  `json:"telegram_id" binding:"required"`
	ContainerID   string `json:"container_id" binding:"required"`
	PaymentMethod string `json:"payment_method" binding:"required"`
}

type telegramUpgradePreviewRequest struct {
	TelegramID      int64  `json:"telegram_id" binding:"required"`
	ContainerID     string `json:"container_id" binding:"required"`
	TargetPackageID string `json:"target_package_id" binding:"required"`
}

type telegramUpgradeSubmitRequest struct {
	TelegramID      int64  `json:"telegram_id" binding:"required"`
	ContainerID     string `json:"container_id" binding:"required"`
	TargetPackageID string `json:"target_package_id" binding:"required"`
	PaymentMethod   string `json:"payment_method" binding:"required"`
}

type telegramReinstallRequest struct {
	TelegramID  int64  `json:"telegram_id" binding:"required"`
	ContainerID string `json:"container_id" binding:"required"`
	ImageAlias  string `json:"image_alias" binding:"required"`
}

type telegramDomainRequest struct {
	TelegramID  int64  `json:"telegram_id" binding:"required"`
	ContainerID string `json:"container_id" binding:"required"`
	Domain      string `json:"domain" binding:"required"`
	TargetPort  int    `json:"target_port" binding:"required"`
	ProxyMode   string `json:"proxy_mode" binding:"required"`
}

type telegramReconfigIPSubmitRequest struct {
	TelegramID       int64  `json:"telegram_id" binding:"required"`
	ContainerID      string `json:"container_id" binding:"required"`
	RequestedSSHPort *int   `json:"requested_ssh_port"`
}

type telegramTransferRequest struct {
	TelegramID       int64   `json:"telegram_id" binding:"required"`
	ContainerID      string  `json:"container_id" binding:"required"`
	TargetTelegramID int64   `json:"target_telegram_id" binding:"required"`
	Reason           *string `json:"reason"`
}

func (h TelegramHandler) StartAction(c *gin.Context) {
	h.runtimeAction(c, "start")
}

func (h TelegramHandler) StopAction(c *gin.Context) {
	h.runtimeAction(c, "stop")
}

func (h TelegramHandler) RebootAction(c *gin.Context) {
	h.runtimeAction(c, "reboot")
}

func (h TelegramHandler) ChangePassword(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.ChangePassword(c.Request.Context(), telegram.ChangePasswordInput{
		TelegramID:  req.TelegramID,
		ContainerID: req.ContainerID,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to change telegram my vps password")
		return
	}

	response.Success(c, http.StatusOK, "telegram my vps password changed successfully", result)
}

func (h TelegramHandler) ResetSSH(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.ResetSSH(c.Request.Context(), telegram.ResetSSHInput{
		TelegramID:  req.TelegramID,
		ContainerID: req.ContainerID,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to reset telegram my vps ssh access")
		return
	}

	response.Success(c, http.StatusOK, "telegram my vps ssh access reset successfully", result)
}

func (h TelegramHandler) RenewPreview(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.RenewPreview(c.Request.Context(), telegram.RenewPreviewInput{
		TelegramID:  req.TelegramID,
		ContainerID: req.ContainerID,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to fetch telegram renew preview")
		return
	}

	response.Success(c, http.StatusOK, "telegram renew preview fetched successfully", result)
}

func (h TelegramHandler) RenewSubmit(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramRenewSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.RenewSubmit(c.Request.Context(), telegram.RenewSubmitInput{
		TelegramID:    req.TelegramID,
		ContainerID:   req.ContainerID,
		PaymentMethod: req.PaymentMethod,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to submit telegram renewal")
		return
	}

	response.Success(c, http.StatusOK, "telegram renewal submitted successfully", result)
}

func (h TelegramHandler) UpgradeOptions(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.UpgradeOptions(c.Request.Context(), telegram.UpgradeOptionsInput{
		TelegramID:  req.TelegramID,
		ContainerID: req.ContainerID,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to fetch telegram upgrade options")
		return
	}

	response.Success(c, http.StatusOK, "telegram upgrade options fetched successfully", result)
}

func (h TelegramHandler) UpgradePreview(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramUpgradePreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.UpgradePreview(c.Request.Context(), telegram.UpgradePreviewInput{
		TelegramID:      req.TelegramID,
		ContainerID:     req.ContainerID,
		TargetPackageID: req.TargetPackageID,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to fetch telegram upgrade preview")
		return
	}

	response.Success(c, http.StatusOK, "telegram upgrade preview fetched successfully", result)
}

func (h TelegramHandler) UpgradeSubmit(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramUpgradeSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.UpgradeSubmit(c.Request.Context(), telegram.UpgradeSubmitInput{
		TelegramID:      req.TelegramID,
		ContainerID:     req.ContainerID,
		TargetPackageID: req.TargetPackageID,
		PaymentMethod:   req.PaymentMethod,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to submit telegram upgrade")
		return
	}

	response.Success(c, http.StatusOK, "telegram upgrade submitted successfully", result)
}

func (h TelegramHandler) ReinstallOptions(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.ReinstallOptions(c.Request.Context(), telegram.ReinstallOptionsInput{
		TelegramID:  req.TelegramID,
		ContainerID: req.ContainerID,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to fetch telegram reinstall options")
		return
	}

	response.Success(c, http.StatusOK, "telegram reinstall options fetched successfully", result)
}

func (h TelegramHandler) ReinstallPreview(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramReinstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.ReinstallPreview(c.Request.Context(), telegram.ReinstallPreviewInput{
		TelegramID:  req.TelegramID,
		ContainerID: req.ContainerID,
		ImageAlias:  req.ImageAlias,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to fetch telegram reinstall preview")
		return
	}

	response.Success(c, http.StatusOK, "telegram reinstall preview fetched successfully", result)
}

func (h TelegramHandler) ReinstallSubmit(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramReinstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.ReinstallSubmit(c.Request.Context(), telegram.ReinstallSubmitInput{
		TelegramID:  req.TelegramID,
		ContainerID: req.ContainerID,
		ImageAlias:  req.ImageAlias,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to submit telegram reinstall")
		return
	}

	response.Success(c, http.StatusOK, "telegram reinstall submitted successfully", result)
}

func (h TelegramHandler) DomainPreview(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.DomainPreview(c.Request.Context(), telegram.DomainPreviewInput{
		TelegramID:  req.TelegramID,
		ContainerID: req.ContainerID,
		Domain:      req.Domain,
		TargetPort:  req.TargetPort,
		ProxyMode:   req.ProxyMode,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to fetch telegram domain preview")
		return
	}

	response.Success(c, http.StatusOK, "telegram domain preview fetched successfully", result)
}

func (h TelegramHandler) DomainSubmit(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.DomainSubmit(c.Request.Context(), telegram.DomainSubmitInput{
		TelegramID:  req.TelegramID,
		ContainerID: req.ContainerID,
		Domain:      req.Domain,
		TargetPort:  req.TargetPort,
		ProxyMode:   req.ProxyMode,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to submit telegram domain setup")
		return
	}

	response.Success(c, http.StatusOK, "telegram domain setup submitted successfully", result)
}

func (h TelegramHandler) ReconfigIPPreview(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.ReconfigIPPreview(c.Request.Context(), telegram.ReconfigIPPreviewInput{
		TelegramID:  req.TelegramID,
		ContainerID: req.ContainerID,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to fetch telegram reconfig ip preview")
		return
	}

	response.Success(c, http.StatusOK, "telegram reconfig ip preview fetched successfully", result)
}

func (h TelegramHandler) ReconfigIPSubmit(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramReconfigIPSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.ReconfigIPSubmit(c.Request.Context(), telegram.ReconfigIPSubmitInput{
		TelegramID:       req.TelegramID,
		ContainerID:      req.ContainerID,
		RequestedSSHPort: req.RequestedSSHPort,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to submit telegram reconfig ip")
		return
	}

	response.Success(c, http.StatusOK, "telegram reconfig ip submitted successfully", result)
}

func (h TelegramHandler) TransferPreview(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.TransferPreview(c.Request.Context(), telegram.TransferPreviewInput{
		TelegramID:       req.TelegramID,
		ContainerID:      req.ContainerID,
		TargetTelegramID: req.TargetTelegramID,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to fetch telegram transfer preview")
		return
	}

	response.Success(c, http.StatusOK, "telegram transfer preview fetched successfully", result)
}

func (h TelegramHandler) TransferSubmit(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.TransferSubmit(c.Request.Context(), telegram.TransferSubmitInput{
		TelegramID:       req.TelegramID,
		ContainerID:      req.ContainerID,
		TargetTelegramID: req.TargetTelegramID,
		Reason:           req.Reason,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to submit telegram transfer")
		return
	}

	response.Success(c, http.StatusOK, "telegram transfer submitted successfully", result)
}

func (h TelegramHandler) CancelPreview(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.CancelPreview(c.Request.Context(), telegram.CancelPreviewInput{
		TelegramID:  req.TelegramID,
		ContainerID: req.ContainerID,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to fetch telegram cancel preview")
		return
	}

	response.Success(c, http.StatusOK, "telegram cancel preview fetched successfully", result)
}

func (h TelegramHandler) CancelSubmit(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.CancelSubmit(c.Request.Context(), telegram.CancelSubmitInput{
		TelegramID:  req.TelegramID,
		ContainerID: req.ContainerID,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to submit telegram cancel")
		return
	}

	response.Success(c, http.StatusOK, "telegram cancel submitted successfully", result)
}

func (h TelegramHandler) runtimeAction(c *gin.Context, action string) {
	if !h.authorize(c) {
		return
	}

	var req telegramRuntimeActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.RuntimeAction(c.Request.Context(), telegram.RuntimeActionInput{
		TelegramID:  req.TelegramID,
		ContainerID: req.ContainerID,
		Action:      action,
	})
	if err != nil {
		h.writeActionError(c, err, "failed to submit telegram runtime action")
		return
	}

	response.Success(c, http.StatusOK, "telegram runtime action submitted successfully", result)
}

func (h TelegramHandler) writeActionError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, telegram.ErrInvalidTelegramUser), errors.Is(err, telegram.ErrInvalidActionRequest):
		response.Fail(c, http.StatusBadRequest, err.Error(), "bad_request", nil)
	case errors.Is(err, telegram.ErrTelegramUserNotFound), errors.Is(err, telegram.ErrMyVPSNotFound), errors.Is(err, telegram.ErrUpgradePackageNotFound), errors.Is(err, telegram.ErrTransferTargetNotFound):
		response.Fail(c, http.StatusNotFound, err.Error(), "not_found", nil)
	case errors.Is(err, telegram.ErrInsufficientBalance), errors.Is(err, telegram.ErrUpgradePackageIneligible), errors.Is(err, telegram.ErrTransferTargetSame), errors.Is(err, telegram.ErrDomainAlreadyExists), errors.Is(err, telegram.ErrActionConflict), errors.Is(err, telegram.ErrContainerNotRunning), errors.Is(err, telegram.ErrServiceNotOperable), errors.Is(err, telegram.ErrNoActiveSSHMapping), errors.Is(err, telegram.ErrDomainDNSMismatch):
		response.Fail(c, http.StatusConflict, err.Error(), "conflict", nil)
	case errors.Is(err, telegram.ErrUnsupportedPayment), errors.Is(err, telegram.ErrInvalidDomain):
		response.Fail(c, http.StatusBadRequest, err.Error(), "bad_request", nil)
	case errors.Is(err, telegram.ErrIncusUnavailable), errors.Is(err, telegram.ErrReverseProxyUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, err.Error(), "service_unavailable", nil)
	default:
		response.Fail(c, http.StatusInternalServerError, fallback, "internal_server_error", nil)
	}
}
