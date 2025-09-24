package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/defi-maker/golighter/client"
	liteclient "github.com/elliottech/lighter-go/client"
	lighterTypes "github.com/elliottech/lighter-go/types"
	"github.com/elliottech/lighter-go/types/txtypes"
	"github.com/joho/godotenv"
)

// Config mirrors the knobs from the Python strategy while keeping sensible defaults.
type Config struct {
	BaseURL              string  // REST endpoint
	MarketSymbol         string  // e.g. PAXG
	Spread               float64 // fallback spread fraction (0.00035 == 0.035%)
	BaseAmount           float64 // fallback order size
	UseDynamicSizing     bool
	CapitalUsage         float64
	SafetyMargin         float64
	OrderTimeout         time.Duration
	ParamsDir            string
	AvellanedaRefresh    time.Duration
	RequireParams        bool
	CloseLongOnStartup   bool
	AccountRefresh       time.Duration
	MinPositionValueUSD  float64
	AvellanedaCandidates []string
}

type MarketInfo struct {
	ID        uint8
	Symbol    string
	PriceTick float64
	SizeTick  float64
}

type OrderState struct {
	ID         int64
	Side       string
	Price      float64
	BaseAmount float64
	PlacedAt   time.Time
	ReduceOnly bool
}

type avellanedaParams struct {
	LimitOrders struct {
		DeltaA float64 `json:"delta_a"`
		DeltaB float64 `json:"delta_b"`
	} `json:"limit_orders"`
}

type Strategy struct {
	cfg       Config
	http      *client.HTTPClient
	tx        *liteclient.TxClient
	market    MarketInfo
	accountID int64

	mu                sync.RWMutex
	midPrice          float64
	midPriceUpdatedAt time.Time

	availableCapital float64
	portfolioValue   float64
	positionSize     float64
	lastAccountFetch time.Time

	currentSide   string
	lastMidPrice  float64
	lastOrderBase float64
	openOrder     *OrderState

	avaParams      *avellanedaParams
	lastParamsLoad time.Time
}

func newStrategy(cfg Config, httpClient *client.HTTPClient, txClient *liteclient.TxClient, market MarketInfo, accountID int64) *Strategy {
	return &Strategy{
		cfg:          cfg,
		http:         httpClient,
		tx:           txClient,
		market:       market,
		accountID:    accountID,
		currentSide:  "buy",
		lastMidPrice: 0,
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("[market-maker] unable to load .env automatically: %v", err)
	}

	cfg := loadConfigFromEnv()

	httpClient := client.NewHTTPClient(cfg.BaseURL)
	if httpClient == nil {
		log.Fatalf("failed to create HTTP client for %s", cfg.BaseURL)
	}

	accountIndex := mustParseInt64("ACCOUNT_INDEX")
	apiKeyIndex := mustParseUint8("API_KEY_INDEX")
	privateKey := mustEnv("API_KEY_PRIVATE_KEY")

	liteHTTP := liteclient.NewHTTPClient(cfg.BaseURL)
	if liteHTTP == nil {
		log.Fatalf("failed to init lighter-go HTTP client for %s", cfg.BaseURL)
	}

	txClient, err := liteclient.NewTxClient(liteHTTP, privateKey, accountIndex, apiKeyIndex, 0)
	if err != nil {
		log.Fatalf("failed to init TxClient: %v", err)
	}

	market, err := discoverMarket(httpClient, cfg.MarketSymbol)
	if err != nil {
		log.Fatalf("unable to locate market %s: %v", cfg.MarketSymbol, err)
	}

	// Cancel any existing orders for a clean start.
	if err := cancelAllOrders(httpClient, txClient); err != nil {
		log.Printf("[market-maker] warn: failed to cancel existing orders: %v", err)
	}

	strategy := newStrategy(cfg, httpClient, txClient, market, accountIndex)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := strategy.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("strategy stopped with error: %v", err)
	}
}

// loadConfigFromEnv builds Config based on env variables, mirroring the Python defaults where possible.
func loadConfigFromEnv() Config {
	readBool := func(key string, def bool) bool {
		raw := os.Getenv(key)
		if raw == "" {
			return def
		}
		val, err := strconv.ParseBool(raw)
		if err != nil {
			return def
		}
		return val
	}

	readFloat := func(key string, def float64) float64 {
		raw := os.Getenv(key)
		if raw == "" {
			return def
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return def
		}
		return v
	}

	readDuration := func(key string, def time.Duration) time.Duration {
		raw := os.Getenv(key)
		if raw == "" {
			return def
		}
		d, err := time.ParseDuration(raw)
		if err != nil {
			return def
		}
		return d
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("LIGHTER_ENDPOINT")
	}
	if baseURL == "" {
		baseURL = "https://mainnet.zklighter.elliot.ai"
	}

	paramsDir := os.Getenv("PARAMS_DIR")
	if paramsDir == "" {
		paramsDir = "params"
	}

	cfg := Config{
		BaseURL:             baseURL,
		MarketSymbol:        getenvDefault("MARKET_SYMBOL", "PAXG"),
		Spread:              readFloat("SPREAD", 0.035/100.0),
		BaseAmount:          readFloat("BASE_AMOUNT", 0.047),
		UseDynamicSizing:    readBool("USE_DYNAMIC_SIZING", true),
		CapitalUsage:        readFloat("CAPITAL_USAGE_PERCENT", 0.99),
		SafetyMargin:        readFloat("SAFETY_MARGIN_PERCENT", 0.01),
		OrderTimeout:        readDuration("ORDER_TIMEOUT", 90*time.Second),
		ParamsDir:           paramsDir,
		AvellanedaRefresh:   readDuration("AVELLANEDA_REFRESH_INTERVAL", 15*time.Minute),
		RequireParams:       readBool("REQUIRE_PARAMS", false),
		CloseLongOnStartup:  readBool("CLOSE_LONG_ON_STARTUP", false),
		AccountRefresh:      readDuration("ACCOUNT_REFRESH_INTERVAL", 15*time.Second),
		MinPositionValueUSD: readFloat("MIN_POSITION_VALUE_USD", 15.0),
	}

	cfg.AvellanedaCandidates = []string{
		filepath.Join(cfg.ParamsDir, fmt.Sprintf("avellaneda_parameters_%s.json", cfg.MarketSymbol)),
		filepath.Join("params", fmt.Sprintf("avellaneda_parameters_%s.json", cfg.MarketSymbol)),
		fmt.Sprintf("avellaneda_parameters_%s.json", cfg.MarketSymbol),
		filepath.Join("TRADER", fmt.Sprintf("avellaneda_parameters_%s.json", cfg.MarketSymbol)),
	}

	return cfg
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env %s", key)
	}
	return v
}

func mustParseInt64(key string) int64 {
	raw := mustEnv(key)
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		log.Fatalf("invalid %s: %v", key, err)
	}
	return v
}

func mustParseUint8(key string) uint8 {
	raw := mustEnv(key)
	v, err := strconv.ParseUint(raw, 10, 8)
	if err != nil {
		log.Fatalf("invalid %s: %v", key, err)
	}
	return uint8(v)
}

func discoverMarket(httpClient *client.HTTPClient, symbol string) (MarketInfo, error) {
	books, err := httpClient.GetOrderBooks()
	if err != nil {
		return MarketInfo{}, fmt.Errorf("get order books: %w", err)
	}
	upper := strings.ToUpper(symbol)
	for _, ob := range books.OrderBooks {
		if strings.ToUpper(ob.Symbol) == upper {
			priceTick := math.Pow(10, -float64(ob.SupportedPriceDecimals))
			sizeTick := math.Pow(10, -float64(ob.SupportedSizeDecimals))
			return MarketInfo{
				ID:        ob.MarketId,
				Symbol:    ob.Symbol,
				PriceTick: priceTick,
				SizeTick:  sizeTick,
			}, nil
		}
	}
	return MarketInfo{}, fmt.Errorf("symbol %s not found", symbol)
}

func cancelAllOrders(httpClient *client.HTTPClient, txClient *liteclient.TxClient) error {
	req := &lighterTypes.CancelAllOrdersTxReq{
		TimeInForce: txtypes.ImmediateCancelAll,
		Time:        time.Now().Add(5 * time.Minute).UnixMilli(),
	}
	txInfo, err := txClient.GetCancelAllOrdersTransaction(req, nil)
	if err != nil {
		return err
	}
	_, err = httpClient.SendRawTx(txInfo)
	return err
}

// Run orchestrates the strategy lifecycle.
func (s *Strategy) Run(ctx context.Context) error {
	log.Printf("[market-maker] starting for %s (id=%d)", s.market.Symbol, s.market.ID)

	// Start websocket subscription for order book updates.
	wsCtx, wsCancel := context.WithCancel(ctx)
	defer wsCancel()

	publicSvc := client.NewLighterWebsocketPublicService(nil)
	if err := publicSvc.Start(wsCtx, func(err error) {
		log.Printf("[market-maker] websocket error: %v", err)
	}); err != nil {
		return fmt.Errorf("start websocket: %w", err)
	}
	defer func() {
		_ = publicSvc.Close()
	}()

	_, err := publicSvc.SubscribeOrderBook(client.LighterOrderBookParamKey{MarketId: s.market.ID}, func(resp client.LighterOrderBookResponse) error {
		s.handleOrderBook(resp)
		return nil
	})
	if err != nil {
		return fmt.Errorf("subscribe order book: %w", err)
	}

	// Wait for initial data.
	if err := s.waitForInitialData(ctx); err != nil {
		return err
	}

	if s.cfg.CloseLongOnStartup {
		if err := s.closeExistingLong(ctx); err != nil {
			return fmt.Errorf("close-long: %w", err)
		}
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.step(ctx); err != nil {
				log.Printf("[market-maker] step error: %v", err)
			}
		}
	}
}

func (s *Strategy) handleOrderBook(resp client.LighterOrderBookResponse) {
	if len(resp.Bids) == 0 || len(resp.Asks) == 0 {
		return
	}
	bestBid, err1 := strconv.ParseFloat(resp.Bids[0].Price, 64)
	bestAsk, err2 := strconv.ParseFloat(resp.Asks[0].Price, 64)
	if err1 != nil || err2 != nil || bestBid <= 0 || bestAsk <= 0 {
		return
	}

	mid := (bestBid + bestAsk) / 2

	s.mu.Lock()
	s.midPrice = mid
	s.midPriceUpdatedAt = time.Now()
	s.mu.Unlock()
}

func (s *Strategy) waitForInitialData(ctx context.Context) error {
	deadline := time.After(30 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return errors.New("timeout waiting for initial mid price")
		default:
		}
		mid, fresh := s.getMidPrice()
		if fresh && mid > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if err := s.refreshAccount(); err != nil {
		return fmt.Errorf("initial account fetch: %w", err)
	}

	log.Printf("[market-maker] account ready: available=%.2f portfolio=%.2f position=%.6f", s.availableCapital, s.portfolioValue, s.positionSize)
	return nil
}

func (s *Strategy) closeExistingLong(ctx context.Context) error {
	s.mu.RLock()
	pos := s.positionSize
	s.mu.RUnlock()

	if pos <= 0 {
		return nil
	}
	mid, fresh := s.getMidPrice()
	if !fresh || mid <= 0 {
		return errors.New("mid price unavailable for close-long")
	}

	price := mid * (1 + s.cfg.Spread)
	if err := s.placeOrder("sell", price, pos, true); err != nil {
		return fmt.Errorf("place reduce-only sell: %w", err)
	}
	log.Printf("[market-maker] reduce-only sell placed to close initial position")

	// Wait for position to reach ~0 or timeout.
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return errors.New("timed out waiting for position to close")
		case <-ticker.C:
			if err := s.refreshAccount(); err != nil {
				log.Printf("[market-maker] refresh account during close-long: %v", err)
				continue
			}
			s.mu.RLock()
			remaining := s.positionSize
			s.mu.RUnlock()
			if math.Abs(remaining) < 1e-9 {
				log.Printf("[market-maker] position closed successfully")
				s.setSide("buy")
				return nil
			}
		}
	}
}

func (s *Strategy) setSide(side string) {
	s.mu.Lock()
	s.currentSide = side
	s.mu.Unlock()
}

func (s *Strategy) getMidPrice() (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if time.Since(s.midPriceUpdatedAt) > 10*time.Second {
		return 0, false
	}
	return s.midPrice, true
}

func (s *Strategy) step(ctx context.Context) error {
	mid, fresh := s.getMidPrice()
	if !fresh || mid <= 0 {
		return errors.New("stale mid price")
	}

	if time.Since(s.lastAccountFetch) > s.cfg.AccountRefresh {
		if err := s.refreshAccount(); err != nil {
			log.Printf("[market-maker] refresh account error: %v", err)
		}
	}

	s.mu.RLock()
	side := s.currentSide
	lastMid := s.lastMidPrice
	order := s.openOrder
	pos := s.positionSize
	s.mu.RUnlock()

	priceChanged := false
	if lastMid > 0 {
		priceChanged = math.Abs(mid-lastMid)/lastMid > 0.001
	}

	targetPrice, ok := s.calculateTargetPrice(mid, side)
	if !ok {
		return nil
	}

	spreadPct := ((targetPrice - mid) / mid) * 100
	log.Printf("[market-maker] mid=%.4f side=%s target=%.4f (%+.4f%%) priceChanged=%v", mid, side, targetPrice, spreadPct, priceChanged)

	if order != nil {
		if priceChanged {
			if err := s.cancelAll(); err != nil {
				log.Printf("[market-maker] cancel error: %v", err)
			}
			order = nil
		} else if time.Since(order.PlacedAt) > s.cfg.OrderTimeout {
			log.Printf("[market-maker] order %d timeout reached -> cancel", order.ID)
			if err := s.cancelAll(); err != nil {
				log.Printf("[market-maker] cancel error: %v", err)
			}
			order = nil
			if err := s.refreshAccount(); err != nil {
				log.Printf("[market-maker] refresh after cancel error: %v", err)
			}
			s.evaluatePostCycle(mid)
			return nil
		}
	}

	if order != nil {
		return nil
	}

	baseAmount := s.determineBaseAmount(side, mid, pos)
	if baseAmount <= 0 {
		if side == "buy" {
			log.Printf("[market-maker] computed buy size <= 0, skipping")
		}
		return nil
	}

	reduceOnly := side == "sell"
	if err := s.placeOrder(side, targetPrice, baseAmount, reduceOnly); err != nil {
		return fmt.Errorf("place order: %w", err)
	}

	s.mu.Lock()
	s.lastMidPrice = mid
	s.lastOrderBase = baseAmount
	s.mu.Unlock()
	return nil
}

func (s *Strategy) determineBaseAmount(side string, mid, position float64) float64 {
	if side == "sell" {
		if position <= 0 {
			return 0
		}
		value := position * mid
		if value < s.cfg.MinPositionValueUSD {
			log.Printf("[market-maker] position value %.2f < %.2f -> skip sell", value, s.cfg.MinPositionValueUSD)
			return 0
		}
		return position
	}

	// Buy side
	if !s.cfg.UseDynamicSizing {
		return s.cfg.BaseAmount
	}

	s.mu.RLock()
	capital := s.availableCapital
	s.mu.RUnlock()

	if capital <= 0 {
		log.Printf("[market-maker] no available capital, fallback base amount %.6f", s.cfg.BaseAmount)
		return s.cfg.BaseAmount
	}

	usable := capital * (1 - s.cfg.SafetyMargin)
	orderCapital := usable * s.cfg.CapitalUsage
	if mid <= 0 {
		return s.cfg.BaseAmount
	}
	dynamic := orderCapital / mid
	if dynamic < s.market.SizeTick {
		dynamic = s.market.SizeTick
	}
	log.Printf("[market-maker] dynamic sizing capital %.2f -> %.6f units", orderCapital, dynamic)
	return dynamic
}

func (s *Strategy) placeOrder(side string, price, baseAmount float64, reduceOnly bool) error {
	priceRounded := math.Floor(price/s.market.PriceTick) * s.market.PriceTick
	sizeSteps := math.Floor(baseAmount / s.market.SizeTick)
	if sizeSteps < 1 {
		return errors.New("order size below minimum tick")
	}
	baseRounded := sizeSteps * s.market.SizeTick

	priceSteps := math.Floor(priceRounded / s.market.PriceTick)
	if priceSteps < 1 {
		priceSteps = 1
	}

	req := &lighterTypes.CreateOrderTxReq{
		MarketIndex:      s.market.ID,
		ClientOrderIndex: nextOrderID(),
		BaseAmount:       int64(sizeSteps),
		Price:            uint32(priceSteps),
		IsAsk:            boolToUint8(side == "sell"),
		Type:             txtypes.LimitOrder,
		TimeInForce:      txtypes.PostOnly,
		ReduceOnly:       boolToUint8(reduceOnly),
		OrderExpiry:      time.Now().Add(30 * time.Minute).UnixMilli(),
	}

	txInfo, err := s.tx.GetCreateOrderTransaction(req, nil)
	if err != nil {
		return err
	}

	hash, err := s.http.SendRawTx(txInfo)
	if err != nil {
		return err
	}

	log.Printf("[market-maker] placed %s order id=%d base=%.6f price=%.6f tx=%s reduceOnly=%v", side, req.ClientOrderIndex, baseRounded, priceRounded, hash, reduceOnly)

	s.mu.Lock()
	s.openOrder = &OrderState{
		ID:         req.ClientOrderIndex,
		Side:       side,
		Price:      priceRounded,
		BaseAmount: baseRounded,
		PlacedAt:   time.Now(),
		ReduceOnly: reduceOnly,
	}
	s.mu.Unlock()

	return nil
}

func (s *Strategy) cancelAll() error {
	if err := cancelAllOrders(s.http, s.tx); err != nil {
		return err
	}
	s.mu.Lock()
	s.openOrder = nil
	s.mu.Unlock()
	return nil
}

func (s *Strategy) refreshAccount() error {
	resp, err := s.http.GetAccount(s.accountID)
	if err != nil {
		return err
	}
	if resp == nil || len(resp.Accounts) == 0 {
		return errors.New("account response empty")
	}

	var account *client.Account
	for i := range resp.Accounts {
		if resp.Accounts[i].Index == s.accountID {
			account = &resp.Accounts[i]
			break
		}
	}
	if account == nil {
		account = &resp.Accounts[0]
	}

	available, _ := strconv.ParseFloat(account.AvailableBalance, 64)
	portfolio, _ := strconv.ParseFloat(account.TotalAssetValue, 64)

	position := 0.0
	for _, pos := range account.Positions {
		if pos.MarketId == s.market.ID {
			position, _ = strconv.ParseFloat(pos.Position, 64)
			break
		}
	}

	s.mu.Lock()
	s.availableCapital = available
	s.portfolioValue = portfolio
	s.positionSize = position
	s.lastAccountFetch = time.Now()
	s.mu.Unlock()

	return nil
}

func (s *Strategy) evaluatePostCycle(mid float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	position := s.positionSize
	value := position * mid

	switch s.currentSide {
	case "buy":
		if position > 0 {
			if value >= s.cfg.MinPositionValueUSD {
				log.Printf("[market-maker] buy cycle filled, flipping to sell (pos=%.6f, value=%.2f)", position, value)
				s.currentSide = "sell"
			} else {
				log.Printf("[market-maker] buy cycle filled but value %.2f < %.2f, stay on buy", value, s.cfg.MinPositionValueUSD)
			}
		}
	case "sell":
		if math.Abs(position) < 1e-9 {
			log.Printf("[market-maker] sell cycle closed position -> buy mode")
			s.currentSide = "buy"
			s.lastOrderBase = 0
		} else if value < s.cfg.MinPositionValueUSD {
			log.Printf("[market-maker] sell cycle leftover value %.2f < %.2f -> buy mode", value, s.cfg.MinPositionValueUSD)
			s.currentSide = "buy"
		} else {
			log.Printf("[market-maker] sell cycle incomplete, remaining position %.6f", position)
		}
	}
}

func (s *Strategy) calculateTargetPrice(mid float64, side string) (float64, bool) {
	params := s.loadAvellaneda()
	if params != nil {
		if side == "buy" {
			return mid - params.LimitOrders.DeltaB, true
		}
		return mid + params.LimitOrders.DeltaA, true
	}

	if s.cfg.RequireParams {
		log.Printf("[market-maker] avellaneda params required but unavailable")
		return 0, false
	}

	if side == "buy" {
		return mid * (1 - s.cfg.Spread), true
	}
	return mid * (1 + s.cfg.Spread), true
}

func (s *Strategy) loadAvellaneda() *avellanedaParams {
	s.mu.RLock()
	params := s.avaParams
	lastLoad := s.lastParamsLoad
	s.mu.RUnlock()

	if params != nil && time.Since(lastLoad) < s.cfg.AvellanedaRefresh {
		return params
	}

	for _, path := range s.cfg.AvellanedaCandidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var parsed avellanedaParams
		if err := json.Unmarshal(data, &parsed); err != nil {
			log.Printf("[market-maker] invalid avellaneda file %s: %v", path, err)
			continue
		}
		if parsed.LimitOrders.DeltaA <= 0 || parsed.LimitOrders.DeltaB <= 0 {
			log.Printf("[market-maker] avellaneda file %s missing deltas", path)
			continue
		}
		log.Printf("[market-maker] loaded avellaneda params from %s (deltaA=%.6f deltaB=%.6f)", path, parsed.LimitOrders.DeltaA, parsed.LimitOrders.DeltaB)
		s.mu.Lock()
		s.avaParams = &parsed
		s.lastParamsLoad = time.Now()
		s.mu.Unlock()
		return &parsed
	}

	return nil
}

func nextOrderID() int64 {
	return time.Now().UnixNano() % 1_000_000_000_000
}

func boolToUint8(v bool) uint8 {
	if v {
		return 1
	}
	return 0
}
