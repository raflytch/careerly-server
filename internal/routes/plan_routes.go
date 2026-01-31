package routes

import (
	"github.com/raflytch/careerly-server/internal/handler"
	"github.com/raflytch/careerly-server/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func setupPlanRoutes(router fiber.Router, h *handler.PlanHandler, authMiddleware *middleware.AuthMiddleware) {
	plans := router.Group("/plans")
	plans.Use(authMiddleware.Authenticate())
	plans.Use(middleware.RequireAdmin())

	plans.Post("/", h.Create)
	plans.Get("/", h.GetAll)
	plans.Get("/:id", h.GetByID)
	plans.Put("/:id", h.Update)
	plans.Delete("/:id", h.Delete)
}
