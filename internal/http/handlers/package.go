package handlers

import (
	"errors"
	"net/http"
	"strconv"

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
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
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

	c.JSON(http.StatusCreated, gin.H{
		"message": "package created successfully",
		"data":    toPackageResponse(pkg),
	})
}

func (h PackageHandler) List(c *gin.Context) {
	var isActive *bool
	raw := c.Query("is_active")
	if raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "is_active must be a boolean"})
			return
		}

		isActive = &value
	}

	items, err := h.service.List(c.Request.Context(), isActive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to list packages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": toPackageResponses(items)})
}

func (h PackageHandler) GetByID(c *gin.Context) {
	pkg, err := h.service.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		handlePackageError(c, err, "failed to get package")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": toPackageResponse(pkg)})
}

func (h PackageHandler) Update(c *gin.Context) {
	var req updatePackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
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

	c.JSON(http.StatusOK, gin.H{
		"message": "package updated successfully",
		"data":    toPackageResponse(pkg),
	})
}

func (h PackageHandler) Delete(c *gin.Context) {
	pkg, err := h.service.Delete(c.Request.Context(), c.Param("id"))
	if err != nil {
		handlePackageError(c, err, "failed to delete package")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "package deactivated successfully",
		"data":    toPackageResponse(pkg),
	})
}

func handlePackageError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, packages.ErrPackageNotFound):
		c.JSON(http.StatusNotFound, gin.H{"message": "package not found"})
	case errors.Is(err, packages.ErrInvalidPackageInput):
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"message": fallback})
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
