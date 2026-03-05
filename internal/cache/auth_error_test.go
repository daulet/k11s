package cache

import (
	"errors"
	"strings"
	"testing"
)

func TestFriendlyKubeAccessErrorDetectsTeleportExecExpiry(t *testing.T) {
	err := errors.New(`Get "https://example.teleport.sh/api/v1/pods?watch=true": getting credentials: exec: executable /usr/local/bin/tsh failed with exit code 1`)
	msg := FriendlyKubeAccessError(err, "mc1-lab1")
	if !strings.Contains(strings.ToLower(msg), "authentication expired") {
		t.Fatalf("expected auth-expiry guidance, got %q", msg)
	}
	if !strings.Contains(msg, "mc1-lab1") {
		t.Fatalf("expected context in prompt, got %q", msg)
	}
	if !strings.Contains(msg, "tsh login") {
		t.Fatalf("expected tsh relogin hint, got %q", msg)
	}
}

func TestFriendlyKubeAccessErrorPreservesNonAuthErrors(t *testing.T) {
	err := errors.New("pods is forbidden: User cannot list resource pods in API group")
	msg := FriendlyKubeAccessError(err, "dev")
	if msg != err.Error() {
		t.Fatalf("expected non-auth errors unchanged, got %q", msg)
	}
}

func TestFriendlyKubeAccessErrorSuggestsTailscaleLogin(t *testing.T) {
	err := errors.New(`Unauthorized: getting credentials via tailscale auth helper failed`)
	msg := FriendlyKubeAccessError(err, "lab")
	if !strings.Contains(strings.ToLower(msg), "authentication expired") {
		t.Fatalf("expected auth-expiry guidance, got %q", msg)
	}
	if !strings.Contains(msg, "tailscale login") {
		t.Fatalf("expected tailscale relogin hint, got %q", msg)
	}
}
