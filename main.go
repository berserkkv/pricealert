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

	storage, err := NewStorage(cfg.StorageFile)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	telegram := NewTelegram(cfg.TelegramToken, cfg.TelegramChatID)
	if !telegram.Enabled() {
		log.Println("telegram: not configured (set telegram_token and telegram_chat_id in config.json)")
	}

	checker := NewChecker(cfg, storage, telegram)
	stop := make(chan struct{})

	go checker.Run(stop)

	srv := StartHTTPServer(cfg, storage)

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
