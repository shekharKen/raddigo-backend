package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/raddigo/raddigo/internal/auth"
)

// Context keys and role values set by the authentication middleware.
const (
	ContextSubjectKey = "authSubject"
	ContextRoleKey    = "authRole"
)

// Logger returns a Gin middleware that logs each request via slog.
func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		logger.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", time.Since(start).String(),
			"remote", c.ClientIP(),
		)
	}
}

// Recoverer returns a Gin middleware that converts panics into 500 responses.
func Recoverer(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic recovered",
					"error", rec,
					"stack", string(debug.Stack()),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
		}()
		c.Next()
	}
}

// Authenticate validates the Bearer access token and stores the subject id and
// role in the request context. It aborts with 401 when the token is missing or
// invalid.
func Authenticate(tokens *auth.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.SplitN(c.GetHeader("Authorization"), " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed authorization header"})
			return
		}
		claims, err := tokens.ParseAccess(strings.TrimSpace(parts[1]))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		c.Set(ContextSubjectKey, claims.Subject)
		c.Set(ContextRoleKey, string(claims.Role))
		c.Next()
	}
}

// RequireUser ensures the caller is an authenticated user whose id matches the
// given path parameter.
func RequireUser(param string) gin.HandlerFunc {
	return requireSelf(string(auth.RoleUser), param)
}

// RequirePartner ensures the caller is an authenticated partner whose id
// matches the given path parameter.
func RequirePartner(param string) gin.HandlerFunc {
	return requireSelf(string(auth.RolePartner), param)
}

func requireSelf(role, param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString(ContextRoleKey) != role || c.GetString(ContextSubjectKey) != c.Param(param) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}
