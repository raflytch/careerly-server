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
	transactionCachePrefix   = "transaction:"
	transactionListCacheKey  = "transactions:list"
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

func (s *transactionService) CreateTransaction(ctx context.Context, userID uuid.UUID, req *domain.CreateTransactionRequest) (*domain.TransactionResponse, error) {
	plan, err := s.planRepo.FindByID(ctx, req.PlanID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPlanNotAvailable
		}
		return nil, fmt.Errorf("failed to fetch plan: %w", err)
	}

	if !plan.IsActive {
		return nil, ErrPlanNotAvailable
	}

	if plan.Price.IsZero() {
		return nil, errors.New("free plans do not require payment")
	}

	existingSub, _ := s.subscriptionRepo.FindActiveByUserID(ctx, userID)
	if existingSub != nil && existingSub.PlanID == req.PlanID {
		return nil, ErrActiveSubscriptionExists
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	orderID := fmt.Sprintf("CAREERLY-%s-%s-%d",
		plan.ID.String()[:8],
		userID.String()[:8],
		time.Now().UnixMilli(),
	)

	grossAmount := plan.Price.IntPart()

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

	expiryTime := time.Now().Add(defaultTransactionExpiry)

	transaction := &domain.Transaction{
		ID:          uuid.New(),
		UserID:      userID,
		PlanID:      plan.ID,
		OrderID:     orderID,
		GrossAmount: plan.Price,
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

	transaction.Plan = plan

	return &domain.TransactionResponse{
		Transaction: transaction,
		SnapToken:   snapResp.Token,
		RedirectURL: snapResp.RedirectURL,
	}, nil
}

func (s *transactionService) GetByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*domain.Transaction, error) {
	transaction, err := s.transactionRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, err
	}

	if transaction.UserID != userID {
		return nil, ErrTransactionNotFound
	}

	return transaction, nil
}

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

func (s *transactionService) GetUserTransactions(ctx context.Context, userID uuid.UUID, page, limit int) (*domain.PaginatedTransactions, error) {
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

	total, err := s.transactionRepo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count transactions: %w", err)
	}

	transactions, err := s.transactionRepo.FindByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transactions: %w", err)
	}

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

func (s *transactionService) HandleWebhook(ctx context.Context, payload map[string]interface{}) error {
	orderID, ok := payload["order_id"].(string)
	if !ok || orderID == "" {
		return errors.New("missing order_id in webhook payload")
	}

	statusCode, _ := payload["status_code"].(string)
	grossAmount, _ := payload["gross_amount"].(string)
	signatureKey, _ := payload["signature_key"].(string)

	if signatureKey != "" {
		if !s.midtransClient.VerifySignatureKey(orderID, statusCode, grossAmount, signatureKey) {
			return ErrInvalidSignature
		}
	}

	transaction, err := s.transactionRepo.FindByOrderID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTransactionNotFound
		}
		return fmt.Errorf("failed to find transaction: %w", err)
	}

	if transaction.Status == domain.TransactionStatusSuccess ||
		transaction.Status == domain.TransactionStatusFailed {
		return nil
	}

	statusResp, err := s.midtransClient.CheckTransaction(orderID)
	if err != nil {
		return fmt.Errorf("failed to verify transaction with midtrans: %w", err)
	}

	transaction.TransactionID = &statusResp.TransactionID
	transaction.PaymentType = &statusResp.PaymentType
	transaction.TransactionStatus = &statusResp.TransactionStatus
	transaction.FraudStatus = &statusResp.FraudStatus

	responseJSON, _ := json.Marshal(payload)
	transaction.MidtransResponse = responseJSON

	newStatus := s.mapMidtransStatus(statusResp.TransactionStatus, statusResp.FraudStatus)
	transaction.Status = newStatus

	if newStatus == domain.TransactionStatusSuccess {
		now := time.Now()
		transaction.PaidAt = &now

		if transaction.SubscriptionID == nil {
			subscriptionID, err := s.createSubscription(ctx, transaction)
			if err != nil {
				return fmt.Errorf("failed to create subscription: %w", err)
			}
			transaction.SubscriptionID = &subscriptionID
		}
	}

	if err := s.transactionRepo.Update(ctx, transaction); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	s.invalidateCache(ctx, transaction.ID)

	return nil
}

func (s *transactionService) CheckTransactionStatus(ctx context.Context, orderID string) (*domain.Transaction, error) {
	transaction, err := s.transactionRepo.FindByOrderID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, err
	}

	if transaction.Status == domain.TransactionStatusSuccess ||
		transaction.Status == domain.TransactionStatusFailed {
		return transaction, nil
	}

	statusResp, err := s.midtransClient.CheckTransaction(orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to check transaction status: %w", err)
	}

	transaction.TransactionID = &statusResp.TransactionID
	transaction.PaymentType = &statusResp.PaymentType
	transaction.TransactionStatus = &statusResp.TransactionStatus
	transaction.FraudStatus = &statusResp.FraudStatus

	newStatus := s.mapMidtransStatus(statusResp.TransactionStatus, statusResp.FraudStatus)
	transaction.Status = newStatus

	if newStatus == domain.TransactionStatusSuccess && transaction.SubscriptionID == nil {
		now := time.Now()
		transaction.PaidAt = &now

		subscriptionID, err := s.createSubscription(ctx, transaction)
		if err != nil {
			return nil, fmt.Errorf("failed to create subscription: %w", err)
		}
		transaction.SubscriptionID = &subscriptionID
	}

	if err := s.transactionRepo.Update(ctx, transaction); err != nil {
		return nil, fmt.Errorf("failed to update transaction: %w", err)
	}

	return transaction, nil
}

func (s *transactionService) createSubscription(ctx context.Context, transaction *domain.Transaction) (uuid.UUID, error) {
	plan, err := s.planRepo.FindByID(ctx, transaction.PlanID)
	if err != nil {
		return uuid.Nil, err
	}

	durationDays := 30
	if plan.DurationDays != nil {
		durationDays = *plan.DurationDays
	}

	now := time.Now()
	endDate := now.AddDate(0, 0, durationDays)

	existingSub, _ := s.subscriptionRepo.FindActiveByUserID(ctx, transaction.UserID)
	if existingSub != nil {
		existingSub.Status = domain.SubscriptionStatusCanceled
		_ = s.subscriptionRepo.Update(ctx, existingSub)
	}

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

func (s *transactionService) mapMidtransStatus(transactionStatus, fraudStatus string) domain.TransactionStatus {
	switch transactionStatus {
	case "capture":
		if fraudStatus == "accept" {
			return domain.TransactionStatusSuccess
		}
		return domain.TransactionStatusPending

	case "settlement":
		return domain.TransactionStatusSuccess

	case "pending":
		return domain.TransactionStatusPending

	case "deny":
		return domain.TransactionStatusFailed

	case "cancel":
		return domain.TransactionStatusCancel

	case "expire":
		return domain.TransactionStatusExpired

	case "refund", "partial_refund":
		return domain.TransactionStatusFailed

	default:
		return domain.TransactionStatusPending
	}
}

func (s *transactionService) invalidateCache(ctx context.Context, transactionID uuid.UUID) {
	cacheKey := fmt.Sprintf("%s%s", transactionCachePrefix, transactionID.String())
	_ = s.cacheRepo.Delete(ctx, cacheKey)
	_ = s.cacheRepo.DeleteByPattern(ctx, transactionListCacheKey+"*")
}
