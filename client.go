// Package bingx provides a client for interacting with the BingX Perpetual Swap API.
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

// Client represents a BingX API client.
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
	Symbol       string  `json:"symbol"`                 // Trading pair, e.g. "BTC-USDT"
	Type         string  `json:"type"`                   // Order type: MARKET, LIMIT, STOP_MARKET, STOP, TAKE_PROFIT_MARKET, TAKE_PROFIT, TRIGGER_LIMIT, TRIGGER_MARKET
	Side         string  `json:"side"`                   // Order side: BUY, SELL
	PositionSide string  `json:"positionSide,omitempty"` // Position side: LONG, SHORT (required for hedge mode)
	ReduceOnly   string  `json:"reduceOnly,omitempty"`   // Reduce only flag: true, false
	Price        float64 `json:"price,omitempty"`        // Order price (required for LIMIT orders)
	Quantity     float64 `json:"quantity,omitempty"`     // Order quantity
	StopPrice    float64 `json:"stopPrice,omitempty"`    // Stop price for stop orders
	PriceRate    float64 `json:"priceRate,omitempty"`    // Price rate for trailing stop orders
	StopLoss     string  `json:"stopLoss,omitempty"`     // Stop loss parameters in JSON format
	TakeProfit   string  `json:"takeProfit,omitempty"`   // Take profit parameters in JSON format
	// WorkingType specifies the price type for triggers: MARK_PRICE (mark price), CONTRACT_PRICE (last price)
	// MARK_PRICE is recommended to prevent manipulation
	WorkingType     string  `json:"workingType,omitempty"`
	ClientOrderID   string  `json:"clientOrderId,omitempty"`   // Custom order ID
	RecvWindow      int64   `json:"recvWindow,omitempty"`      // Request validity window in milliseconds
	TimeInForce     string  `json:"timeInForce,omitempty"`     // Time in force: GTC (Good Till Cancel), IOC (Immediate or Cancel), FOK (Fill or Kill), GTX (Good Till Crossing)
	ClosePosition   string  `json:"closePosition,omitempty"`   // Close position flag: true, false
	ActivationPrice float64 `json:"activationPrice,omitempty"` // Activation price for trailing stop orders
	StopGuaranteed  string  `json:"stopGuaranteed,omitempty"`  // Guaranteed stop flag: TRUE, FALSE
	PositionID      int64   `json:"positionId,omitempty"`      // Position ID for closing specific position
}

// PlaceOrderResponse represents the response from placing an order.
type PlaceOrderResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Symbol         string `json:"symbol"`         // Trading pair
		Side           string `json:"side"`           // Order side: BUY, SELL
		Type           string `json:"type"`           // Order type
		PositionSide   string `json:"positionSide"`   // Position side: LONG, SHORT
		ReduceOnly     string `json:"reduceOnly"`     // Reduce only flag
		OrderID        string `json:"orderId"`        // Order ID
		WorkingType    string `json:"workingType"`    // Working type: MARK_PRICE, CONTRACT_PRICE
		ClientOrderID  string `json:"clientOrderId"`  // Client order ID
		StopGuaranteed string `json:"stopGuaranteed"` // Guaranteed stop flag
		Status         string `json:"status"`         // Order status: NEW, PARTIALLY_FILLED, FILLED, CANCELED, REJECTED, EXPIRED
		AvgPrice       string `json:"avgPrice"`       // Average execution price
		ExecutedQty    string `json:"executedQty"`    // Executed quantity
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
	Symbol     string `json:"symbol"`               // Trading pair, e.g. "BTC-USDT"
	RecvWindow int64  `json:"recvWindow,omitempty"` // Request validity window in milliseconds
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
	Symbol   string `json:"symbol"`   // Trading pair, e.g. "BTC-USDT"
	Side     string `json:"side"`     // Position side: LONG, SHORT
	Leverage int    `json:"leverage"` // Leverage value (1-125 depending on symbol)
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
	Symbol string `json:"symbol"` // Trading pair, e.g. "BTC-USDT"
	Price  string `json:"price"`  // Current price
	Time   int64  `json:"time"`   // Timestamp in milliseconds
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

// Contract represents detailed contract information.
type Contract struct {
	ContractID        string  `json:"contractId"`        // Contract ID
	Symbol            string  `json:"symbol"`            // Trading pair, e.g. "BTC-USDT"
	QuantityPrecision int     `json:"quantityPrecision"` // Quantity decimal precision
	PricePrecision    int     `json:"pricePrecision"`    // Price decimal precision
	TakerFeeRate      float64 `json:"takerFeeRate"`      // Taker fee rate
	MakerFeeRate      float64 `json:"makerFeeRate"`      // Maker fee rate
	TradeMinQuantity  float64 `json:"tradeMinQuantity"`  // Minimum trade quantity
	TradeMinUSDT      float64 `json:"tradeMinUSDT"`      // Minimum trade value in USDT
	Currency          string  `json:"currency"`          // Quote currency (usually USDT)
	Asset             string  `json:"asset"`             // Base asset (e.g. BTC, ETH)
	Status            int     `json:"status"`            // Contract status: 0 (offline), 1 (online)
	APIStateOpen      string  `json:"apiStateOpen"`      // API open state
	APIStateClose     string  `json:"apiStateClose"`     // API close state
	EnsureTrigger     bool    `json:"ensureTrigger"`     // Ensure trigger flag
	TriggerFeeRate    string  `json:"triggerFeeRate"`    // Trigger order fee rate
	BrokerState       bool    `json:"brokerState"`       // Broker state
	LaunchTime        int64   `json:"launchTime"`        // Contract launch timestamp
	MaintainTime      int64   `json:"maintainTime"`      // Last maintenance timestamp
	OffTime           int64   `json:"offTime"`           // Contract offline timestamp
	DisplayName       string  `json:"displayName"`       // Display name
}

// GetContractsResponse represents the response from getting contracts information.
type GetContractsResponse struct {
	Code int        `json:"code"`
	Msg  string     `json:"msg"`
	Data []Contract `json:"data"`
}

// GetContracts retrieves detailed information about all available perpetual swap contracts.
func (c *Client) GetContracts(ctx context.Context) (*GetContractsResponse, error) {
	var resp GetContractsResponse
	if err := c.doRequest(ctx, http.MethodGet, "/openApi/swap/v2/quote/contracts", nil, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// CloseAllPositionsRequest represents parameters for closing all positions.
type CloseAllPositionsRequest struct {
	Symbol     string `json:"symbol"`               // Trading pair, e.g. "BTC-USDT"
	RecvWindow int64  `json:"recvWindow,omitempty"` // Request validity window in milliseconds
}

// CloseAllPositionsResponse represents the response from closing all positions.
type CloseAllPositionsResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Success bool `json:"success"` // Operation success flag
	} `json:"data"`
}

// CloseAllPositions closes all positions for a given symbol.
func (c *Client) CloseAllPositions(ctx context.Context, req CloseAllPositionsRequest) (*CloseAllPositionsResponse, error) {
	params, err := structToMap(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	var resp CloseAllPositionsResponse
	if err := c.doRequest(ctx, http.MethodPost, "/openApi/swap/v2/trade/closeAllPositions", params, &resp); err != nil {
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
		if v == nil || v == "" || v == 0.0 || v == int64(0) {
			delete(result, k)
		}
	}

	return result, nil
}
