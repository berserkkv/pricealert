package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed templates/* static/*
var embeddedFS embed.FS

// Web serves the UI and REST API.
type Web struct {
	cfg     *Config
	cfgPath string
	storage *Storage
	loc     *time.Location
}

// NewWeb creates the HTTP server handler setup.
func NewWeb(cfg *Config, cfgPath string, storage *Storage, loc *time.Location) *Web {
	return &Web{cfg: cfg, cfgPath: cfgPath, storage: storage, loc: loc}
}

// Handler returns the root mux.
func (w *Web) Handler() http.Handler {
	mux := http.NewServeMux()

	// Helper to require auth for protected endpoints
	authWrap := func(h http.HandlerFunc) http.HandlerFunc {
		return func(rw http.ResponseWriter, r *http.Request) {
			if !w.isAuthorized(r) {
				writeError(rw, "unauthorized", http.StatusUnauthorized)
				return
			}
			h(rw, r)
		}
	}

	// API (protected)
	mux.HandleFunc("GET /api/alerts", authWrap(w.handleListAlerts))
	mux.HandleFunc("POST /api/alerts", authWrap(w.handleCreateAlert))
	mux.HandleFunc("PUT /api/alerts/{id}", authWrap(w.handleUpdateAlert))
	mux.HandleFunc("DELETE /api/alerts/{id}", authWrap(w.handleDeleteAlert))
	mux.HandleFunc("POST /api/alerts/{id}/toggle", authWrap(w.handleToggleAlert))

	// Config and auth endpoints (login & settings)
	mux.HandleFunc("GET /api/config", w.handleConfig)
	mux.HandleFunc("POST /api/login", w.handleLogin)
	mux.HandleFunc("GET /api/settings", authWrap(w.handleGetSettings))
	mux.HandleFunc("PUT /api/settings", authWrap(w.handleUpdateSettings))
	mux.HandleFunc("POST /api/update", authWrap(w.handleUpdateApp))

	// Static files from embed
	staticSub, _ := fs.Sub(embeddedFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// Index page
	mux.HandleFunc("GET /", w.handleIndex)

	return mux
}

func (w *Web) handleIndex(rw http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(rw, r)
		return
	}
	data, err := embeddedFS.ReadFile("templates/index.html")
	if err != nil {
		http.Error(rw, "template missing", http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = rw.Write(data)
}

func (w *Web) handleListAlerts(rw http.ResponseWriter, r *http.Request) {
	alerts := w.storage.All()
	out := make([]AlertView, 0, len(alerts))
	nowUnix := time.Now().Unix()
	prices := make(map[string]float64)

	for _, a := range alerts {
		v := AlertView{Alert: a}
		if a.Type == AlertChannel {
			v.P1DateTime = formatDateTimeInZone(w.loc, a.P1Time)
			v.P2DateTime = formatDateTimeInZone(w.loc, a.P2Time)
			upper, lower := ChannelBounds(a, nowUnix)
			v.CurrentUpper = &upper
			v.CurrentLower = &lower

			if _, ok := prices[a.Pair]; !ok {
				if price, err := FetchFuturesPrice(a.Pair); err == nil {
					prices[a.Pair] = price
				} else {
					log.Printf("failed to fetch price for %s: %v", a.Pair, err)
				}
			}
			if price, ok := prices[a.Pair]; ok {
				v.CurrentPrice = &price
			}
		}
		out = append(out, v)
	}
	writeJSON(rw, out)
}

func (w *Web) handleCreateAlert(rw http.ResponseWriter, r *http.Request) {
	input, err := readAlertInput(r.Body)
	if err != nil {
		writeError(rw, err.Error(), http.StatusBadRequest)
		return
	}

	alert, err := w.inputToAlert(input, "")
	if err != nil {
		writeError(rw, err.Error(), http.StatusBadRequest)
		return
	}

	alert.ID = newID()
	alert.CreatedAt = time.Now()
	alert.Enabled = true

	if err := w.storage.Add(alert); err != nil {
		writeError(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(rw, alert)
}

func (w *Web) handleUpdateAlert(rw http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, ok := w.storage.Get(id)
	if !ok {
		writeError(rw, "not found", http.StatusNotFound)
		return
	}

	input, err := readAlertInput(r.Body)
	if err != nil {
		writeError(rw, err.Error(), http.StatusBadRequest)
		return
	}

	alert, err := w.inputToAlert(input, id)
	if err != nil {
		writeError(rw, err.Error(), http.StatusBadRequest)
		return
	}

	alert.ID = id
	alert.CreatedAt = existing.CreatedAt
	alert.Enabled = existing.Enabled
	alert.LastTrigger = existing.LastTrigger

	if err := w.storage.Update(alert); err != nil {
		writeError(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(rw, alert)
}

func (w *Web) handleDeleteAlert(rw http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := w.storage.Delete(id); err != nil {
		writeError(rw, "not found", http.StatusNotFound)
		return
	}
	rw.WriteHeader(http.StatusNoContent)
}

func (w *Web) handleConfig(rw http.ResponseWriter, r *http.Request) {
	tz := w.cfg.UTC
	if tz == "" {
		tz = "Europe/Istanbul"
	}
	writeJSON(rw, map[string]string{"utc": tz})
}

// isAuthorized checks for Bearer token or X-Auth-Token header matching stored access token.
func (w *Web) isAuthorized(r *http.Request) bool {
	if w.cfg == nil || w.cfg.AccessToken == "" {
		// no token configured -> no auth required
		return true
	}
	// Check Authorization header
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		tok := strings.TrimPrefix(auth, "Bearer ")
		return tok == w.cfg.AccessToken
	}
	// Fallback header
	if r.Header.Get("X-Auth-Token") == w.cfg.AccessToken {
		return true
	}
	return false
}

// handleLogin allows the first-time client to set an access token, or validate an existing one.
func (w *Web) handleLogin(rw http.ResponseWriter, r *http.Request) {
	var in struct {
		Secret string `json:"secret"`
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&in); err != nil {
		writeError(rw, "invalid request", http.StatusBadRequest)
		return
	}
	if in.Secret == "" {
		writeError(rw, "secret required", http.StatusBadRequest)
		return
	}

	// If no access token configured yet, set this secret as the token and persist config
	if w.cfg.AccessToken == "" {
		w.cfg.AccessToken = in.Secret
		if err := SaveConfig(w.cfgPath, *w.cfg); err != nil {
			writeError(rw, "failed to save config", http.StatusInternalServerError)
			return
		}
		writeJSON(rw, map[string]string{"token": in.Secret})
		return
	}

	// Otherwise validate
	if in.Secret != w.cfg.AccessToken {
		writeError(rw, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(rw, map[string]string{"token": in.Secret})
}

// handleGetSettings returns editable settings.
func (w *Web) handleGetSettings(rw http.ResponseWriter, r *http.Request) {
	out := map[string]any{
		"utc":                     w.cfg.UTC,
		"telegram_token":          w.cfg.TelegramToken,
		"telegram_chat_id":        w.cfg.TelegramChatID,
		"touch_tolerance_percent": w.cfg.TouchTolerancePercent,
		"poll_interval_sec":       w.cfg.PollIntervalSec,
	}
	writeJSON(rw, out)
}

// handleUpdateSettings updates selected config fields and persists them.
func (w *Web) handleUpdateSettings(rw http.ResponseWriter, r *http.Request) {
	var in struct {
		UTC                   *string  `json:"utc"`
		TelegramToken         *string  `json:"telegram_token"`
		TelegramChatID        *string  `json:"telegram_chat_id"`
		TouchTolerancePercent *float64 `json:"touch_tolerance_percent"`
		PollIntervalSec       *int     `json:"poll_interval_sec"`
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&in); err != nil {
		writeError(rw, "invalid request", http.StatusBadRequest)
		return
	}
	if in.UTC != nil {
		w.cfg.UTC = *in.UTC
	}
	if in.TelegramToken != nil {
		w.cfg.TelegramToken = *in.TelegramToken
	}
	if in.TelegramChatID != nil {
		w.cfg.TelegramChatID = *in.TelegramChatID
	}
	if in.TouchTolerancePercent != nil {
		w.cfg.TouchTolerancePercent = *in.TouchTolerancePercent
	}
	if in.PollIntervalSec != nil {
		w.cfg.PollIntervalSec = *in.PollIntervalSec
	}

	if err := SaveConfig(w.cfgPath, *w.cfg); err != nil {
		writeError(rw, "failed to save config", http.StatusInternalServerError)
		return
	}
	writeJSON(rw, map[string]string{"status": "ok"})
}

func (w *Web) handleUpdateApp(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := w.runSelfUpdate(); err != nil {
		writeError(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(rw, map[string]string{"status": "ok", "message": "updated successfully"})
}

func (w *Web) runSelfUpdate() error {
	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to determine executable path: %w", err)
	}
	binPath, err = filepath.EvalSymlinks(binPath)
	if err != nil {
		return fmt.Errorf("unable to resolve executable path: %w", err)
	}

	resp, err := http.Get("https://raw.githubusercontent.com/berserkkv/pricealert/main/pricealert")
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(binPath), "pricealert-update-*")
	if err != nil {
		return fmt.Errorf("unable to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("failed to write update binary: %w", err)
	}
	if err := tmpFile.Chmod(0o755); err != nil {
		return fmt.Errorf("failed to set executable permission: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, binPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	output, err := exec.Command("systemctl", "restart", "pricealert").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart service: %v: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

func (w *Web) handleToggleAlert(rw http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	alert, err := w.storage.ToggleEnabled(id)
	if err != nil {
		writeError(rw, "not found", http.StatusNotFound)
		return
	}
	writeJSON(rw, alert)
}

func readAlertInput(body io.Reader) (AlertInput, error) {
	var input AlertInput
	dec := json.NewDecoder(body)
	if err := dec.Decode(&input); err != nil {
		return input, err
	}
	return input, nil
}

// inputToAlert converts API input to a stored Alert.
func (w *Web) inputToAlert(in AlertInput, id string) (Alert, error) {
	if in.Pair == "" {
		in.Pair = "SOLUSDT"
	}
	if in.Type != AlertHorizontal && in.Type != AlertChannel {
		return Alert{}, fmt.Errorf("invalid type: %s", in.Type)
	}

	a := Alert{
		Pair:  strings.ToUpper(in.Pair),
		Type:  in.Type,
		Label: in.Label,
	}

	switch in.Type {
	case AlertHorizontal:
		if in.TargetPrice <= 0 {
			return Alert{}, fmt.Errorf("target price required")
		}
		if in.Condition != ConditionAbove && in.Condition != ConditionBelow {
			return Alert{}, fmt.Errorf("condition must be above or below")
		}
		a.TargetPrice = in.TargetPrice
		a.Condition = in.Condition

	case AlertChannel:
		p1, err := parseDateTimeInZone(w.loc, in.P1DateTime)
		if err != nil {
			return Alert{}, fmt.Errorf("point 1 datetime: %w", err)
		}
		p2, err := parseDateTimeInZone(w.loc, in.P2DateTime)
		if err != nil {
			return Alert{}, fmt.Errorf("point 2 datetime: %w", err)
		}
		if in.P1Price <= 0 || in.P2Price <= 0 {
			return Alert{}, fmt.Errorf("point prices required")
		}
		if p1 == p2 {
			return Alert{}, fmt.Errorf("points must have different times")
		}
		if in.TriggerSide != SideUpper && in.TriggerSide != SideLower && in.TriggerSide != SideBoth {
			return Alert{}, fmt.Errorf("trigger_side must be upper, lower, or both")
		}
		a.P1Time = p1
		a.P1Price = in.P1Price
		a.P2Time = p2
		a.P2Price = in.P2Price
		a.Offset = in.Offset
		a.TriggerSide = in.TriggerSide
	}

	_ = id
	return a, nil
}

func newID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func writeJSON(rw http.ResponseWriter, v any) {
	rw.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(rw http.ResponseWriter, msg string, code int) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(code)
	_ = json.NewEncoder(rw).Encode(map[string]string{"error": msg})
}

// StartHTTPServer runs the web server.
func StartHTTPServer(cfg *Config, cfgPath string, storage *Storage, loc *time.Location) *http.Server {
	web := NewWeb(cfg, cfgPath, storage, loc)
	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	srv := &http.Server{
		Addr:    addr,
		Handler: web.Handler(),
	}
	go func() {
		log.Printf("web UI http://localhost%s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()
	return srv
}
