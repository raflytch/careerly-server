package routes

import (
	"github.com/raflytch/careerly-server/internal/handler"
	"github.com/raflytch/careerly-server/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func setupATSCheckRoutes(router fiber.Router, h *handler.ATSCheckHandler, auth *middleware.AuthMiddleware) {
	ats := router.Group("/ats-checks")

	ats.Use(auth.Authenticate())

	ats.Post("/analyze", h.Analyze)
	ats.Get("/", h.GetMyATSChecks)
	ats.Get("/:id", h.GetByID)
	ats.Delete("/:id", h.Delete)
}
