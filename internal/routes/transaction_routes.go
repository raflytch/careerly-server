package routes

import (
	"github.com/raflytch/careerly-server/internal/handler"
	"github.com/raflytch/careerly-server/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func setupTransactionRoutes(api fiber.Router, h *handler.TransactionHandler, auth *middleware.AuthMiddleware) {
	transactions := api.Group("/transactions")

	transactions.Post("/webhook", h.MidtransWebhook)

	protected := transactions.Group("", auth.Authenticate())

	protected.Post("", h.CreateTransaction)

	protected.Get("", h.GetUserTransactions)

	protected.Get("/:id", h.GetTransaction)

	protected.Get("/:id/status", h.CheckTransactionStatus)
}
