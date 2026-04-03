package handlers

import (
	"errors"
	"net/http"

	"github.com/DioSaputra28/vps-nat/internal/auth"
	"github.com/DioSaputra28/vps-nat/internal/http/middleware"
	"github.com/DioSaputra28/vps-nat/internal/http/response"
	"github.com/gin-gonic/gin"
)

type AdminAuthHandler struct {
	authService *auth.Service
}

type adminLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

func NewAdminAuthHandler(authService *auth.Service) AdminAuthHandler {
	return AdminAuthHandler{
		authService: authService,
	}
}

func (h AdminAuthHandler) Login(c *gin.Context) {
	var req adminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.authService.Login(auth.LoginInput{
		Email:     req.Email,
		Password:  req.Password,
		UserAgent: c.GetHeader("User-Agent"),
		IPAddress: c.ClientIP(),
	})
	if err != nil {
		status := http.StatusInternalServerError
		message := "failed to login"
		errorType := "internal_server_error"
		if errors.Is(err, auth.ErrInvalidCredentials) {
			status = http.StatusUnauthorized
			message = "invalid email or password"
			errorType = "unauthorized"
		}

		response.Fail(c, status, message, errorType, nil)
		return
	}

	response.Success(c, http.StatusOK, "login successful", gin.H{
		"token":      result.Token,
		"expires_at": result.ExpiresAt,
		"admin": gin.H{
			"id":         result.Admin.ID,
			"email":      result.Admin.Email,
			"role":       result.Admin.Role,
			"status":     result.Admin.Status,
			"created_at": result.Admin.CreatedAt,
		},
	})
}

func (h AdminAuthHandler) Logout(c *gin.Context) {
	authorization := c.GetHeader("Authorization")
	token, ok := middlewareBearerToken(authorization)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "missing or invalid authorization header", "unauthorized", nil)
		return
	}

	if err := h.authService.Logout(token); err != nil {
		if errors.Is(err, auth.ErrUnauthorized) {
			response.Fail(c, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
			return
		}

		response.Fail(c, http.StatusInternalServerError, "failed to logout", "internal_server_error", nil)
		return
	}

	response.Success(c, http.StatusOK, "logout successful", nil)
}

func (h AdminAuthHandler) Me(c *gin.Context) {
	admin, ok := middleware.CurrentAdmin(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}

	response.Success(c, http.StatusOK, "admin profile fetched successfully", gin.H{
		"id":         admin.ID,
		"email":      admin.Email,
		"role":       admin.Role,
		"status":     admin.Status,
		"created_at": admin.CreatedAt,
		"updated_at": admin.UpdatedAt,
	})
}

func middlewareBearerToken(header string) (string, bool) {
	return middlewareBearerTokenInternal(header)
}
