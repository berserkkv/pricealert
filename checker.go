package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"
)

// Checker polls Binance USDT-M futures and evaluates alerts.
type Checker struct {
	cfg      Config
	storage  *Storage
	telegram *Telegram

	mu          sync.Mutex
	lastPrices  map[string]float64 // alert ID -> last price (horizontal crossing)
	wasTouching map[string]bool    // alert ID -> was in channel tolerance last tick
}

// NewChecker creates the price monitor.
func NewChecker(cfg Config, storage *Storage, telegram *Telegram) *Checker {
	return &Checker{
		cfg:        cfg,
		storage:    storage,
		telegram:   telegram,
		lastPrices:  make(map[string]float64),
		wasTouching: make(map[string]bool),
	}
}

// Run starts the polling loop (blocking).
func (c *Checker) Run(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Duration(c.cfg.PollIntervalSec) * time.Second)
	defer ticker.Stop()

	// Check immediately on start
	c.tick()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			c.tick()
		}
	}
}

func (c *Checker) tick() {
	alerts := c.storage.All()
	if len(alerts) == 0 {
		return
	}

	// Group by pair to minimize API calls
	byPair := make(map[string][]Alert)
	for _, a := range alerts {
		if !a.Enabled {
			continue
		}
		pair := a.Pair
		if pair == "" {
			pair = c.cfg.BinanceSymbol
		}
		byPair[pair] = append(byPair[pair], a)
	}

	for pair, group := range byPair {
		price, err := c.fetchPrice(pair)
		if err != nil {
			log.Printf("binance futures %s: %v", pair, err)
			continue
		}
		for _, alert := range group {
			c.evaluate(alert, price)
		}
	}
}

// fetchPrice gets latest mark price from Binance USDT-M futures public API.
func (c *Checker) fetchPrice(symbol string) (float64, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/ticker/price?symbol=%s", symbol)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	var price float64
	_, err = fmt.Sscanf(result.Price, "%f", &price)
	return price, err
}

func (c *Checker) evaluate(alert Alert, price float64) {
	switch alert.Type {
	case AlertHorizontal:
		c.checkHorizontal(alert, price)
	case AlertChannel:
		c.checkChannel(alert, price)
	}
}

func (c *Checker) checkHorizontal(alert Alert, price float64) {
	c.mu.Lock()
	prev, hadPrev := c.lastPrices[alert.ID]
	c.lastPrices[alert.ID] = price
	c.mu.Unlock()

	var triggered bool
	switch alert.Condition {
	case ConditionAbove:
		// Trigger when price crosses above target
		if price > alert.TargetPrice {
			if !hadPrev || prev <= alert.TargetPrice {
				triggered = true
			}
		}
	case ConditionBelow:
		if price < alert.TargetPrice {
			if !hadPrev || prev >= alert.TargetPrice {
				triggered = true
			}
		}
	}

	if !triggered {
		return
	}

	detail := FormatHorizontalDetail(alert.Condition)
	if err := c.telegram.SendAlert(alert, detail, price); err != nil {
		log.Printf("telegram: %v", err)
	}
	_ = c.storage.SetLastTrigger(alert.ID, time.Now())
}

func (c *Checker) checkChannel(alert Alert, price float64) {
	upper, lower := channelBounds(alert, time.Now().Unix())
	tol := c.cfg.TouchTolerancePercent / 100.0

	touchUpper := withinTolerance(price, upper, tol)
	touchLower := withinTolerance(price, lower, tol)

	// Fire only when entering the tolerance zone (avoid spam every poll)
	c.mu.Lock()
	was := c.wasTouching[alert.ID]
	nowTouching := touchUpper || touchLower
	c.wasTouching[alert.ID] = nowTouching
	c.mu.Unlock()

	if was || !nowTouching {
		return
	}

	var triggered bool
	var touched string

	switch alert.TriggerSide {
	case SideUpper:
		if touchUpper {
			triggered = true
			touched = "Upper boundary"
		}
	case SideLower:
		if touchLower {
			triggered = true
			touched = "Lower boundary"
		}
	case SideBoth:
		if touchUpper {
			triggered = true
			touched = "Upper boundary"
		} else if touchLower {
			triggered = true
			touched = "Lower boundary"
		}
	}

	if !triggered {
		return
	}

	detail := FormatTouchDetail(touched)
	if err := c.telegram.SendAlert(alert, detail, price); err != nil {
		log.Printf("telegram: %v", err)
	}
	_ = c.storage.SetLastTrigger(alert.ID, time.Now())
}

// channelBounds returns upper and lower line prices at unix time t.
func channelBounds(alert Alert, t int64) (upper, lower float64) {
	m, b := lineSlopeIntercept(alert.P1Time, alert.P1Price, alert.P2Time, alert.P2Price)
	line1 := m*float64(t) + b
	line2 := line1 + alert.Offset

	if line1 >= line2 {
		return line1, line2
	}
	return line2, line1
}

// lineSlopeIntercept computes y = m*x + b for unix x and price y.
func lineSlopeIntercept(x1 int64, y1 float64, x2 int64, y2 float64) (m, b float64) {
	dx := float64(x2 - x1)
	if dx == 0 {
		return 0, y1
	}
	m = (y2 - y1) / dx
	b = y1 - m*float64(x1)
	return m, b
}

// withinTolerance returns true if price is within ±tol of target.
func withinTolerance(price, target, tol float64) bool {
	if target == 0 {
		return false
	}
	diff := math.Abs(price-target) / target
	return diff <= tol
}
