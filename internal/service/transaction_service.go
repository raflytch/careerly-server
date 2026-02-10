package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/raflytch/careerly-server/internal/domain"
	"github.com/raflytch/careerly-server/pkg/midtrans"

	"github.com/google/uuid"
)

const (
	// Cache key prefixes for transaction caching
	transactionCachePrefix  = "transaction:"
	transactionListCacheKey = "transactions:list"
	// Transaction expiry duration (24 hours)
	defaultTransactionExpiry = 24 * time.Hour
)

var (
	ErrTransactionNotFound      = errors.New("transaction not found")
	ErrTransactionAlreadyPaid   = errors.New("transaction has already been paid")
	ErrInvalidTransactionAmount = errors.New("transaction amount does not match plan price")
	ErrPlanNotAvailable         = errors.New("plan is not available for purchase")
	ErrActiveSubscriptionExists = errors.New("user already has an active subscription for this plan")
	ErrInvalidSignature         = errors.New("invalid webhook signature")
)

type transactionService struct {
	transactionRepo  domain.TransactionRepository
	planRepo         domain.PlanRepository
	subscriptionRepo domain.SubscriptionRepository
	userRepo         domain.UserRepository
	cacheRepo        domain.CacheRepository
	midtransClient   *midtrans.Client
}

// NewTransactionService creates a new transaction service instance
func NewTransactionService(
	transactionRepo domain.TransactionRepository,
	planRepo domain.PlanRepository,
	subscriptionRepo domain.SubscriptionRepository,
	userRepo domain.UserRepository,
	cacheRepo domain.CacheRepository,
	midtransClient *midtrans.Client,
) domain.TransactionService {
	return &transactionService{
		transactionRepo:  transactionRepo,
		planRepo:         planRepo,
		subscriptionRepo: subscriptionRepo,
		userRepo:         userRepo,
		cacheRepo:        cacheRepo,
		midtransClient:   midtransClient,
	}
}

// CreateTransaction creates a new transaction and generates Snap token for payment
// SECURITY: Validates plan price from database, never trusts frontend amount
func (s *transactionService) CreateTransaction(ctx context.Context, userID uuid.UUID, req *domain.CreateTransactionRequest) (*domain.TransactionResponse, error) {
	// Fetch plan from database - NEVER trust frontend price
	plan, err := s.planRepo.FindByID(ctx, req.PlanID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPlanNotAvailable
		}
		return nil, fmt.Errorf("failed to fetch plan: %w", err)
	}

	// Validate plan is active and purchasable
	if !plan.IsActive {
		return nil, ErrPlanNotAvailable
	}

	// Check if plan is free (price = 0), no transaction needed
	if plan.Price.IsZero() {
		return nil, errors.New("free plans do not require payment")
	}

	// Check for existing active subscription for this plan
	existingSub, _ := s.subscriptionRepo.FindActiveByUserID(ctx, userID)
	if existingSub != nil && existingSub.PlanID == req.PlanID {
		return nil, ErrActiveSubscriptionExists
	}

	// Fetch user details for customer info
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	// Generate unique order ID with timestamp for idempotency
	// Format: CAREERLY-{planID_short}-{userID_short}-{timestamp}
	orderID := fmt.Sprintf("CAREERLY-%s-%s-%d",
		plan.ID.String()[:8],
		userID.String()[:8],
		time.Now().UnixMilli(),
	)

	// Use plan price from database (TRUSTED SOURCE)
	grossAmount := plan.Price.IntPart() // Convert decimal to int64 for Midtrans

	// Create Midtrans Snap transaction
	midtransReq := midtrans.CreateTransactionRequest{
		OrderID:     orderID,
		GrossAmount: grossAmount,
		ItemDetails: []midtrans.ItemDetail{
			{
				ID:       plan.ID.String(),
				Name:     plan.DisplayName,
				Price:    grossAmount,
				Quantity: 1,
			},
		},
		CustomerDetails: midtrans.CustomerDetail{
			FirstName: user.Name,
			Email:     user.Email,
		},
	}

	snapResp, err := s.midtransClient.CreateSnapTransaction(midtransReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create midtrans transaction: %w", err)
	}

	// Calculate expiry time (24 hours from now)
	expiryTime := time.Now().Add(defaultTransactionExpiry)

	// Create transaction record in database
	transaction := &domain.Transaction{
		ID:          uuid.New(),
		UserID:      userID,
		PlanID:      plan.ID,
		OrderID:     orderID,
		GrossAmount: plan.Price, // Store exact plan price
		Status:      domain.TransactionStatusPending,
		SnapToken:   &snapResp.Token,
		RedirectURL: &snapResp.RedirectURL,
		ExpiredAt:   &expiryTime,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.transactionRepo.Create(ctx, transaction); err != nil {
		return nil, fmt.Errorf("failed to create transaction record: %w", err)
	}

	// Attach plan info for response
	transaction.Plan = plan

	return &domain.TransactionResponse{
		Transaction: transaction,
		SnapToken:   snapResp.Token,
		RedirectURL: snapResp.RedirectURL,
	}, nil
}

// GetByID retrieves a transaction by ID, ensuring it belongs to the requesting user
func (s *transactionService) GetByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*domain.Transaction, error) {
	transaction, err := s.transactionRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, err
	}

	// Security check: ensure transaction belongs to the requesting user
	if transaction.UserID != userID {
		return nil, ErrTransactionNotFound
	}

	return transaction, nil
}

// GetByOrderID retrieves a transaction by Midtrans order ID
func (s *transactionService) GetByOrderID(ctx context.Context, orderID string) (*domain.Transaction, error) {
	transaction, err := s.transactionRepo.FindByOrderID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, err
	}
	return transaction, nil
}

// GetUserTransactions retrieves all transactions for a user with pagination
func (s *transactionService) GetUserTransactions(ctx context.Context, userID uuid.UUID, page, limit int) (*domain.PaginatedTransactions, error) {
	// Validate and normalize pagination parameters
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	// Get total count for pagination
	total, err := s.transactionRepo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count transactions: %w", err)
	}

	// Fetch transactions
	transactions, err := s.transactionRepo.FindByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transactions: %w", err)
	}

	// Calculate total pages
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	return &domain.PaginatedTransactions{
		Transactions: transactions,
		Pagination: domain.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

// HandleWebhook processes Midtrans webhook notification
// This is called when Midtrans sends payment status updates
func (s *transactionService) HandleWebhook(ctx context.Context, payload map[string]interface{}) error {
	// Extract required fields from payload
	orderID, ok := payload["order_id"].(string)
	if !ok || orderID == "" {
		return errors.New("missing order_id in webhook payload")
	}

	statusCode, _ := payload["status_code"].(string)
	grossAmount, _ := payload["gross_amount"].(string)
	signatureKey, _ := payload["signature_key"].(string)

	// Verify webhook signature to prevent tampering
	// Skip verification if signature is empty (sandbox mode may not send it)
	if signatureKey != "" {
		if !s.midtransClient.VerifySignatureKey(orderID, statusCode, grossAmount, signatureKey) {
			return ErrInvalidSignature
		}
	}

	// Find transaction in our database first
	transaction, err := s.transactionRepo.FindByOrderID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTransactionNotFound
		}
		return fmt.Errorf("failed to find transaction: %w", err)
	}

	// Skip if transaction is already in final state
	if transaction.Status == domain.TransactionStatusSuccess ||
		transaction.Status == domain.TransactionStatusFailed {
		return nil
	}

	// Fetch transaction from Midtrans Core API for verification
	// IMPORTANT: Never trust webhook payload directly for status changes
	statusResp, err := s.midtransClient.CheckTransaction(orderID)
	if err != nil {
		return fmt.Errorf("failed to verify transaction with midtrans: %w", err)
	}

	// Update transaction with Midtrans response data
	transaction.TransactionID = &statusResp.TransactionID
	transaction.PaymentType = &statusResp.PaymentType
	transaction.TransactionStatus = &statusResp.TransactionStatus
	transaction.FraudStatus = &statusResp.FraudStatus

	// Store raw Midtrans response for audit trail
	responseJSON, _ := json.Marshal(payload)
	transaction.MidtransResponse = responseJSON

	// Determine our internal status based on Midtrans status
	newStatus := s.mapMidtransStatus(statusResp.TransactionStatus, statusResp.FraudStatus)
	transaction.Status = newStatus

	// If payment is successful, create subscription and update transaction
	if newStatus == domain.TransactionStatusSuccess {
		now := time.Now()
		transaction.PaidAt = &now

		// Create subscription for the user only if not already created
		if transaction.SubscriptionID == nil {
			subscriptionID, err := s.createSubscription(ctx, transaction)
			if err != nil {
				return fmt.Errorf("failed to create subscription: %w", err)
			}
			transaction.SubscriptionID = &subscriptionID
		}
	}

	// Update transaction in database
	if err := s.transactionRepo.Update(ctx, transaction); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	// Invalidate any cached data
	s.invalidateCache(ctx, transaction.ID)

	return nil
}

// CheckTransactionStatus manually checks and updates transaction status from Midtrans
func (s *transactionService) CheckTransactionStatus(ctx context.Context, orderID string) (*domain.Transaction, error) {
	// Fetch transaction from our database
	transaction, err := s.transactionRepo.FindByOrderID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, err
	}

	// Skip check if transaction is already in final state
	if transaction.Status == domain.TransactionStatusSuccess ||
		transaction.Status == domain.TransactionStatusFailed {
		return transaction, nil
	}

	// Check status with Midtrans Core API
	statusResp, err := s.midtransClient.CheckTransaction(orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to check transaction status: %w", err)
	}

	// Update transaction with fresh Midtrans data
	transaction.TransactionID = &statusResp.TransactionID
	transaction.PaymentType = &statusResp.PaymentType
	transaction.TransactionStatus = &statusResp.TransactionStatus
	transaction.FraudStatus = &statusResp.FraudStatus

	// Map Midtrans status to our internal status
	newStatus := s.mapMidtransStatus(statusResp.TransactionStatus, statusResp.FraudStatus)
	transaction.Status = newStatus

	// Handle successful payment
	if newStatus == domain.TransactionStatusSuccess && transaction.SubscriptionID == nil {
		now := time.Now()
		transaction.PaidAt = &now

		subscriptionID, err := s.createSubscription(ctx, transaction)
		if err != nil {
			return nil, fmt.Errorf("failed to create subscription: %w", err)
		}
		transaction.SubscriptionID = &subscriptionID
	}

	// Persist updates
	if err := s.transactionRepo.Update(ctx, transaction); err != nil {
		return nil, fmt.Errorf("failed to update transaction: %w", err)
	}

	return transaction, nil
}

// createSubscription creates a new subscription when payment is successful
func (s *transactionService) createSubscription(ctx context.Context, transaction *domain.Transaction) (uuid.UUID, error) {
	// Fetch plan to get duration
	plan, err := s.planRepo.FindByID(ctx, transaction.PlanID)
	if err != nil {
		return uuid.Nil, err
	}

	// Calculate subscription duration
	durationDays := 30 // Default 30 days
	if plan.DurationDays != nil {
		durationDays = *plan.DurationDays
	}

	now := time.Now()
	endDate := now.AddDate(0, 0, durationDays)

	// Cancel any existing active subscription for this user
	existingSub, _ := s.subscriptionRepo.FindActiveByUserID(ctx, transaction.UserID)
	if existingSub != nil {
		existingSub.Status = domain.SubscriptionStatusCanceled
		_ = s.subscriptionRepo.Update(ctx, existingSub)
	}

	// Create new subscription
	subscription := &domain.Subscription{
		ID:        uuid.New(),
		UserID:    transaction.UserID,
		PlanID:    transaction.PlanID,
		StartDate: now,
		EndDate:   endDate,
		Status:    domain.SubscriptionStatusActive,
		CreatedAt: now,
	}

	if err := s.subscriptionRepo.Create(ctx, subscription); err != nil {
		return uuid.Nil, err
	}

	return subscription.ID, nil
}

// mapMidtransStatus maps Midtrans transaction status to our internal status
// Reference: https://docs.midtrans.com/docs/https-notification-webhooks
func (s *transactionService) mapMidtransStatus(transactionStatus, fraudStatus string) domain.TransactionStatus {
	switch transactionStatus {
	case "capture":
		// For credit card, check fraud status
		if fraudStatus == "accept" {
			return domain.TransactionStatusSuccess
		}
		// "challenge" status requires manual review - keep as pending
		return domain.TransactionStatusPending

	case "settlement":
		// Payment has been settled (final success state)
		return domain.TransactionStatusSuccess

	case "pending":
		// Waiting for customer to complete payment
		return domain.TransactionStatusPending

	case "deny":
		// Transaction denied (but may allow retry)
		return domain.TransactionStatusFailed

	case "cancel":
		// Transaction cancelled
		return domain.TransactionStatusCancel

	case "expire":
		// Transaction expired
		return domain.TransactionStatusExpired

	case "refund", "partial_refund":
		// Refunded transactions - treat as failed for our purposes
		return domain.TransactionStatusFailed

	default:
		return domain.TransactionStatusPending
	}
}

// invalidateCache removes cached transaction data
func (s *transactionService) invalidateCache(ctx context.Context, transactionID uuid.UUID) {
	cacheKey := fmt.Sprintf("%s%s", transactionCachePrefix, transactionID.String())
	_ = s.cacheRepo.Delete(ctx, cacheKey)
	_ = s.cacheRepo.DeleteByPattern(ctx, transactionListCacheKey+"*")
}
