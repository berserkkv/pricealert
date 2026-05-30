package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NtfyMessage is a minimal ntfy-compatible notification payload.
type NtfyMessage struct {
	ID       string `json:"id"`
	Time     int64  `json:"time"`
	Event    string `json:"event"`
	Topic    string `json:"topic"`
	Message  string `json:"message,omitempty"`
	Title    string `json:"title,omitempty"`
	Priority int    `json:"priority,omitempty"`
}

// NtfyServer is a local in-process pub/sub server compatible with the ntfy app.
type NtfyServer struct {
	mu   sync.RWMutex
	subs map[string]map[chan NtfyMessage]struct{}
}

// NewNtfyServer creates a broker for local notifications.
func NewNtfyServer() *NtfyServer {
	return &NtfyServer{subs: make(map[string]map[chan NtfyMessage]struct{})}
}

// StartNtfyServer listens for publish/subscribe on all interfaces.
func StartNtfyServer(port int) *http.Server {
	if port <= 0 {
		port = 8090
	}
	broker := NewNtfyServer()
	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{Addr: addr, Handler: broker}
	go func() {
		log.Printf("ntfy (local) http://0.0.0.0%s — use this host:port in the ntfy app", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ntfy server: %v", err)
		}
	}()
	return srv
}

func (s *NtfyServer) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if path == "" || path == "v1/health" {
		rw.Header().Set("Content-Type", "application/json")
		_, _ = rw.Write([]byte(`{"healthy":true}`))
		return
	}

	jsonStream := strings.HasSuffix(path, "/json")
	topic := path
	if jsonStream {
		topic = strings.TrimSuffix(path, "/json")
	}
	topic = strings.Trim(topic, "/")
	if topic == "" || strings.Contains(topic, "/") {
		http.NotFound(rw, r)
		return
	}

	switch r.Method {
	case http.MethodPost, http.MethodPut:
		s.handlePublish(rw, r, topic)
	case http.MethodGet:
		if jsonStream {
			s.handleSubscribeJSON(rw, r, topic)
		} else {
			s.handleSubscribeSSE(rw, r, topic)
		}
	default:
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *NtfyServer) handlePublish(rw http.ResponseWriter, r *http.Request, topic string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(rw, "read body", http.StatusBadRequest)
		return
	}

	msg := NtfyMessage{
		ID:       fmt.Sprintf("%d", time.Now().UnixNano()),
		Time:     time.Now().Unix(),
		Event:    "message",
		Topic:    topic,
		Message:  string(body),
		Title:    r.Header.Get("Title"),
		Priority: parsePriority(r.Header.Get("Priority")),
	}

	s.broadcast(topic, msg)

	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(msg)
}

func (s *NtfyServer) handleSubscribeJSON(rw http.ResponseWriter, r *http.Request, topic string) {
	flusher, ok := rw.(http.Flusher)
	if !ok {
		http.Error(rw, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/x-ndjson")
	rw.WriteHeader(http.StatusOK)
	flusher.Flush()

	open := NtfyMessage{
		ID:    fmt.Sprintf("%d", time.Now().UnixNano()),
		Time:  time.Now().Unix(),
		Event: "open",
		Topic: topic,
	}
	_ = writeNDJSON(rw, open)
	flusher.Flush()

	ch := s.subscribe(topic)
	defer s.unsubscribe(topic, ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			if err := writeNDJSON(rw, msg); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *NtfyServer) handleSubscribeSSE(rw http.ResponseWriter, r *http.Request, topic string) {
	flusher, ok := rw.(http.Flusher)
	if !ok {
		http.Error(rw, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")
	rw.WriteHeader(http.StatusOK)
	flusher.Flush()

	open := NtfyMessage{
		ID:    fmt.Sprintf("%d", time.Now().UnixNano()),
		Time:  time.Now().Unix(),
		Event: "open",
		Topic: topic,
	}
	writeSSE(rw, open)
	flusher.Flush()

	ch := s.subscribe(topic)
	defer s.unsubscribe(topic, ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			writeSSE(rw, msg)
			flusher.Flush()
		}
	}
}

func (s *NtfyServer) subscribe(topic string) chan NtfyMessage {
	ch := make(chan NtfyMessage, 8)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.subs[topic] == nil {
		s.subs[topic] = make(map[chan NtfyMessage]struct{})
	}
	s.subs[topic][ch] = struct{}{}
	return ch
}

func (s *NtfyServer) unsubscribe(topic string, ch chan NtfyMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m := s.subs[topic]; m != nil {
		delete(m, ch)
		if len(m) == 0 {
			delete(s.subs, topic)
		}
	}
	close(ch)
}

func (s *NtfyServer) broadcast(topic string, msg NtfyMessage) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for ch := range s.subs[topic] {
		select {
		case ch <- msg:
		default:
		}
	}
}

func writeNDJSON(w io.Writer, msg NtfyMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

func writeSSE(w io.Writer, msg NtfyMessage) {
	fmt.Fprintf(w, "event: %s\n", msg.Event)
	fmt.Fprintf(w, "id: %s\n", msg.ID)
	fmt.Fprintf(w, "topic: %s\n", msg.Topic)
	if msg.Title != "" {
		fmt.Fprintf(w, "title: %s\n", msg.Title)
	}
	if msg.Message != "" {
		fmt.Fprintf(w, "message: %s\n", msg.Message)
	}
	fmt.Fprintf(w, "time: %d\n", msg.Time)
	if msg.Priority > 0 {
		fmt.Fprintf(w, "priority: %d\n", msg.Priority)
	}
	fmt.Fprint(w, "\n")
}

func parsePriority(p string) int {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "min", "1":
		return 1
	case "low", "2":
		return 2
	case "default", "3", "":
		return 3
	case "high", "4":
		return 4
	case "max", "urgent", "5":
		return 5
	default:
		if n, err := strconv.Atoi(p); err == nil && n >= 1 && n <= 5 {
			return n
		}
		return 3
	}
}
