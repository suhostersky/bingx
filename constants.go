package bingx

// Order types
const (
	OrderTypeMarket           = "MARKET"
	OrderTypeLimit            = "LIMIT"
	OrderTypeStopMarket       = "STOP_MARKET"
	OrderTypeStop             = "STOP"
	OrderTypeTakeProfitMarket = "TAKE_PROFIT_MARKET"
	OrderTypeTakeProfit       = "TAKE_PROFIT"
	OrderTypeTriggerLimit     = "TRIGGER_LIMIT"
	OrderTypeTriggerMarket    = "TRIGGER_MARKET"
)

// Order sides
const (
	SideBuy  = "BUY"
	SideSell = "SELL"
)

// Position sides
const (
	PositionSideLong  = "LONG"
	PositionSideShort = "SHORT"
)

// Working types
const (
	WorkingTypeMarkPrice     = "MARK_PRICE"     // Recommended: uses mark price to prevent manipulation
	WorkingTypeContractPrice = "CONTRACT_PRICE" // Uses last traded price
)

// Time in force options
const (
	TimeInForceGTC = "GTC" // Good Till Cancel
	TimeInForceIOC = "IOC" // Immediate or Cancel
	TimeInForceFOK = "FOK" // Fill or Kill
	TimeInForceGTX = "GTX" // Good Till Crossing (Post Only)
)

// Order status
const (
	OrderStatusNew             = "NEW"
	OrderStatusPartiallyFilled = "PARTIALLY_FILLED"
	OrderStatusFilled          = "FILLED"
	OrderStatusCanceled        = "CANCELED"
	OrderStatusRejected        = "REJECTED"
	OrderStatusExpired         = "EXPIRED"
)

// Contract status
const (
	ContractStatusOffline = 0
	ContractStatusOnline  = 1
)

// Boolean string values
const (
	BoolTrue  = "true"
	BoolFalse = "false"
)

// Stop guaranteed values
const (
	StopGuaranteedTrue  = "TRUE"
	StopGuaranteedFalse = "FALSE"
)
