package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/daulet/k11s/internal/protocol"
)

func TestSaveAndLoadSession(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")
	store := NewStore(path)

	input := protocol.SessionState{
		KubeContext: "dev-cluster",
		Namespace:   "payments",
		Resource:    "pods",
		Filter:      "app=api",
		Selection:   "api-0",
	}

	if err := store.Save(input); err != nil {
		t.Fatalf("save session: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("load session: %v", err)
	}

	if got.KubeContext != input.KubeContext {
		t.Fatalf("expected kube context %q, got %q", input.KubeContext, got.KubeContext)
	}
	if got.Namespace != input.Namespace {
		t.Fatalf("expected namespace %q, got %q", input.Namespace, got.Namespace)
	}
	if got.Selection != input.Selection {
		t.Fatalf("expected selection %q, got %q", input.Selection, got.Selection)
	}
	if got.UpdatedAtMs == 0 {
		t.Fatalf("expected updated timestamp to be set")
	}
}

func TestLoadCorruptSessionFallsBackToDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")
	store := NewStore(path)

	if err := os.WriteFile(path, []byte("{invalid"), 0o600); err != nil {
		t.Fatalf("write corrupt session file: %v", err)
	}

	got, err := store.Load()
	if !errors.Is(err, ErrCorruptSession) {
		t.Fatalf("expected ErrCorruptSession, got: %v", err)
	}

	defaults := protocol.DefaultSessionState()
	if got.Namespace != defaults.Namespace {
		t.Fatalf("expected default namespace %q, got %q", defaults.Namespace, got.Namespace)
	}
	if got.Resource != defaults.Resource {
		t.Fatalf("expected default resource %q, got %q", defaults.Resource, got.Resource)
	}
}
