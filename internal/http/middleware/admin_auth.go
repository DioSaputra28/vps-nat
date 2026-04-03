package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/DioSaputra28/vps-nat/internal/auth"
	"github.com/DioSaputra28/vps-nat/internal/http/response"
	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/gin-gonic/gin"
)

const adminContextKey = "admin"

type AdminAuth struct {
	authService *auth.Service
}

func NewAdminAuth(authService *auth.Service) AdminAuth {
	return AdminAuth{authService: authService}
}

func (m AdminAuth) Require() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			response.Fail(c, http.StatusUnauthorized, "missing or invalid authorization header", "unauthorized", nil)
			c.Abort()
			return
		}

		admin, _, err := m.authService.Authenticate(token)
		if err != nil {
			message := "unauthorized"
			if errors.Is(err, auth.ErrSessionExpired) {
				message = "session expired"
			}

			response.Fail(c, http.StatusUnauthorized, message, "unauthorized", nil)
			c.Abort()
			return
		}

		c.Set(adminContextKey, *admin)
		c.Next()
	}
}

func CurrentAdmin(c *gin.Context) (model.AdminUser, bool) {
	value, ok := c.Get(adminContextKey)
	if !ok {
		return model.AdminUser{}, false
	}

	admin, ok := value.(model.AdminUser)
	return admin, ok
}

func bearerToken(header string) (string, bool) {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}

	return token, true
}
