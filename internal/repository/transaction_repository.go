package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/raflytch/careerly-server/internal/domain"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	// Column definitions for transactions table
	transactionColumns = `
		id, user_id, plan_id, subscription_id, order_id, transaction_id, 
		gross_amount, payment_type, payment_method, status, transaction_status, 
		fraud_status, snap_token, redirect_url, midtrans_response, 
		paid_at, expired_at, created_at, updated_at, deleted_at
	`
)

type transactionRepository struct {
	db *sql.DB
}

// NewTransactionRepository creates a new transaction repository instance
func NewTransactionRepository(db *sql.DB) domain.TransactionRepository {
	return &transactionRepository{db: db}
}

// Create inserts a new transaction into the database
func (r *transactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	query := `
		INSERT INTO transactions (
			id, user_id, plan_id, subscription_id, order_id, transaction_id,
			gross_amount, payment_type, payment_method, status, transaction_status,
			fraud_status, snap_token, redirect_url, midtrans_response,
			paid_at, expired_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
	`

	// Handle nil MidtransResponse - convert to sql.NullString for PostgreSQL jsonb
	var midtransResp sql.NullString
	if len(tx.MidtransResponse) > 0 {
		midtransResp = sql.NullString{String: string(tx.MidtransResponse), Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query,
		tx.ID,
		tx.UserID,
		tx.PlanID,
		tx.SubscriptionID,
		tx.OrderID,
		tx.TransactionID,
		tx.GrossAmount,
		tx.PaymentType,
		tx.PaymentMethod,
		tx.Status,
		tx.TransactionStatus,
		tx.FraudStatus,
		tx.SnapToken,
		tx.RedirectURL,
		midtransResp,
		tx.PaidAt,
		tx.ExpiredAt,
		tx.CreatedAt,
		tx.UpdatedAt,
	)
	return err
}

// FindByID retrieves a transaction by its ID
func (r *transactionRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	query := `
		SELECT ` + transactionColumns + `
		FROM transactions
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanTransaction(r.db.QueryRowContext(ctx, query, id))
}

// FindByOrderID retrieves a transaction by its Midtrans order ID
func (r *transactionRepository) FindByOrderID(ctx context.Context, orderID string) (*domain.Transaction, error) {
	query := `
		SELECT ` + transactionColumns + `
		FROM transactions
		WHERE order_id = $1 AND deleted_at IS NULL
	`
	return r.scanTransaction(r.db.QueryRowContext(ctx, query, orderID))
}

// FindByUserID retrieves all transactions for a user with pagination
func (r *transactionRepository) FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Transaction, error) {
	query := `
		SELECT ` + transactionColumns + `
		FROM transactions
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	transactions := make([]domain.Transaction, 0)
	for rows.Next() {
		tx, err := r.scanTransactionFromRows(rows)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, *tx)
	}
	return transactions, rows.Err()
}

// CountByUserID counts total transactions for a user
func (r *transactionRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `SELECT COUNT(id) FROM transactions WHERE user_id = $1 AND deleted_at IS NULL`
	var count int64
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	return count, err
}

// Update updates an existing transaction
func (r *transactionRepository) Update(ctx context.Context, tx *domain.Transaction) error {
	query := `
		UPDATE transactions SET
			subscription_id = $1,
			transaction_id = $2,
			payment_type = $3,
			payment_method = $4,
			status = $5,
			transaction_status = $6,
			fraud_status = $7,
			snap_token = $8,
			redirect_url = $9,
			midtrans_response = $10,
			paid_at = $11,
			expired_at = $12,
			updated_at = $13
		WHERE id = $14 AND deleted_at IS NULL
	`

	// Handle nil MidtransResponse for PostgreSQL jsonb column
	var midtransResp sql.NullString
	if len(tx.MidtransResponse) > 0 {
		midtransResp = sql.NullString{String: string(tx.MidtransResponse), Valid: true}
	}

	tx.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, query,
		tx.SubscriptionID,
		tx.TransactionID,
		tx.PaymentType,
		tx.PaymentMethod,
		tx.Status,
		tx.TransactionStatus,
		tx.FraudStatus,
		tx.SnapToken,
		tx.RedirectURL,
		midtransResp,
		tx.PaidAt,
		tx.ExpiredAt,
		tx.UpdatedAt,
		tx.ID,
	)
	return err
}

// UpdateStatus updates the transaction status and stores Midtrans response (optimized for webhook)
func (r *transactionRepository) UpdateStatus(ctx context.Context, orderID string, status domain.TransactionStatus, midtransResponse json.RawMessage) error {
	query := `
		UPDATE transactions SET
			status = $1,
			midtrans_response = $2,
			updated_at = $3,
			paid_at = CASE WHEN $1 = 'success' THEN $3 ELSE paid_at END
		WHERE order_id = $4 AND deleted_at IS NULL
	`

	// Handle nil MidtransResponse
	var midtransResp sql.NullString
	if len(midtransResponse) > 0 {
		midtransResp = sql.NullString{String: string(midtransResponse), Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, status, midtransResp, time.Now(), orderID)
	return err
}

// SoftDelete marks a transaction as deleted
func (r *transactionRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE transactions
		SET deleted_at = $1, updated_at = $1
		WHERE id = $2 AND deleted_at IS NULL
	`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

// scanTransaction scans a single transaction from sql.Row
// Uses sql.NullString for nullable JSONB column to handle NULL values properly
func (r *transactionRepository) scanTransaction(row *sql.Row) (*domain.Transaction, error) {
	var tx domain.Transaction
	var grossAmountStr string
	var status string
	var midtransRespNull sql.NullString // Use NullString to handle NULL from jsonb

	err := row.Scan(
		&tx.ID,
		&tx.UserID,
		&tx.PlanID,
		&tx.SubscriptionID,
		&tx.OrderID,
		&tx.TransactionID,
		&grossAmountStr,
		&tx.PaymentType,
		&tx.PaymentMethod,
		&status,
		&tx.TransactionStatus,
		&tx.FraudStatus,
		&tx.SnapToken,
		&tx.RedirectURL,
		&midtransRespNull, // Scan into NullString
		&tx.PaidAt,
		&tx.ExpiredAt,
		&tx.CreatedAt,
		&tx.UpdatedAt,
		&tx.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse gross amount from string to decimal
	tx.GrossAmount, _ = decimal.NewFromString(grossAmountStr)
	tx.Status = domain.TransactionStatus(status)

	// Convert NullString to json.RawMessage if valid
	if midtransRespNull.Valid {
		tx.MidtransResponse = json.RawMessage(midtransRespNull.String)
	}

	return &tx, nil
}

// scanTransactionFromRows scans a transaction from sql.Rows (used in FindByUserID)
// Uses sql.NullString for nullable JSONB column to handle NULL values properly
func (r *transactionRepository) scanTransactionFromRows(rows *sql.Rows) (*domain.Transaction, error) {
	var tx domain.Transaction
	var grossAmountStr string
	var status string
	var midtransRespNull sql.NullString // Use NullString to handle NULL from jsonb

	err := rows.Scan(
		&tx.ID,
		&tx.UserID,
		&tx.PlanID,
		&tx.SubscriptionID,
		&tx.OrderID,
		&tx.TransactionID,
		&grossAmountStr,
		&tx.PaymentType,
		&tx.PaymentMethod,
		&status,
		&tx.TransactionStatus,
		&tx.FraudStatus,
		&tx.SnapToken,
		&tx.RedirectURL,
		&midtransRespNull, // Scan into NullString
		&tx.PaidAt,
		&tx.ExpiredAt,
		&tx.CreatedAt,
		&tx.UpdatedAt,
		&tx.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	tx.GrossAmount, _ = decimal.NewFromString(grossAmountStr)
	tx.Status = domain.TransactionStatus(status)

	// Convert NullString to json.RawMessage if valid
	if midtransRespNull.Valid {
		tx.MidtransResponse = json.RawMessage(midtransRespNull.String)
	}

	return &tx, nil
}
