package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const persistenceVersion = 1

type persistedState struct {
	Version       int               `json:"version"`
	Items         []DownloadItem    `json:"items"`
	Configuration Configuration     `json:"configuration"`
	NextID        int               `json:"nextId"`
	RetryAt       map[string]string `json:"retryAt,omitempty"`
}

type StateStore interface {
	Load() (persistedState, error)
	Save(persistedState) error
}

type JSONStateStore struct{ path string }

func NewJSONStateStore(path string) *JSONStateStore { return &JSONStateStore{path: path} }

func (s *JSONStateStore) Load() (persistedState, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return persistedState{Version: persistenceVersion, RetryAt: map[string]string{}}, nil
	}
	if err != nil {
		return persistedState{}, fmt.Errorf("read application state: %w", err)
	}
	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return persistedState{}, fmt.Errorf("decode application state: %w", err)
	}
	if state.Version != persistenceVersion {
		return persistedState{}, fmt.Errorf("unsupported application state version %d", state.Version)
	}
	if state.RetryAt == nil {
		state.RetryAt = map[string]string{}
	}
	return state, nil
}

func (s *JSONStateStore) Save(state persistedState) error {
	if s.path == "" {
		return errors.New("application state path is required")
	}
	state.Version = persistenceVersion
	if state.RetryAt == nil {
		state.RetryAt = map[string]string{}
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode application state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("create application state directory: %w", err)
	}
	temporary := s.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0600); err != nil {
		return fmt.Errorf("write application state: %w", err)
	}
	if err := os.Rename(temporary, s.path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("replace application state: %w", err)
	}
	return nil
}
