package midtrans

import (
	"crypto/sha512"
	"encoding/hex"
	"errors"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/midtrans/midtrans-go/snap"
)

// Config holds Midtrans configuration
type Config struct {
	ServerKey     string
	ClientKey     string
	IsSandbox     bool
	WebhookURL    string
	MerchantID    string
}

// Client wraps Midtrans SDK clients
type Client struct {
	config     Config
	snapClient snap.Client
	coreClient coreapi.Client
}

// NewClient creates a new Midtrans client with the provided configuration
func NewClient(cfg Config) *Client {
	// Determine environment based on sandbox flag
	env := midtrans.Sandbox
	if !cfg.IsSandbox {
		env = midtrans.Production
	}

	// Initialize Snap client for creating payment pages
	s := snap.Client{}
	s.New(cfg.ServerKey, env)

	// Initialize Core API client for checking transaction status
	c := coreapi.Client{}
	c.New(cfg.ServerKey, env)

	return &Client{
		config:     cfg,
		snapClient: s,
		coreClient: c,
	}
}

// ItemDetail represents an item in the transaction
type ItemDetail struct {
	ID       string
	Name     string
	Price    int64
	Quantity int32
}

// CustomerDetail represents customer information for the transaction
type CustomerDetail struct {
	FirstName string
	LastName  string
	Email     string
	Phone     string
}

// CreateTransactionRequest represents request to create a Snap transaction
type CreateTransactionRequest struct {
	OrderID         string
	GrossAmount     int64
	ItemDetails     []ItemDetail
	CustomerDetails CustomerDetail
}

// CreateTransactionResponse represents the response from Snap transaction creation
type CreateTransactionResponse struct {
	Token       string
	RedirectURL string
}

// TransactionStatusResponse represents the response from status check
type TransactionStatusResponse struct {
	TransactionID     string
	OrderID           string
	TransactionStatus string
	FraudStatus       string
	PaymentType       string
	GrossAmount       string
	TransactionTime   string
	SettlementTime    string
	StatusCode        string
	StatusMessage     string
}

// Errors that can be returned by the client
var (
	ErrNilResponse        = errors.New("received nil response from midtrans")
	ErrEmptyOrderID       = errors.New("order id is required")
	ErrTransactionFailed  = errors.New("failed to create transaction")
	ErrStatusCheckFailed  = errors.New("failed to check transaction status")
	ErrInvalidSignature   = errors.New("invalid webhook signature")
)

// CreateSnapTransaction creates a new Snap payment transaction
func (c *Client) CreateSnapTransaction(req CreateTransactionRequest) (*CreateTransactionResponse, error) {
	if req.OrderID == "" {
		return nil, ErrEmptyOrderID
	}

	// Build Midtrans item details
	itemDetails := make([]midtrans.ItemDetails, len(req.ItemDetails))
	for i, item := range req.ItemDetails {
		itemDetails[i] = midtrans.ItemDetails{
			ID:    item.ID,
			Name:  item.Name,
			Price: item.Price,
			Qty:   item.Quantity,
		}
	}

	// Build Snap request
	snapReq := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  req.OrderID,
			GrossAmt: req.GrossAmount,
		},
		CustomerDetail: &midtrans.CustomerDetails{
			FName: req.CustomerDetails.FirstName,
			LName: req.CustomerDetails.LastName,
			Email: req.CustomerDetails.Email,
			Phone: req.CustomerDetails.Phone,
		},
		Items: &itemDetails,
	}

	// Create Snap token
	snapResp, err := c.snapClient.CreateTransaction(snapReq)
	if err != nil {
		return nil, err
	}

	if snapResp == nil {
		return nil, ErrNilResponse
	}

	return &CreateTransactionResponse{
		Token:       snapResp.Token,
		RedirectURL: snapResp.RedirectURL,
	}, nil
}

// CheckTransaction checks the status of a transaction by order ID
func (c *Client) CheckTransaction(orderID string) (*TransactionStatusResponse, error) {
	if orderID == "" {
		return nil, ErrEmptyOrderID
	}

	resp, err := c.coreClient.CheckTransaction(orderID)
	if err != nil {
		return nil, err
	}

	if resp == nil {
		return nil, ErrNilResponse
	}

	return &TransactionStatusResponse{
		TransactionID:     resp.TransactionID,
		OrderID:           resp.OrderID,
		TransactionStatus: resp.TransactionStatus,
		FraudStatus:       resp.FraudStatus,
		PaymentType:       resp.PaymentType,
		GrossAmount:       resp.GrossAmount,
		TransactionTime:   resp.TransactionTime,
		SettlementTime:    resp.SettlementTime,
		StatusCode:        resp.StatusCode,
		StatusMessage:     resp.StatusMessage,
	}, nil
}

// VerifySignatureKey verifies the webhook signature from Midtrans
// Signature = SHA512(order_id+status_code+gross_amount+server_key)
func (c *Client) VerifySignatureKey(orderID, statusCode, grossAmount, signatureKey string) bool {
	rawSignature := orderID + statusCode + grossAmount + c.config.ServerKey
	hash := sha512.New()
	hash.Write([]byte(rawSignature))
	calculatedSignature := hex.EncodeToString(hash.Sum(nil))
	return calculatedSignature == signatureKey
}

// GetClientKey returns the client key for frontend use
func (c *Client) GetClientKey() string {
	return c.config.ClientKey
}

// IsSandbox returns whether the client is in sandbox mode
func (c *Client) IsSandbox() bool {
	return c.config.IsSandbox
}
