package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Telegram sends alert notifications via Bot API.
type Telegram struct {
	token  string
	chatID string
	loc    *time.Location
	client *http.Client
}

// NewTelegram creates a sender; empty token/chatID means notifications are skipped.
func NewTelegram(token, chatID string, loc *time.Location) *Telegram {
	return &Telegram{
		token:  token,
		chatID: chatID,
		loc:    loc,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// SendAlert posts a formatted message to Telegram.
func (t *Telegram) SendAlert(alert Alert, detail string, price float64) error {
	if t.token == "" || t.chatID == "" {
		return nil // configured off
	}

	now := formatWallClock(t.loc, time.Now())
	typeLabel := "Horizontal"
	if alert.Type == AlertChannel {
		typeLabel = "Channel"
	}

	label := alert.Label
	if label == "" {
		label = alert.Pair
	}

	msg := fmt.Sprintf(
		"🚨 Alert\n"+
			"Pair: %s\n"+
			"Type: %s\n"+
			"%s\n"+
			"Price: %.4f\n"+
			"Label: %s\n"+
			"Time: %s",
		alert.Pair,
		typeLabel,
		detail,
		price,
		label,
		now,
	)

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.token)
	form := url.Values{}
	form.Set("chat_id", t.chatID)
	form.Set("text", msg)

	resp, err := t.client.PostForm(apiURL, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API status %d", resp.StatusCode)
	}
	return nil
}

// FormatTouchDetail builds the "Touched: ..." line for channel alerts.
func FormatTouchDetail(touched string) string {
	return "Touched: " + touched
}

// FormatHorizontalDetail builds detail for horizontal alerts.
func FormatHorizontalDetail(cond HorizontalCondition) string {
	if cond == ConditionAbove {
		return "Condition: price went above target"
	}
	return "Condition: price went below target"
}

// Enabled reports whether Telegram is configured.
func (t *Telegram) Enabled() bool {
	return strings.TrimSpace(t.token) != "" && strings.TrimSpace(t.chatID) != ""
}
