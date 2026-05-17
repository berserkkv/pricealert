package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"
)

//go:embed templates/* static/*
var embeddedFS embed.FS

// Web serves the UI and REST API.
type Web struct {
	cfg     Config
	storage *Storage
	loc     *time.Location
}

// NewWeb creates the HTTP server handler setup.
func NewWeb(cfg Config, storage *Storage, loc *time.Location) *Web {
	return &Web{cfg: cfg, storage: storage, loc: loc}
}

// Handler returns the root mux.
func (w *Web) Handler() http.Handler {
	mux := http.NewServeMux()

	// API
	mux.HandleFunc("GET /api/alerts", w.handleListAlerts)
	mux.HandleFunc("POST /api/alerts", w.handleCreateAlert)
	mux.HandleFunc("PUT /api/alerts/{id}", w.handleUpdateAlert)
	mux.HandleFunc("DELETE /api/alerts/{id}", w.handleDeleteAlert)
	mux.HandleFunc("POST /api/alerts/{id}/toggle", w.handleToggleAlert)
	mux.HandleFunc("GET /api/config", w.handleConfig)

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

	for _, a := range alerts {
		v := AlertView{Alert: a}
		if a.Type == AlertChannel {
			v.P1DateTime = formatDateTimeInZone(w.loc, a.P1Time)
			v.P2DateTime = formatDateTimeInZone(w.loc, a.P2Time)
			upper, lower := ChannelBounds(a, nowUnix)
			v.CurrentUpper = &upper
			v.CurrentLower = &lower
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
func StartHTTPServer(cfg Config, storage *Storage, loc *time.Location) *http.Server {
	web := NewWeb(cfg, storage, loc)
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
