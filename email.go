package main

import (
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

// Email sends alert notifications via SMTP.
type Email struct {
	fromEmail  string
	toEmail    string
	smtpHost   string
	smtpPort   int
	smtpUser   string
	smtpPass   string
	loc        *time.Location
	smtpAuth   smtp.Auth
	useAuth    bool
	useTLS     bool
}

// NewEmail creates an SMTP email sender; empty fields means notifications are skipped.
func NewEmail(fromEmail, toEmail, smtpHost string, smtpPort int, smtpUser, smtpPass string, useTLS bool, loc *time.Location) *Email {
	e := &Email{
		fromEmail: fromEmail,
		toEmail:   toEmail,
		smtpHost:  smtpHost,
		smtpPort:  smtpPort,
		smtpUser:  smtpUser,
		smtpPass:  smtpPass,
		loc:       loc,
		useTLS:    useTLS,
	}

	// Only set up auth if credentials are provided
	if smtpUser != "" && smtpPass != "" {
		e.smtpAuth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
		e.useAuth = true
	}

	return e
}

// SendAlert sends a formatted email message.
func (e *Email) SendAlert(alert Alert, detail string, price float64) error {
	if e.fromEmail == "" || e.toEmail == "" || e.smtpHost == "" {
		return nil // configured off
	}

	now := formatWallClock(e.loc, time.Now())
	typeLabel := "Horizontal"
	if alert.Type == AlertChannel {
		typeLabel = "Channel"
	}

	label := alert.Label
	if label == "" {
		label = alert.Pair
	}

	subject := fmt.Sprintf("Price Alert: %s", label)
	body := fmt.Sprintf(
		"Price Alert\n"+
			"============\n\n"+
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

	message := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"\r\n"+
		"%s",
		e.fromEmail,
		e.toEmail,
		subject,
		body,
	)

	addr := fmt.Sprintf("%s:%d", e.smtpHost, e.smtpPort)
	var err error
	
	if e.useTLS {
		// For TLS connections, we would need to use smtp.Dial with TLS
		// For simplicity, using plain SMTP; can be extended for TLS support
	}

	if e.useAuth {
		err = smtp.SendMail(addr, e.smtpAuth, e.fromEmail, []string{e.toEmail}, []byte(message))
	} else {
		err = smtp.SendMail(addr, nil, e.fromEmail, []string{e.toEmail}, []byte(message))
	}

	if err != nil {
		return fmt.Errorf("email send failed: %w", err)
	}
	return nil
}

// Enabled reports whether Email is configured.
func (e *Email) Enabled() bool {
	return strings.TrimSpace(e.fromEmail) != "" &&
		strings.TrimSpace(e.toEmail) != "" &&
		strings.TrimSpace(e.smtpHost) != ""
}
