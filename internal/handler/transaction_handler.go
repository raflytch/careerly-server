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

func NewTransactionHandler(transactionService domain.TransactionService) *TransactionHandler {
	return &TransactionHandler{
		transactionService: transactionService,
	}
}

func (h *TransactionHandler) CreateTransaction(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "unauthorized")
	}

	var req domain.CreateTransactionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	if req.PlanID == uuid.Nil {
		return response.BadRequest(c, "plan_id is required")
	}

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

func (h *TransactionHandler) GetTransaction(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "unauthorized")
	}

	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid transaction id")
	}

	transaction, err := h.transactionService.GetByID(c.UserContext(), user.ID, id)
	if err != nil {
		if errors.Is(err, service.ErrTransactionNotFound) {
			return response.NotFound(c, "transaction not found")
		}
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "transaction retrieved", transaction)
}

func (h *TransactionHandler) GetUserTransactions(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "unauthorized")
	}

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	result, err := h.transactionService.GetUserTransactions(c.UserContext(), user.ID, page, limit)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "transactions retrieved", result)
}

func (h *TransactionHandler) CheckTransactionStatus(c *fiber.Ctx) error {
	user := middleware.GetUserFromContext(c)
	if user == nil {
		return response.Unauthorized(c, "unauthorized")
	}

	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "invalid transaction id")
	}

	transaction, err := h.transactionService.GetByID(c.UserContext(), user.ID, id)
	if err != nil {
		if errors.Is(err, service.ErrTransactionNotFound) {
			return response.NotFound(c, "transaction not found")
		}
		return response.InternalError(c, err.Error())
	}

	updated, err := h.transactionService.CheckTransactionStatus(c.UserContext(), transaction.OrderID)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "transaction status updated", updated)
}

func (h *TransactionHandler) MidtransWebhook(c *fiber.Ctx) error {
	log.Printf("[WEBHOOK] Received Midtrans notification")

	var payload map[string]interface{}
	if err := c.BodyParser(&payload); err != nil {
		log.Printf("[WEBHOOK] Failed to parse payload: %v", err)
		return response.BadRequest(c, "invalid webhook payload")
	}

	orderID, exists := payload["order_id"].(string)
	if !exists || orderID == "" {
		log.Printf("[WEBHOOK] Missing order_id in payload")
		return response.BadRequest(c, "missing order_id in payload")
	}

	transactionStatus, _ := payload["transaction_status"].(string)
	log.Printf("[WEBHOOK] Order: %s, Status: %s", orderID, transactionStatus)

	if err := h.transactionService.HandleWebhook(c.UserContext(), payload); err != nil {
		log.Printf("[WEBHOOK] Error processing webhook for order %s: %v", orderID, err)

		switch {
		case errors.Is(err, service.ErrInvalidSignature):
			log.Printf("[WEBHOOK] Invalid signature for order %s", orderID)
			return response.Unauthorized(c, "invalid signature")
		case errors.Is(err, service.ErrTransactionNotFound):
			log.Printf("[WEBHOOK] Order not found in database: %s", orderID)
			return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ignored", "message": "order not found"})
		default:
			log.Printf("[WEBHOOK] Internal error for order %s: %v", orderID, err)
			return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "error", "message": err.Error()})
		}
	}

	log.Printf("[WEBHOOK] Successfully processed order %s", orderID)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

