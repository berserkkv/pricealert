package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Ntfy sends alert notifications to the embedded local ntfy server.
type Ntfy struct {
	topic   string
	baseURL string
	loc     *time.Location
	client  *http.Client
}

// NewNtfy creates a sender; empty topic means notifications are skipped.
func NewNtfy(topic, baseURL string, loc *time.Location) *Ntfy {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8090"
	}
	return &Ntfy{
		topic:   strings.TrimSpace(topic),
		baseURL: baseURL,
		loc:     loc,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// SendAlert posts a formatted message to ntfy.
func (n *Ntfy) SendAlert(alert Alert, detail string, price float64) error {
	if n.topic == "" {
		return nil // configured off
	}

	// now := formatWallClock(n.loc, time.Now())
	typeLabel := "Horizontal"
	if alert.Type == AlertChannel {
		typeLabel = "Channel"
	}

	label := alert.Pair + " " + alert.Label

	title := fmt.Sprintf("🚨 %s", label)
	message := fmt.Sprintf(
		"Type: %s\n%s\nPrice: %.4f",
		typeLabel,
		detail,
		price,
	)

	apiURL := fmt.Sprintf("%s/%s", n.baseURL, n.topic)
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(message))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	req.Header.Set("Title", title)
	req.Header.Set("Priority", "high")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy API status %d", resp.StatusCode)
	}
	return nil
}

// Enabled reports whether Ntfy is configured.
func (n *Ntfy) Enabled() bool {
	return n.topic != ""
}
