package routes

import (
	"github.com/raflytch/careerly-server/internal/handler"
	"github.com/raflytch/careerly-server/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func setupUserRoutes(router fiber.Router, h *handler.UserHandler, authMiddleware *middleware.AuthMiddleware) {
	users := router.Group("/users")
	users.Use(authMiddleware.Authenticate())

	users.Get("/profile", h.GetProfile)
	users.Put("/profile", h.Update)
	users.Get("/", middleware.RequireAdmin(), h.GetAll)
	users.Get("/:id", middleware.RequireAdmin(), h.GetByID)
	users.Delete("/:id", middleware.RequireAdmin(), h.Delete)
}
