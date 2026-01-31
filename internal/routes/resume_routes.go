package routes

import (
	"github.com/raflytch/careerly-server/internal/handler"
	"github.com/raflytch/careerly-server/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func setupResumeRoutes(router fiber.Router, h *handler.ResumeHandler, authMiddleware *middleware.AuthMiddleware) {
	resumes := router.Group("/resumes")
	resumes.Use(authMiddleware.Authenticate())

	resumes.Post("/", h.Create)
	resumes.Get("/", h.GetMyResumes)
	resumes.Get("/quota", h.GetQuota)
	resumes.Get("/:id", h.GetByID)
	resumes.Put("/:id", h.Update)
	resumes.Delete("/:id", h.Delete)
	resumes.Get("/:id/pdf", h.DownloadPDF)
}
