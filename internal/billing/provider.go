package billing

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Status transaksi gateway TERNORMALISASI (docs/09-billing.md "settlement/
// capture -> payment paid").
const (
	TransactionStatusPending = "pending"
	TransactionStatusPaid    = "paid"
	TransactionStatusFailed  = "failed"
)

// TransactionRequest adalah parameter PaymentProvider.CreateTransaction.
type TransactionRequest struct {
	OrderID      string
	Amount       int64
	ItemName     string
	CustomerName string
}

// PaymentProvider mengabstraksi gateway pembayaran — SATU provider
// diimplementasikan dulu (Midtrans Snap, docs/09-billing.md "Pilih SATU
// provider gateway dulu ... abstraksi PaymentProvider interface membuat
// penambahan provider kedua murah").
type PaymentProvider interface {
	// Name — nilai kanonik disimpan di payments.provider (mis. "midtrans").
	Name() string
	// CreateTransaction membuat transaksi Snap, mengembalikan redirect_url checkout.
	CreateTransaction(ctx context.Context, req TransactionRequest) (redirectURL string, err error)
	// VerifySignature memverifikasi signature_key webhook: sha512(order_id+status_code+gross_amount+ServerKey).
	VerifySignature(orderID, statusCode, grossAmount, signatureKey string) bool
	// TransactionStatus menerjemahkan transaction_status (+fraud_status utk
	// kartu kredit) Midtrans menjadi status ternormalisasi di atas.
	TransactionStatus(transactionStatus, fraudStatus string) string
}

// MidtransProvider — implementasi PaymentProvider utk Midtrans Snap API.
type MidtransProvider struct {
	serverKey string
	baseURL   string // Snap API base, sandbox atau production
	client    *http.Client
}

// NewMidtransProvider membangun provider dari MIDTRANS_SERVER_KEY +
// MIDTRANS_ENV (sandbox|production, default sandbox).
func NewMidtransProvider(serverKey, env string) *MidtransProvider {
	base := "https://app.sandbox.midtrans.com/snap/v1"
	if env == "production" {
		base = "https://app.midtrans.com/snap/v1"
	}
	return &MidtransProvider{serverKey: serverKey, baseURL: base, client: http.DefaultClient}
}

// Configured melaporkan apakah MIDTRANS_SERVER_KEY terisi (pola yang sama
// dengan notification.Provider.Configurable — lihat internal/notification/model.go).
func (p *MidtransProvider) Configured() bool { return p.serverKey != "" }

func (p *MidtransProvider) Name() string { return "midtrans" }

type snapTransactionDetails struct {
	OrderID     string `json:"order_id"`
	GrossAmount int64  `json:"gross_amount"`
}

type snapItemDetail struct {
	ID       string `json:"id"`
	Price    int64  `json:"price"`
	Quantity int    `json:"quantity"`
	Name     string `json:"name"`
}

type snapCustomerDetails struct {
	FirstName string `json:"first_name"`
}

type snapCreateRequest struct {
	TransactionDetails snapTransactionDetails `json:"transaction_details"`
	ItemDetails        []snapItemDetail       `json:"item_details"`
	CustomerDetails    snapCustomerDetails    `json:"customer_details"`
}

type snapCreateResponse struct {
	Token       string   `json:"token"`
	RedirectURL string   `json:"redirect_url"`
	ErrorMsgs   []string `json:"error_messages"`
}

// CreateTransaction — POST {baseURL}/transactions (Basic Auth: ServerKey:"").
func (p *MidtransProvider) CreateTransaction(ctx context.Context, req TransactionRequest) (string, error) {
	if !p.Configured() {
		return "", errGatewayNotConfigured
	}
	custName := req.CustomerName
	if custName == "" {
		custName = "Sekolah"
	}
	body := snapCreateRequest{
		TransactionDetails: snapTransactionDetails{OrderID: req.OrderID, GrossAmount: req.Amount},
		ItemDetails:        []snapItemDetail{{ID: "subscription", Price: req.Amount, Quantity: 1, Name: req.ItemName}},
		CustomerDetails:    snapCustomerDetails{FirstName: custName},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/transactions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(p.serverKey+":")))

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var parsed snapCreateResponse
	_ = json.Unmarshal(raw, &parsed)
	if resp.StatusCode >= 300 || parsed.RedirectURL == "" {
		msg := "gateway pembayaran gagal membuat transaksi"
		if len(parsed.ErrorMsgs) > 0 {
			msg = parsed.ErrorMsgs[0]
		}
		return "", fmt.Errorf("billing: midtrans create transaction (status %d): %s", resp.StatusCode, msg)
	}
	return parsed.RedirectURL, nil
}

// VerifySignature — sha512(order_id+status_code+gross_amount+ServerKey)
// (docs/09-billing.md "verifikasi signature!").
func (p *MidtransProvider) VerifySignature(orderID, statusCode, grossAmount, signatureKey string) bool {
	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + p.serverKey))
	expected := hex.EncodeToString(sum[:])
	return expected == signatureKey
}

// TransactionStatus — Midtrans: settlement/capture(+fraud accept) = paid;
// pending = pending; deny/cancel/expire/refund/capture(+fraud challenge) = failed.
func (p *MidtransProvider) TransactionStatus(transactionStatus, fraudStatus string) string {
	switch transactionStatus {
	case "capture":
		if fraudStatus == "" || fraudStatus == "accept" {
			return TransactionStatusPaid
		}
		return TransactionStatusFailed
	case "settlement":
		return TransactionStatusPaid
	case "pending":
		return TransactionStatusPending
	default: // deny, cancel, expire, refund, partial_refund
		return TransactionStatusFailed
	}
}
