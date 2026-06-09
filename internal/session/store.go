package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/daulet/k11s/internal/protocol"
)

var ErrCorruptSession = errors.New("corrupt session file")

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() (protocol.SessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return protocol.DefaultSessionState(), nil
		}
		return protocol.DefaultSessionState(), fmt.Errorf("read session file: %w", err)
	}

	var state protocol.SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return protocol.DefaultSessionState(), fmt.Errorf("%w: %v", ErrCorruptSession, err)
	}

	return normalizeState(state), nil
}

func (s *Store) Save(state protocol.SessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized := normalizeState(state)
	normalized.UpdatedAtMs = time.Now().UnixMilli()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}

	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write temp session file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace session file: %w", err)
	}

	return nil
}

func normalizeState(state protocol.SessionState) protocol.SessionState {
	defaults := protocol.DefaultSessionState()

	if state.Namespace == "" {
		state.Namespace = defaults.Namespace
	}
	if state.Resource == "" {
		state.Resource = defaults.Resource
	}
	state.Filter = strings.TrimSpace(state.Filter)
	state.ListFilter = strings.TrimSpace(state.ListFilter)
	state.Selection = strings.TrimSpace(state.Selection)

	return state
}
