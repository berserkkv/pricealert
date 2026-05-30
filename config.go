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
	StorageFile           string  `json:"storage_file"`
	TouchTolerancePercent float64 `json:"touch_tolerance_percent"`
	UTC                   string  `json:"utc"` // IANA timezone for channel datetimes, e.g. Europe/Istanbul
	AccessToken           string  `json:"access_token,omitempty"`

	// Telegram notification settings
	TelegramToken   string `json:"telegram_token"`
	TelegramChatID  string `json:"telegram_chat_id"`
	TelegramEnabled bool   `json:"telegram_enabled"`

	// Email notification settings
	EmailFrom       string `json:"email_from"`
	EmailTo         string `json:"email_to"`
	SMTPHost        string `json:"smtp_host"`
	SMTPPort        int    `json:"smtp_port"`
	SMTPUser        string `json:"smtp_user"`
	SMTPPassword    string `json:"smtp_password"`
	SMTPUseTLS      bool   `json:"smtp_use_tls"`
	EmailEnabled    bool   `json:"email_enabled"`

	// Ntfy notification settings (local server, ntfy-app compatible)
	NtfyTopic   string `json:"ntfy_topic"`
	NtfyPort    int    `json:"ntfy_port"`
	NtfyEnabled bool   `json:"ntfy_enabled"`
}

// defaultConfig returns built-in defaults.
func defaultConfig() Config {
	return Config{
		HTTPPort:              8080,
		PollIntervalSec:       5,
		BinanceSymbol:         "SOLUSDT",
		StorageFile:           "alerts.json",
		TouchTolerancePercent: 0.15,
		UTC:                   "Europe/Istanbul",
		TelegramToken:         "",
		TelegramChatID:        "",
		TelegramEnabled:       false,
		EmailFrom:             "",
		EmailTo:               "",
		SMTPHost:              "",
		SMTPPort:              587,
		SMTPUser:              "",
		SMTPPassword:          "",
		SMTPUseTLS:            true,
		EmailEnabled:          false,
		NtfyTopic:             "price_alerts1412",
		NtfyPort:              8090,
		NtfyEnabled:           false,
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
	cfg.TelegramEnabled = fileCfg.TelegramEnabled
	if fileCfg.EmailFrom != "" {
		cfg.EmailFrom = fileCfg.EmailFrom
	}
	if fileCfg.EmailTo != "" {
		cfg.EmailTo = fileCfg.EmailTo
	}
	if fileCfg.SMTPHost != "" {
		cfg.SMTPHost = fileCfg.SMTPHost
	}
	if fileCfg.SMTPPort != 0 {
		cfg.SMTPPort = fileCfg.SMTPPort
	}
	if fileCfg.SMTPUser != "" {
		cfg.SMTPUser = fileCfg.SMTPUser
	}
	if fileCfg.SMTPPassword != "" {
		cfg.SMTPPassword = fileCfg.SMTPPassword
	}
	if fileCfg.SMTPUseTLS {
		cfg.SMTPUseTLS = fileCfg.SMTPUseTLS
	}
	cfg.EmailEnabled = fileCfg.EmailEnabled
	if fileCfg.NtfyTopic != "" {
		cfg.NtfyTopic = fileCfg.NtfyTopic
	}
	if fileCfg.NtfyPort != 0 {
		cfg.NtfyPort = fileCfg.NtfyPort
	}
	cfg.NtfyEnabled = fileCfg.NtfyEnabled
	if fileCfg.StorageFile != "" {
		cfg.StorageFile = fileCfg.StorageFile
	}
	if fileCfg.TouchTolerancePercent != 0 {
		cfg.TouchTolerancePercent = fileCfg.TouchTolerancePercent
	}
	if fileCfg.UTC != "" {
		cfg.UTC = fileCfg.UTC
	}

	if fileCfg.AccessToken != "" {
		cfg.AccessToken = fileCfg.AccessToken
	}

	return cfg, nil
}

// SaveConfig writes the provided config to the given path as JSON.
func SaveConfig(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
