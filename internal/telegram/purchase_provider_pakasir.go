package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type pakasirGateway struct {
	baseURL     string
	projectSlug string
	apiKey      string
	httpClient  *http.Client
}

func NewPakasirGateway(baseURL string, projectSlug string, apiKey string) PaymentGateway {
	if strings.TrimSpace(projectSlug) == "" || strings.TrimSpace(apiKey) == "" {
		return nil
	}

	normalizedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if normalizedBaseURL == "" {
		normalizedBaseURL = "https://app.pakasir.com"
	}

	return &pakasirGateway{
		baseURL:     normalizedBaseURL,
		projectSlug: strings.TrimSpace(projectSlug),
		apiKey:      strings.TrimSpace(apiKey),
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (g *pakasirGateway) CreateQris(ctx context.Context, req PakasirCreateTransactionRequest) (*PakasirTransaction, error) {
	body := map[string]any{
		"project":  g.projectSlug,
		"order_id": req.OrderID,
		"amount":   req.Amount,
		"api_key":  g.apiKey,
	}

	respBody := struct {
		Payment struct {
			Project       string    `json:"project"`
			OrderID       string    `json:"order_id"`
			Amount        int64     `json:"amount"`
			Fee           int64     `json:"fee"`
			TotalPayment  int64     `json:"total_payment"`
			PaymentMethod string    `json:"payment_method"`
			PaymentNumber string    `json:"payment_number"`
			ExpiredAt     time.Time `json:"expired_at"`
		} `json:"payment"`
	}{}

	if err := g.doJSON(ctx, http.MethodPost, g.baseURL+"/api/transactioncreate/qris", body, &respBody); err != nil {
		return nil, err
	}

	return &PakasirTransaction{
		Provider:     "pakasir",
		OrderID:      respBody.Payment.OrderID,
		Amount:       respBody.Payment.Amount,
		Fee:          respBody.Payment.Fee,
		TotalPayment: respBody.Payment.TotalPayment,
		QRString:     respBody.Payment.PaymentNumber,
		ExpiredAt:    respBody.Payment.ExpiredAt,
		Raw: map[string]any{
			"project":        respBody.Payment.Project,
			"order_id":       respBody.Payment.OrderID,
			"amount":         respBody.Payment.Amount,
			"fee":            respBody.Payment.Fee,
			"total_payment":  respBody.Payment.TotalPayment,
			"payment_method": respBody.Payment.PaymentMethod,
			"payment_number": respBody.Payment.PaymentNumber,
			"expired_at":     respBody.Payment.ExpiredAt,
		},
	}, nil
}

func (g *pakasirGateway) VerifyTransaction(ctx context.Context, req PakasirVerifyTransactionRequest) (*PakasirVerifiedTransaction, error) {
	params := url.Values{}
	params.Set("project", g.projectSlug)
	params.Set("amount", fmt.Sprintf("%d", req.Amount))
	params.Set("order_id", req.OrderID)
	params.Set("api_key", g.apiKey)

	verifyURL := g.baseURL + "/api/transactiondetail?" + params.Encode()
	respBody := struct {
		Transaction struct {
			Amount        int64  `json:"amount"`
			OrderID       string `json:"order_id"`
			Project       string `json:"project"`
			Status        string `json:"status"`
			PaymentMethod string `json:"payment_method"`
			CompletedAt   string `json:"completed_at"`
		} `json:"transaction"`
	}{}

	if err := g.doJSON(ctx, http.MethodGet, verifyURL, nil, &respBody); err != nil {
		return nil, err
	}

	completedAt, _ := time.Parse(time.RFC3339, respBody.Transaction.CompletedAt)

	return &PakasirVerifiedTransaction{
		OrderID:       respBody.Transaction.OrderID,
		Amount:        respBody.Transaction.Amount,
		Status:        respBody.Transaction.Status,
		PaymentMethod: respBody.Transaction.PaymentMethod,
		CompletedAt:   completedAt,
		Raw: map[string]any{
			"amount":         respBody.Transaction.Amount,
			"order_id":       respBody.Transaction.OrderID,
			"project":        respBody.Transaction.Project,
			"status":         respBody.Transaction.Status,
			"payment_method": respBody.Transaction.PaymentMethod,
			"completed_at":   respBody.Transaction.CompletedAt,
		},
	}, nil
}

func (g *pakasirGateway) doJSON(ctx context.Context, method string, endpoint string, payload any, out any) error {
	var bodyReader *bytes.Reader
	if payload == nil {
		bodyReader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pakasir request failed with status %d", resp.StatusCode)
	}

	if out == nil {
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
