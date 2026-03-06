package routes

import (
	"github.com/raflytch/careerly-server/internal/handler"
	"github.com/raflytch/careerly-server/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func setupPlanRoutes(router fiber.Router, h *handler.PlanHandler, authMiddleware *middleware.AuthMiddleware) {
	plans := router.Group("/plans")
	plans.Use(authMiddleware.Authenticate())

	plans.Post("/", h.Create)
	plans.Get("/", h.GetAll)

	adminPlans := plans.Group("/")
	adminPlans.Use(middleware.RequireAdmin())
	adminPlans.Get("/:id", h.GetByID)
	adminPlans.Put("/:id", h.Update)
	adminPlans.Delete("/:id", h.Delete)
}
