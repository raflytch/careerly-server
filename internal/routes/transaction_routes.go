package routes

import (
	"github.com/raflytch/careerly-server/internal/handler"
	"github.com/raflytch/careerly-server/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

// setupTransactionRoutes configures routes for transaction management
// Includes both authenticated endpoints and public webhook endpoint
func setupTransactionRoutes(api fiber.Router, h *handler.TransactionHandler, auth *middleware.AuthMiddleware) {
	transactions := api.Group("/transactions")

	// Public webhook endpoint - called by Midtrans servers
	// No authentication required as Midtrans uses signature verification
	transactions.Post("/webhook", h.MidtransWebhook)

	// Protected routes - require user authentication
	protected := transactions.Group("", auth.Authenticate())

	// POST /transactions - Create new transaction (initiate payment)
	protected.Post("", h.CreateTransaction)

	// GET /transactions - Get all user transactions with pagination
	protected.Get("", h.GetUserTransactions)

	// GET /transactions/:id - Get single transaction by ID
	protected.Get("/:id", h.GetTransaction)

	// GET /transactions/:id/status - Manually check and update status from Midtrans
	protected.Get("/:id/status", h.CheckTransactionStatus)
}
