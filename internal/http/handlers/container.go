package handlers

import (
	"errors"
	"net/http"

	"github.com/DioSaputra28/vps-nat/internal/containers"
	"github.com/DioSaputra28/vps-nat/internal/http/middleware"
	"github.com/DioSaputra28/vps-nat/internal/http/response"
	"github.com/gin-gonic/gin"
)

type ContainerHandler struct {
	service *containers.Service
}

func NewContainerHandler(service *containers.Service) ContainerHandler {
	return ContainerHandler{service: service}
}

func (h ContainerHandler) List(c *gin.Context) {
	page, err := parsePositiveInt(c.Query("page"), 1)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "page must be a positive integer", "bad_request", nil)
		return
	}

	limit, err := parsePositiveInt(c.Query("limit"), 10)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "limit must be a positive integer", "bad_request", nil)
		return
	}

	result, err := h.service.List(c.Request.Context(), containers.ListInput{
		Page:   page,
		Limit:  limit,
		Search: c.Query("search"),
	})
	if err != nil {
		handleContainerError(c, err, "failed to list containers")
		return
	}

	response.Success(c, http.StatusOK, "containers fetched successfully", gin.H{
		"items": result.Items,
		"meta": gin.H{
			"page":        result.Page,
			"limit":       result.Limit,
			"total_items": result.TotalItems,
			"total_pages": result.TotalPages,
			"search":      result.Search,
		},
	})
}

func (h ContainerHandler) GetByID(c *gin.Context) {
	result, err := h.service.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		handleContainerError(c, err, "failed to get container")
		return
	}

	response.Success(c, http.StatusOK, "container fetched successfully", result)
}

func (h ContainerHandler) Start(c *gin.Context) {
	h.handleAction(c, "start", "container started successfully")
}

func (h ContainerHandler) Stop(c *gin.Context) {
	h.handleAction(c, "stop", "container stopped successfully")
}

func (h ContainerHandler) Suspend(c *gin.Context) {
	h.handleAction(c, "suspend", "container suspended successfully")
}

func (h ContainerHandler) handleAction(c *gin.Context, action string, message string) {
	admin, ok := middleware.CurrentAdmin(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}

	result, err := h.service.Act(c.Request.Context(), containers.ActionInput{
		ContainerID: c.Param("id"),
		Action:      action,
		Admin:       admin,
	})
	if err != nil {
		handleContainerError(c, err, "failed to update container state")
		return
	}

	response.Success(c, http.StatusOK, message, gin.H{
		"action":       result.Action,
		"operation_id": result.OperationID,
		"container":    result.Container,
	})
}

func handleContainerError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, containers.ErrContainerNotFound):
		response.Fail(c, http.StatusNotFound, "container not found", "not_found", nil)
	case errors.Is(err, containers.ErrInvalidPagination):
		response.Fail(c, http.StatusBadRequest, err.Error(), "bad_request", nil)
	case errors.Is(err, containers.ErrIncusUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, "incus server is unavailable", "service_unavailable", nil)
	case errors.Is(err, containers.ErrUnsupportedAction):
		response.Fail(c, http.StatusBadRequest, "unsupported container action", "bad_request", nil)
	case errors.Is(err, containers.ErrContainerAction):
		response.Fail(c, http.StatusBadRequest, fallback, "incus_operation_failed", gin.H{
			"reason": err.Error(),
		})
	default:
		response.Fail(c, http.StatusInternalServerError, fallback, "internal_server_error", nil)
	}
}
