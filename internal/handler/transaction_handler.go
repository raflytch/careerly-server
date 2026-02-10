package handler

import (
	"errors"
	"log"

	"github.com/raflytch/careerly-server/internal/domain"
	"github.com/raflytch/careerly-server/internal/middleware"
	"github.com/raflytch/careerly-server/internal/service"
	"github.com/raflytch/careerly-server/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type TransactionHandler struct {
	transactionService domain.TransactionService
}

// NewTransactionHandler creates a new transaction handler instance
func NewTransactionHandler(transactionService domain.TransactionService) *TransactionHandler {
	return &TransactionHandler{
		transactionService: transactionService,
	}
}

// CreateTransaction handles POST /transactions
// Creates a new transaction and returns Snap token for Midtrans payment page
func (h *TransactionHandler) CreateTransaction(c *fiber.Ctx) error {
	// Get authenticated user from context
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "unauthorized")
	}

	// Parse request body
	var req domain.CreateTransactionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	// Validate plan_id is provided
	if req.PlanID == uuid.Nil {
		return response.BadRequest(c, "plan_id is required")
	}

	// Create transaction via service
	result, err := h.transactionService.CreateTransaction(c.UserContext(), user.ID, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPlanNotAvailable):
			return response.BadRequest(c, "plan is not available for purchase")
		case errors.Is(err, service.ErrActiveSubscriptionExists):
			return response.BadRequest(c, "you already have an active subscription for this plan")
		default:
			return response.InternalError(c, err.Error())
		}
	}

	return response.Success(c, fiber.StatusCreated, "transaction created, redirect to payment page", result)
}

// GetTransaction handles GET /transactions/:id
// Retrieves a single transaction by ID
func (h *TransactionHandler) GetTransaction(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "unauthorized")
	}

	// Parse transaction ID from URL
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid transaction id")
	}

	// Fetch transaction (service ensures user owns it)
	transaction, err := h.transactionService.GetByID(c.UserContext(), user.ID, id)
	if err != nil {
		if errors.Is(err, service.ErrTransactionNotFound) {
			return response.NotFound(c, "transaction not found")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "transaction retrieved", transaction)
}

// GetUserTransactions handles GET /transactions
// Retrieves all transactions for the authenticated user with pagination
func (h *TransactionHandler) GetUserTransactions(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "unauthorized")
	}

	// Parse pagination parameters
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	// Fetch paginated transactions
	result, err := h.transactionService.GetUserTransactions(c.UserContext(), user.ID, page, limit)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "transactions retrieved", result)
}

// CheckTransactionStatus handles GET /transactions/:id/status
// Manually checks and updates transaction status from Midtrans
func (h *TransactionHandler) CheckTransactionStatus(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "unauthorized")
	}

	// Parse transaction ID
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid transaction id")
	}

	// First verify user owns this transaction
	transaction, err := h.transactionService.GetByID(c.UserContext(), user.ID, id)
	if err != nil {
		if errors.Is(err, service.ErrTransactionNotFound) {
			return response.NotFound(c, "transaction not found")
		}
		return response.InternalError(c, err.Error())
	}

	// Check status with Midtrans
	updated, err := h.transactionService.CheckTransactionStatus(c.UserContext(), transaction.OrderID)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "transaction status updated", updated)
}

// MidtransWebhook handles POST /transactions/webhook
// Processes payment notifications from Midtrans
// This endpoint is called by Midtrans servers, not by authenticated users
func (h *TransactionHandler) MidtransWebhook(c *fiber.Ctx) error {
	// Log incoming webhook for debugging
	log.Printf("[WEBHOOK] Received Midtrans notification")

	// Parse webhook payload
	var payload map[string]interface{}
	if err := c.BodyParser(&payload); err != nil {
		log.Printf("[WEBHOOK] Failed to parse payload: %v", err)
		return response.BadRequest(c, "invalid webhook payload")
	}

	// Log payload details for debugging
	orderID, exists := payload["order_id"].(string)
	if !exists || orderID == "" {
		log.Printf("[WEBHOOK] Missing order_id in payload")
		return response.BadRequest(c, "missing order_id in payload")
	}

	transactionStatus, _ := payload["transaction_status"].(string)
	log.Printf("[WEBHOOK] Order: %s, Status: %s", orderID, transactionStatus)

	// Process webhook notification
	if err := h.transactionService.HandleWebhook(c.UserContext(), payload); err != nil {
		log.Printf("[WEBHOOK] Error processing webhook for order %s: %v", orderID, err)

		switch {
		case errors.Is(err, service.ErrInvalidSignature):
			// Don't expose signature validation failure details
			log.Printf("[WEBHOOK] Invalid signature for order %s", orderID)
			return response.Unauthorized(c, "invalid signature")
		case errors.Is(err, service.ErrTransactionNotFound):
			// Return 200 OK to prevent Midtrans from retrying for unknown orders
			log.Printf("[WEBHOOK] Order not found in database: %s", orderID)
			return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ignored", "message": "order not found"})
		default:
			// Log error but return 200 to acknowledge receipt
			// Midtrans will retry on non-2xx responses
			log.Printf("[WEBHOOK] Internal error for order %s: %v", orderID, err)
			return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "error", "message": err.Error()})
		}
	}

	log.Printf("[WEBHOOK] Successfully processed order %s", orderID)
	// Return 200 OK to acknowledge successful processing
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

