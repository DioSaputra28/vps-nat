package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/DioSaputra28/vps-nat/internal/http/response"
	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/DioSaputra28/vps-nat/internal/packages"
	"github.com/gin-gonic/gin"
)

type PackageHandler struct {
	service *packages.Service
}

type packageResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  *string `json:"description,omitempty"`
	CPU          int     `json:"cpu"`
	RAMMB        int     `json:"ram_mb"`
	DiskGB       int     `json:"disk_gb"`
	Price        int64   `json:"price"`
	DurationDays int     `json:"duration_days"`
	IsActive     bool    `json:"is_active"`
	CreatedAt    any     `json:"created_at"`
	UpdatedAt    any     `json:"updated_at"`
}

type createPackageRequest struct {
	Name         string  `json:"name" binding:"required"`
	Description  *string `json:"description"`
	CPU          int     `json:"cpu" binding:"required"`
	RAMMB        int     `json:"ram_mb" binding:"required"`
	DiskGB       int     `json:"disk_gb" binding:"required"`
	Price        int64   `json:"price" binding:"required"`
	DurationDays int     `json:"duration_days" binding:"required"`
	IsActive     *bool   `json:"is_active"`
}

type updatePackageRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	CPU          *int    `json:"cpu"`
	RAMMB        *int    `json:"ram_mb"`
	DiskGB       *int    `json:"disk_gb"`
	Price        *int64  `json:"price"`
	DurationDays *int    `json:"duration_days"`
	IsActive     *bool   `json:"is_active"`
}

func NewPackageHandler(service *packages.Service) PackageHandler {
	return PackageHandler{service: service}
}

func (h PackageHandler) Create(c *gin.Context) {
	var req createPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	pkg, err := h.service.Create(c.Request.Context(), packages.CreateInput{
		Name:         req.Name,
		Description:  req.Description,
		CPU:          req.CPU,
		RAMMB:        req.RAMMB,
		DiskGB:       req.DiskGB,
		Price:        req.Price,
		DurationDays: req.DurationDays,
		IsActive:     req.IsActive,
	})
	if err != nil {
		handlePackageError(c, err, "failed to create package")
		return
	}

	response.Success(c, http.StatusCreated, "package created successfully", toPackageResponse(pkg))
}

func (h PackageHandler) List(c *gin.Context) {
	var isActive *bool
	raw := c.Query("is_active")
	if raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "is_active must be a boolean", "bad_request", nil)
			return
		}

		isActive = &value
	}

	items, err := h.service.List(c.Request.Context(), isActive)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to list packages", "internal_server_error", nil)
		return
	}

	response.Success(c, http.StatusOK, "packages fetched successfully", toPackageResponses(items))
}

func (h PackageHandler) GetByID(c *gin.Context) {
	pkg, err := h.service.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		handlePackageError(c, err, "failed to get package")
		return
	}

	response.Success(c, http.StatusOK, "package fetched successfully", toPackageResponse(pkg))
}

func (h PackageHandler) Update(c *gin.Context) {
	var req updatePackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	pkg, err := h.service.Update(c.Request.Context(), c.Param("id"), packages.UpdateInput{
		Name:         req.Name,
		Description:  req.Description,
		CPU:          req.CPU,
		RAMMB:        req.RAMMB,
		DiskGB:       req.DiskGB,
		Price:        req.Price,
		DurationDays: req.DurationDays,
		IsActive:     req.IsActive,
	})
	if err != nil {
		handlePackageError(c, err, "failed to update package")
		return
	}

	response.Success(c, http.StatusOK, "package updated successfully", toPackageResponse(pkg))
}

func (h PackageHandler) Delete(c *gin.Context) {
	pkg, err := h.service.Delete(c.Request.Context(), c.Param("id"))
	if err != nil {
		handlePackageError(c, err, "failed to delete package")
		return
	}

	response.Success(c, http.StatusOK, "package deactivated successfully", toPackageResponse(pkg))
}

func handlePackageError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, packages.ErrPackageNotFound):
		response.Fail(c, http.StatusNotFound, "package not found", "not_found", nil)
	case errors.Is(err, packages.ErrInvalidPackageInput):
		response.Fail(c, http.StatusBadRequest, err.Error(), "bad_request", nil)
	default:
		response.Fail(c, http.StatusInternalServerError, fallback, "internal_server_error", nil)
	}
}

func toPackageResponse(pkg *model.Package) packageResponse {
	return packageResponse{
		ID:           pkg.ID,
		Name:         pkg.Name,
		Description:  pkg.Description,
		CPU:          pkg.CPU,
		RAMMB:        pkg.RAMMB,
		DiskGB:       pkg.DiskGB,
		Price:        pkg.Price,
		DurationDays: pkg.DurationDays,
		IsActive:     pkg.IsActive,
		CreatedAt:    pkg.CreatedAt,
		UpdatedAt:    pkg.UpdatedAt,
	}
}

func toPackageResponses(items []model.Package) []packageResponse {
	result := make([]packageResponse, 0, len(items))
	for i := range items {
		result = append(result, toPackageResponse(&items[i]))
	}

	return result
}
