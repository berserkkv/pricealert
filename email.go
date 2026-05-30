package main

import (
	"crypto/tls"
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

	label := alert.Pair + " " + alert.Label
	

	subject := fmt.Sprintf("%s", label)
	body := fmt.Sprintf(
			"Type: %s\n"+
			"Price: %.4f\n"+
			"Time: %s",
		typeLabel,
		price,
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
	msgBytes := []byte(message)

	if e.useTLS {
		tlsConfig := &tls.Config{ServerName: e.smtpHost}
		var client *smtp.Client
		var err error

		if e.smtpPort == 465 {
			conn, err := tls.Dial("tcp", addr, tlsConfig)
			if err != nil {
				return fmt.Errorf("email TLS dial failed: %w", err)
			}
			client, err = smtp.NewClient(conn, e.smtpHost)
			if err != nil {
				return fmt.Errorf("email SMTP client failed: %w", err)
			}
		} else {
			client, err = smtp.Dial(addr)
			if err != nil {
				return fmt.Errorf("email SMTP dial failed: %w", err)
			}
			if ok, _ := client.Extension("STARTTLS"); ok {
				if err := client.StartTLS(tlsConfig); err != nil {
					_ = client.Close()
					return fmt.Errorf("email STARTTLS failed: %w", err)
				}
			} else {
				_ = client.Close()
				return fmt.Errorf("email server does not support STARTTLS")
			}
		}

		if e.useAuth {
			if err := client.Auth(e.smtpAuth); err != nil {
				_ = client.Close()
				return fmt.Errorf("email auth failed: %w", err)
			}
		}

		if err := client.Mail(e.fromEmail); err != nil {
			_ = client.Close()
			return fmt.Errorf("email sender set failed: %w", err)
		}
		if err := client.Rcpt(e.toEmail); err != nil {
			_ = client.Close()
			return fmt.Errorf("email recipient set failed: %w", err)
		}

		w, err := client.Data()
		if err != nil {
			_ = client.Close()
			return fmt.Errorf("email data open failed: %w", err)
		}
		_, err = w.Write(msgBytes)
		if err != nil {
			_ = w.Close()
			_ = client.Close()
			return fmt.Errorf("email data write failed: %w", err)
		}
		if err := w.Close(); err != nil {
			_ = client.Close()
			return fmt.Errorf("email data close failed: %w", err)
		}
		if err := client.Quit(); err != nil {
			return fmt.Errorf("email quit failed: %w", err)
		}
		return nil
	}

	if e.useAuth {
		if err := smtp.SendMail(addr, e.smtpAuth, e.fromEmail, []string{e.toEmail}, msgBytes); err != nil {
			return fmt.Errorf("email send failed: %w", err)
		}
		return nil
	}

	if err := smtp.SendMail(addr, nil, e.fromEmail, []string{e.toEmail}, msgBytes); err != nil {
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
