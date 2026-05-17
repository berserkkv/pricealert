package main

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Storage persists alerts to a JSON file.
type Storage struct {
	mu   sync.RWMutex
	path string
	list []Alert
}

// NewStorage loads alerts from disk or starts empty.
func NewStorage(path string) (*Storage, error) {
	s := &Storage{path: path, list: []Alert{}}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}

	if len(data) > 0 {
		if err := json.Unmarshal(data, &s.list); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// All returns a copy of all alerts.
func (s *Storage) All() []Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Alert, len(s.list))
	copy(out, s.list)
	return out
}

// Get finds one alert by ID.
func (s *Storage) Get(id string) (Alert, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, a := range s.list {
		if a.ID == id {
			return a, true
		}
	}
	return Alert{}, false
}

// Save replaces entire list and writes to disk.
func (s *Storage) saveUnlocked() error {
	data, err := json.MarshalIndent(s.list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

// Add appends an alert and saves.
func (s *Storage) Add(a Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = append(s.list, a)
	return s.saveUnlocked()
}

// Update replaces an alert by ID.
func (s *Storage) Update(a Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.list {
		if existing.ID == a.ID {
			// Preserve created time and last trigger if not set on update
			if a.CreatedAt.IsZero() {
				a.CreatedAt = existing.CreatedAt
			}
			if a.LastTrigger.IsZero() {
				a.LastTrigger = existing.LastTrigger
			}
			s.list[i] = a
			return s.saveUnlocked()
		}
	}
	return os.ErrNotExist
}

// Delete removes an alert by ID.
func (s *Storage) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, a := range s.list {
		if a.ID == id {
			s.list = append(s.list[:i], s.list[i+1:]...)
			return s.saveUnlocked()
		}
	}
	return os.ErrNotExist
}

// SetLastTrigger updates last trigger time for an alert.
func (s *Storage) SetLastTrigger(id string, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, a := range s.list {
		if a.ID == id {
			a.LastTrigger = t
			s.list[i] = a
			return s.saveUnlocked()
		}
	}
	return os.ErrNotExist
}

// ToggleEnabled flips the enabled flag.
func (s *Storage) ToggleEnabled(id string) (Alert, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, a := range s.list {
		if a.ID == id {
			a.Enabled = !a.Enabled
			s.list[i] = a
			if err := s.saveUnlocked(); err != nil {
				return Alert{}, err
			}
			return a, nil
		}
	}
	return Alert{}, os.ErrNotExist
}
