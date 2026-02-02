package routes

import (
	"github.com/raflytch/careerly-server/internal/handler"
	"github.com/raflytch/careerly-server/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

type Handlers struct {
	Auth      *handler.AuthHandler
	User      *handler.UserHandler
	Plan      *handler.PlanHandler
	Resume    *handler.ResumeHandler
	Interview *handler.InterviewHandler
}

type Middlewares struct {
	Auth *middleware.AuthMiddleware
}

func Setup(app *fiber.App, handlers Handlers, middlewares Middlewares) {
	app.Get("/health", healthCheck)

	api := app.Group("/api/v1")

	setupAuthRoutes(api, handlers.Auth)
	setupUserRoutes(api, handlers.User, middlewares.Auth)
	setupPlanRoutes(api, handlers.Plan, middlewares.Auth)
	setupResumeRoutes(api, handlers.Resume, middlewares.Auth)
	setupInterviewRoutes(api, handlers.Interview, middlewares.Auth)
}

func healthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"success": true,
		"message": "server is running",
	})
}
