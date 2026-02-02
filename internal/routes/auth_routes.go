package routes

import (
	"github.com/raflytch/careerly-server/internal/handler"

	"github.com/gofiber/fiber/v2"
)

func setupAuthRoutes(router fiber.Router, h *handler.AuthHandler) {
	auth := router.Group("/auth")

	google := auth.Group("/google")
	google.Get("/login", h.GoogleLogin)
	google.Get("/callback", h.GoogleCallback)

	restore := auth.Group("/restore")
	restore.Post("/request-otp", h.RequestRestoreOTP)
	restore.Post("/verify-otp", h.VerifyRestoreOTP)
	restore.Post("/resend-otp", h.ResendRestoreOTP)
}
