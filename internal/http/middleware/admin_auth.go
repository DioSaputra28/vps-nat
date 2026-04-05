package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/DioSaputra28/vps-nat/internal/auth"
	"github.com/DioSaputra28/vps-nat/internal/http/response"
	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/gin-gonic/gin"
)

const adminContextKey = "admin"

type adminRequestContextKey struct{}

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
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), adminRequestContextKey{}, *admin))
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

func CurrentAdminFromContext(ctx context.Context) (model.AdminUser, bool) {
	if ctx == nil {
		return model.AdminUser{}, false
	}

	value := ctx.Value(adminRequestContextKey{})
	admin, ok := value.(model.AdminUser)
	return admin, ok
}

func ContextWithAdmin(ctx context.Context, admin model.AdminUser) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, adminRequestContextKey{}, admin)
}

func HasRole(admin model.AdminUser, roles ...string) bool {
	if len(roles) == 0 {
		return true
	}

	for _, role := range roles {
		if strings.EqualFold(strings.TrimSpace(admin.Role), strings.TrimSpace(role)) {
			return true
		}
	}

	return false
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
