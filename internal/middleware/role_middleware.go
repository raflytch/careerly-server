package middleware

import (
	"github.com/raflytch/careerly-server/internal/domain"
	"github.com/raflytch/careerly-server/pkg/response"

	"github.com/gofiber/fiber/v2"
)

func RequireRole(roles ...domain.Role) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user := GetUserFromContext(c)
		if user == nil {
			return response.Unauthorized(c, "user not authenticated")
		}

		for _, role := range roles {
			if user.Role == role {
				return c.Next()
			}
		}

		return response.Forbidden(c, "insufficient permissions")
	}
}

func RequireAdmin() fiber.Handler {
	return RequireRole(domain.RoleAdmin)
}
