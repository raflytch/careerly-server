package middleware

import (
	"strings"

	"github.com/raflytch/careerly-server/internal/domain"
	"github.com/raflytch/careerly-server/pkg/response"

	"github.com/gofiber/fiber/v2"
)

const UserContextKey = "user"

type AuthMiddleware struct {
	authService domain.AuthService
}

func NewAuthMiddleware(authService domain.AuthService) *AuthMiddleware {
	return &AuthMiddleware{authService: authService}
}

func (m *AuthMiddleware) Authenticate() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "missing authorization header")
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return response.Unauthorized(c, "invalid authorization header format")
		}

		token := parts[1]
		user, err := m.authService.ValidateToken(c.UserContext(), token)
		if err != nil {
			return response.Unauthorized(c, "invalid or expired token")
		}

		c.Locals(UserContextKey, user)
		return c.Next()
	}
}

func GetUserFromContext(c *fiber.Ctx) *domain.User {
	user, ok := c.Locals(UserContextKey).(*domain.User)
	if !ok {
		return nil
	}
	return user
}
