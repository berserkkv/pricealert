package main

import (
	"encoding/json"
	"os"
)

// Config holds application settings.
type Config struct {
	HTTPPort              int     `json:"http_port"`
	PollIntervalSec       int     `json:"poll_interval_sec"`
	BinanceSymbol         string  `json:"binance_symbol"`
	TelegramToken         string  `json:"telegram_token"`
	TelegramChatID        string  `json:"telegram_chat_id"`
	StorageFile           string  `json:"storage_file"`
	TouchTolerancePercent float64 `json:"touch_tolerance_percent"`
	UTC                   string  `json:"utc"` // IANA timezone for channel datetimes, e.g. Europe/Istanbul
}

// defaultConfig returns built-in defaults.
func defaultConfig() Config {
	return Config{
		HTTPPort:              8080,
		PollIntervalSec:       5,
		BinanceSymbol:         "SOLUSDT",
		TelegramToken:         "",
		TelegramChatID:        "",
		StorageFile:           "alerts.json",
		TouchTolerancePercent: 0.15,
		UTC:                   "Europe/Istanbul",
	}
}

// LoadConfig loads defaults, then merges config.json if present.
func LoadConfig(path string) (Config, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	// Merge: only override fields present in JSON
	var fileCfg Config
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return cfg, err
	}

	if fileCfg.HTTPPort != 0 {
		cfg.HTTPPort = fileCfg.HTTPPort
	}
	if fileCfg.PollIntervalSec != 0 {
		cfg.PollIntervalSec = fileCfg.PollIntervalSec
	}
	if fileCfg.BinanceSymbol != "" {
		cfg.BinanceSymbol = fileCfg.BinanceSymbol
	}
	if fileCfg.TelegramToken != "" {
		cfg.TelegramToken = fileCfg.TelegramToken
	}
	if fileCfg.TelegramChatID != "" {
		cfg.TelegramChatID = fileCfg.TelegramChatID
	}
	if fileCfg.StorageFile != "" {
		cfg.StorageFile = fileCfg.StorageFile
	}
	if fileCfg.TouchTolerancePercent != 0 {
		cfg.TouchTolerancePercent = fileCfg.TouchTolerancePercent
	}
	if fileCfg.UTC != "" {
		cfg.UTC = fileCfg.UTC
	}

	return cfg, nil
}
