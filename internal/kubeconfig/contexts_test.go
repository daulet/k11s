package kubeconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadContextNamesSingleFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(`
current-context: dev
contexts:
  - name: dev
  - name: prod
`), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	original := os.Getenv("KUBECONFIG")
	t.Cleanup(func() {
		if original == "" {
			_ = os.Unsetenv("KUBECONFIG")
		} else {
			_ = os.Setenv("KUBECONFIG", original)
		}
	})
	_ = os.Setenv("KUBECONFIG", path)

	names, err := LoadContextNames()
	if err != nil {
		t.Fatalf("load contexts: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 contexts, got %d", len(names))
	}
	if names[0] != "dev" || names[1] != "prod" {
		t.Fatalf("unexpected contexts: %#v", names)
	}
}

func TestLoadContextNamesMultipleFilesDedupe(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pathA := filepath.Join(dir, "a")
	pathB := filepath.Join(dir, "b")

	if err := os.WriteFile(pathA, []byte(`
current-context: team-a
contexts:
  - name: team-a
  - name: shared
`), 0o600); err != nil {
		t.Fatalf("write kubeconfig A: %v", err)
	}
	if err := os.WriteFile(pathB, []byte(`
contexts:
  - name: shared
  - name: team-b
`), 0o600); err != nil {
		t.Fatalf("write kubeconfig B: %v", err)
	}

	original := os.Getenv("KUBECONFIG")
	t.Cleanup(func() {
		if original == "" {
			_ = os.Unsetenv("KUBECONFIG")
		} else {
			_ = os.Setenv("KUBECONFIG", original)
		}
	})
	_ = os.Setenv("KUBECONFIG", pathA+string(os.PathListSeparator)+pathB)

	names, err := LoadContextNames()
	if err != nil {
		t.Fatalf("load contexts: %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 contexts, got %d (%#v)", len(names), names)
	}
}
