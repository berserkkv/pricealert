package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime)

	cfg, err := LoadConfig("config.json")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	loc, err := LoadTimezone(cfg)
	if err != nil {
		log.Fatalf("timezone: %v", err)
	}
	log.Printf("timezone: %s (channel datetimes and alerts use this)", cfg.UTC)

	storage, err := NewStorage(cfg.StorageFile)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	telegram := NewTelegram(cfg.TelegramToken, cfg.TelegramChatID, loc)
	if !telegram.Enabled() {
		log.Println("telegram: not configured (set telegram_token and telegram_chat_id in config.json)")
	}

	email := NewEmail(cfg.EmailFrom, cfg.EmailTo, cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPUseTLS, loc)
	if !email.Enabled() {
		log.Println("email: not configured (set email_from, email_to, and smtp_host in config.json)")
	}

	ntfy := NewNtfy(cfg.NtfyTopic, loc)
	if !ntfy.Enabled() {
		log.Println("ntfy: not configured (set ntfy_topic in config.json)")
	}

	checker := NewChecker(cfg, storage, telegram, email, ntfy)
	stop := make(chan struct{})

	go checker.Run(stop)

	srv := StartHTTPServer(&cfg, "config.json", storage, loc)

	// Graceful shutdown on Ctrl+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("shutting down...")
	close(stop)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
