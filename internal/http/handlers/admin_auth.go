package handlers

import (
	"errors"
	"net/http"

	"github.com/DioSaputra28/vps-nat/internal/auth"
	"github.com/DioSaputra28/vps-nat/internal/http/middleware"
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
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
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
		if errors.Is(err, auth.ErrInvalidCredentials) {
			status = http.StatusUnauthorized
			message = "invalid email or password"
		}

		c.JSON(status, gin.H{"message": message})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "login successful",
		"data": gin.H{
			"token":      result.Token,
			"expires_at": result.ExpiresAt,
			"admin": gin.H{
				"id":         result.Admin.ID,
				"email":      result.Admin.Email,
				"role":       result.Admin.Role,
				"status":     result.Admin.Status,
				"created_at": result.Admin.CreatedAt,
			},
		},
	})
}

func (h AdminAuthHandler) Logout(c *gin.Context) {
	authorization := c.GetHeader("Authorization")
	token, ok := middlewareBearerToken(authorization)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "missing or invalid authorization header"})
		return
	}

	if err := h.authService.Logout(token); err != nil {
		if errors.Is(err, auth.ErrUnauthorized) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "logout successful"})
}

func (h AdminAuthHandler) Me(c *gin.Context) {
	admin, ok := middleware.CurrentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":         admin.ID,
			"email":      admin.Email,
			"role":       admin.Role,
			"status":     admin.Status,
			"created_at": admin.CreatedAt,
			"updated_at": admin.UpdatedAt,
		},
	})
}

func middlewareBearerToken(header string) (string, bool) {
	return middlewareBearerTokenInternal(header)
}
