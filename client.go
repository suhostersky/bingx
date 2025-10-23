package bingx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://open-api.bingx.com"
	apiKeyHeader   = "X-BX-APIKEY"
)

var (
	ErrInvalidCredentials = errors.New("bingx: invalid API credentials")
	ErrInvalidResponse    = errors.New("bingx: invalid API response")
	ErrRequestFailed      = errors.New("bingx: request failed")
)

// Client represents a BingX API client
type Client struct {
	apiKey     string
	apiSecret  string
	baseURL    string
	httpClient *http.Client
}

// Config holds configuration for creating a new Client.
type Config struct {
	APIKey     string
	APISecret  string
	BaseURL    string       // optional, defaults to defaultBaseURL
	HTTPClient *http.Client // optional, defaults to http.DefaultClient
}

// NewClient creates a new BingX API client.
func NewClient(cfg Config) (*Client, error) {
	if cfg.APIKey == "" || cfg.APISecret == "" {
		return nil, ErrInvalidCredentials
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		apiKey:     cfg.APIKey,
		apiSecret:  cfg.APISecret,
		baseURL:    baseURL,
		httpClient: httpClient,
	}, nil
}

// PlaceOrderRequest represents parameters for placing an order.
type PlaceOrderRequest struct {
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`         // BUY or SELL
	PositionSide string  `json:"positionSide"` // LONG or SHORT
	Type         string  `json:"type"`         // MARKET, LIMIT, etc.
	Quantity     float64 `json:"quantity"`
	Price        float64 `json:"price,omitempty"`
	TakeProfit   string  `json:"takeProfit,omitempty"`
	StopLoss     string  `json:"stopLoss,omitempty"`
}

// PlaceOrderResponse represents the response from placing an order.
type PlaceOrderResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		OrderID int64 `json:"orderId"`
	} `json:"data"`
}

// PlaceOrder places a new order on BingX.
func (c *Client) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*PlaceOrderResponse, error) {
	params, err := structToMap(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	var resp PlaceOrderResponse
	if err := c.doRequest(ctx, http.MethodPost, "/openApi/swap/v2/trade/order", params, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// CancelAllOrdersRequest represents parameters for canceling all orders.
type CancelAllOrdersRequest struct {
	Symbol     string `json:"symbol"`
	RecvWindow int64  `json:"recvWindow,omitempty"`
}

// CancelAllOrdersResponse represents the response from canceling all orders.
type CancelAllOrdersResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// CancelAllOrders cancels all open orders for a symbol.
func (c *Client) CancelAllOrders(ctx context.Context, req CancelAllOrdersRequest) (*CancelAllOrdersResponse, error) {
	params, err := structToMap(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	var resp CancelAllOrdersResponse
	if err := c.doRequest(ctx, http.MethodDelete, "/openApi/swap/v2/trade/allOpenOrders", params, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// SetLeverageRequest represents parameters for setting leverage.
type SetLeverageRequest struct {
	Symbol   string `json:"symbol"`
	Side     string `json:"side"` // LONG or SHORT
	Leverage int    `json:"leverage"`
}

// SetLeverageResponse represents the response from setting leverage.
type SetLeverageResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// SetLeverage sets the leverage for a symbol and side.
func (c *Client) SetLeverage(ctx context.Context, req SetLeverageRequest) (*SetLeverageResponse, error) {
	params, err := structToMap(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	var resp SetLeverageResponse
	if err := c.doRequest(ctx, http.MethodPost, "/openApi/swap/v2/trade/leverage", params, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// TickerPrice represents ticker price information.
type TickerPrice struct {
	Symbol string  `json:"symbol"`
	Price  float64 `json:"price,string"`
	Time   int64   `json:"time"`
}

// ListSymbolsResponse represents the response from listing symbols.
type ListSymbolsResponse struct {
	Code int           `json:"code"`
	Msg  string        `json:"msg"`
	Data []TickerPrice `json:"data"`
}

// ListSymbols retrieves the list of available trading symbols with their prices.
func (c *Client) ListSymbols(ctx context.Context) (*ListSymbolsResponse, error) {
	var resp ListSymbolsResponse
	if err := c.doRequest(ctx, http.MethodGet, "/openApi/swap/v1/ticker/price", nil, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// doRequest performs an authenticated API request.
func (c *Client) doRequest(ctx context.Context, method, endpoint string, params map[string]interface{}, result interface{}) error {
	if params == nil {
		params = make(map[string]interface{})
	}

	timestamp := time.Now().UnixMilli()
	params["timestamp"] = timestamp

	queryString := c.buildQueryString(params, false)
	signature := c.sign(queryString)

	needsEncoding := c.containsComplexValues(params)
	if needsEncoding {
		queryString = c.buildQueryString(params, true)
	}

	finalQuery := fmt.Sprintf("%s&signature=%s", queryString, signature)
	reqURL := fmt.Sprintf("%s%s?%s", c.baseURL, endpoint, finalQuery)

	req, err := http.NewRequestWithContext(ctx, method, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set(apiKeyHeader, c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d, body: %s", ErrRequestFailed, resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}

	return nil
}

// sign creates an HMAC SHA256 signature.
func (c *Client) sign(message string) string {
	h := hmac.New(sha256.New, []byte(c.apiSecret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// buildQueryString builds a query string from parameters.
func (c *Client) buildQueryString(params map[string]interface{}, urlEncode bool) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, key := range keys {
		value := fmt.Sprintf("%v", params[key])
		if urlEncode && c.isComplexValue(value) {
			value = url.QueryEscape(value)
			value = strings.ReplaceAll(value, "+", "%20")
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}

	return strings.Join(parts, "&")
}

// containsComplexValues checks if parameters contain complex JSON values.
func (c *Client) containsComplexValues(params map[string]interface{}) bool {
	for _, v := range params {
		value := fmt.Sprintf("%v", v)
		if c.isComplexValue(value) {
			return true
		}
	}
	return false
}

// isComplexValue checks if a string value contains JSON structures.
func (c *Client) isComplexValue(value string) bool {
	return strings.ContainsAny(value, "[{")
}

// structToMap converts a struct to a map[string]interface{}.
func structToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	// Remove nil/zero values
	for k, v := range result {
		if v == nil || v == "" || v == 0.0 {
			delete(result, k)
		}
	}

	return result, nil
}
