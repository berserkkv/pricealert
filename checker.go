package main

import (
	"log"
	"math"
	"sync"
	"time"
)

// Checker polls Binance USDT-M futures and evaluates alerts.
type Checker struct {
	cfg      Config
	storage  *Storage
	telegram *Telegram
	email    *Email
	ntfy     *Ntfy

	mu          sync.Mutex
	lastPrices  map[string]float64 // alert ID -> last price (horizontal crossing)
	wasTouching map[string]bool    // alert ID -> was in channel tolerance last tick
}

// NewChecker creates the price monitor.
func NewChecker(cfg Config, storage *Storage, telegram *Telegram, email *Email, ntfy *Ntfy) *Checker {
	return &Checker{
		cfg:         cfg,
		storage:     storage,
		telegram:    telegram,
		email:       email,
		ntfy:        ntfy,
		lastPrices:  make(map[string]float64),
		wasTouching: make(map[string]bool),
	}
}

// sendAlert sends the alert via all enabled notifiers.
func (c *Checker) sendAlert(alert Alert, detail string, price float64) {
	// Send via Telegram
	if c.cfg.TelegramEnabled && c.telegram.Enabled() {
		if err := c.telegram.SendAlert(alert, detail, price); err != nil {
			log.Printf("telegram: %v", err)
		}
	}
	// Send via Email
	if c.cfg.EmailEnabled && c.email.Enabled() {
		if err := c.email.SendAlert(alert, detail, price); err != nil {
			log.Printf("email: %v", err)
		}
	}
	// Send via Ntfy
	if c.cfg.NtfyEnabled && c.ntfy.Enabled() {
		if err := c.ntfy.SendAlert(alert, detail, price); err != nil {
			log.Printf("ntfy: %v", err)
		}
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
		price, err := FetchFuturesPrice(pair)
		if err != nil {
			log.Printf("binance futures %s: %v", pair, err)
			continue
		}
		for _, alert := range group {
			c.evaluate(alert, price)
		}
	}
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

	// Respect 1 hour cooldown after a trigger
	if !alert.LastTrigger.IsZero() {
		if time.Since(alert.LastTrigger) < time.Hour {
			return
		}
	}

	detail := FormatHorizontalDetail(alert.Condition)
	c.sendAlert(alert, detail, price)
	_ = c.storage.SetLastTrigger(alert.ID, time.Now())
}

func (c *Checker) checkChannel(alert Alert, price float64) {
	upper, lower := ChannelBoundsNow(alert)
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

	// Respect 1 hour cooldown after a trigger
	if !alert.LastTrigger.IsZero() {
		if time.Since(alert.LastTrigger) < time.Hour {
			return
		}
	}

	detail := FormatTouchDetail(touched)
	c.sendAlert(alert, detail, price)
	_ = c.storage.SetLastTrigger(alert.ID, time.Now())
}

// withinTolerance returns true if price is within ±tol of target.
func withinTolerance(price, target, tol float64) bool {
	if target == 0 {
		return false
	}
	diff := math.Abs(price-target) / target
	return diff <= tol
}
