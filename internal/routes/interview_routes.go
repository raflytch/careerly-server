package routes

import (
	"github.com/raflytch/careerly-server/internal/handler"
	"github.com/raflytch/careerly-server/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func setupInterviewRoutes(router fiber.Router, h *handler.InterviewHandler, auth *middleware.AuthMiddleware) {
	interviews := router.Group("/interviews")

	interviews.Use(auth.Authenticate())

	interviews.Post("/", h.Create)
	interviews.Get("/", h.GetMyInterviews)
	interviews.Get("/:id", h.GetByID)
	interviews.Post("/:id/submit", h.SubmitAnswers)
	interviews.Delete("/:id", h.Delete)
}
