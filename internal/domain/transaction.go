package domain

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// TransactionStatus represents the status of a transaction in our system
type TransactionStatus string

const (
	TransactionStatusPending TransactionStatus = "pending"
	TransactionStatusSuccess TransactionStatus = "success"
	TransactionStatusFailed  TransactionStatus = "failed"
	TransactionStatusExpired TransactionStatus = "expired"
	TransactionStatusCancel  TransactionStatus = "cancel"
)

// Transaction domain errors
var (
	ErrTransactionNotFound      = errors.New("transaction not found")
	ErrTransactionAlreadyPaid   = errors.New("transaction has already been paid")
	ErrInvalidTransactionAmount = errors.New("transaction amount does not match plan price")
	ErrPlanNotAvailable         = errors.New("plan is not available for purchase")
	ErrActiveSubscriptionExists = errors.New("user already has an active subscription")
	ErrInvalidOrderID           = errors.New("invalid order id format")
)

// Transaction represents a payment transaction with Midtrans
// Fields with json:"-" are stored in DB but not exposed in API response for security
type Transaction struct {
	ID                uuid.UUID         `json:"id"`
	UserID            uuid.UUID         `json:"user_id"`
	PlanID            uuid.UUID         `json:"plan_id"`
	SubscriptionID    *uuid.UUID        `json:"subscription_id,omitempty"`
	OrderID           string            `json:"order_id"`
	TransactionID     *string           `json:"-"` // Hidden: Midtrans internal ID
	GrossAmount       decimal.Decimal   `json:"gross_amount"`
	PaymentType       *string           `json:"payment_type,omitempty"`
	PaymentMethod     *string           `json:"payment_method,omitempty"`
	Status            TransactionStatus `json:"status"`
	TransactionStatus *string           `json:"-"` // Hidden: Use Status field instead
	FraudStatus       *string           `json:"-"` // Hidden: Internal use only
	SnapToken         *string           `json:"-"` // Hidden: Only needed during payment init
	RedirectURL       *string           `json:"redirect_url,omitempty"`
	MidtransResponse  json.RawMessage   `json:"-"` // Hidden: Contains sensitive data (signature, merchant_id)
	PaidAt            *time.Time        `json:"paid_at,omitempty"`
	ExpiredAt         *time.Time        `json:"expired_at,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	DeletedAt         *time.Time        `json:"-"` // Hidden: Internal soft delete
	Plan              *Plan             `json:"plan,omitempty"`
	User              *User             `json:"-"` // Hidden: User data available via user context
}

// CreateTransactionRequest is the request payload for creating a transaction
type CreateTransactionRequest struct {
	PlanID uuid.UUID `json:"plan_id" validate:"required"`
}

// TransactionResponse is the response returned after creating a transaction
type TransactionResponse struct {
	Transaction *Transaction `json:"transaction"`
	SnapToken   string       `json:"snap_token"`
	RedirectURL string       `json:"redirect_url"`
}

// PaginatedTransactions contains a list of transactions with pagination info
type PaginatedTransactions struct {
	Transactions []Transaction `json:"transactions"`
	Pagination   Pagination    `json:"pagination"`
}

// MidtransWebhookPayload represents the notification payload from Midtrans
type MidtransWebhookPayload struct {
	TransactionTime   string `json:"transaction_time"`
	TransactionStatus string `json:"transaction_status"`
	TransactionID     string `json:"transaction_id"`
	StatusMessage     string `json:"status_message"`
	StatusCode        string `json:"status_code"`
	SignatureKey      string `json:"signature_key"`
	SettlementTime    string `json:"settlement_time"`
	PaymentType       string `json:"payment_type"`
	OrderID           string `json:"order_id"`
	MerchantID        string `json:"merchant_id"`
	GrossAmount       string `json:"gross_amount"`
	FraudStatus       string `json:"fraud_status"`
	Currency          string `json:"currency"`
}

// TransactionRepository defines the interface for transaction data access
type TransactionRepository interface {
	Create(ctx context.Context, transaction *Transaction) error
	FindByID(ctx context.Context, id uuid.UUID) (*Transaction, error)
	FindByOrderID(ctx context.Context, orderID string) (*Transaction, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Transaction, error)
	CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	Update(ctx context.Context, transaction *Transaction) error
	UpdateStatus(ctx context.Context, orderID string, status TransactionStatus, midtransResponse json.RawMessage) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// TransactionService defines the interface for transaction business logic
type TransactionService interface {
	// CreateTransaction creates a new transaction and returns Snap token for payment
	CreateTransaction(ctx context.Context, userID uuid.UUID, req *CreateTransactionRequest) (*TransactionResponse, error)
	// GetByID retrieves a transaction by ID for a specific user
	GetByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*Transaction, error)
	// GetByOrderID retrieves a transaction by order ID
	GetByOrderID(ctx context.Context, orderID string) (*Transaction, error)
	// GetUserTransactions retrieves all transactions for a user with pagination
	GetUserTransactions(ctx context.Context, userID uuid.UUID, page, limit int) (*PaginatedTransactions, error)
	// HandleWebhook processes Midtrans webhook notification
	HandleWebhook(ctx context.Context, payload map[string]interface{}) error
	// CheckTransactionStatus manually checks transaction status from Midtrans
	CheckTransactionStatus(ctx context.Context, orderID string) (*Transaction, error)
}
