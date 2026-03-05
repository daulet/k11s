package namespaces

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/daulet/k11s/internal/protocol"
)

func TestNamespaceCacheColdStartTransitionsToLive(t *testing.T) {
	fetcher := &queueFetcher{
		responses: []fetchResponse{
			{namespaces: []string{"default", "payments"}},
		},
	}
	cache := New(context.Background(), fetcher, nil)

	initial := cache.Get(protocol.NamespaceListQuery{KubeContext: "dev"})
	if initial.Freshness.State != protocol.FreshnessStateCatchingUp {
		t.Fatalf("expected CATCHING_UP on cold start, got %s", initial.Freshness.State)
	}

	payload := waitForCondition(t, 250*time.Millisecond, func() (protocol.NamespaceListPayload, bool) {
		next := cache.Get(protocol.NamespaceListQuery{KubeContext: "dev"})
		return next, next.Freshness.State == protocol.FreshnessStateLive && len(next.Namespaces) == 2
	})
	if payload.Namespaces[0] != "default" {
		t.Fatalf("unexpected namespace list: %#v", payload.Namespaces)
	}
}

func TestNamespaceCachePreservesDataWhenRefreshFails(t *testing.T) {
	fetcher := &queueFetcher{
		responses: []fetchResponse{
			{namespaces: []string{"default", "kube-system"}},
			{err: errors.New("permission denied")},
		},
	}
	cache := New(context.Background(), fetcher, nil)
	query := protocol.NamespaceListQuery{KubeContext: "prod"}

	_ = cache.Get(query)
	_ = waitForCondition(t, 250*time.Millisecond, func() (protocol.NamespaceListPayload, bool) {
		next := cache.Get(query)
		return next, next.Freshness.State == protocol.FreshnessStateLive
	})

	kctx := "prod"
	cache.mu.Lock()
	ent := cache.entries[kctx]
	ent.refreshing = true
	cache.refreshInterval = time.Hour
	cache.mu.Unlock()

	cache.refresh(kctx)
	payload := cache.Get(query)
	if payload.Freshness.State != protocol.FreshnessStateStale {
		t.Fatalf("expected stale after refresh failure, got %s", payload.Freshness.State)
	}
	if len(payload.Namespaces) != 2 {
		t.Fatalf("expected cached namespaces to remain, got %d", len(payload.Namespaces))
	}
	if !strings.Contains(payload.Freshness.Error, "permission denied") {
		t.Fatalf("expected freshness error to include refresh failure, got %q", payload.Freshness.Error)
	}
}

func TestNamespaceCacheAuthExpiryErrorShowsReloginPrompt(t *testing.T) {
	fetcher := &queueFetcher{
		responses: []fetchResponse{
			{namespaces: []string{"default", "kube-system"}},
			{err: errors.New(`list namespaces: getting credentials: exec: executable /usr/local/bin/tsh failed with exit code 1`)},
		},
	}
	cache := New(context.Background(), fetcher, nil)
	query := protocol.NamespaceListQuery{KubeContext: "mc1-lab1"}

	_ = cache.Get(query)
	_ = waitForCondition(t, 250*time.Millisecond, func() (protocol.NamespaceListPayload, bool) {
		next := cache.Get(query)
		return next, next.Freshness.State == protocol.FreshnessStateLive
	})

	kctx := "mc1-lab1"
	cache.mu.Lock()
	ent := cache.entries[kctx]
	ent.refreshing = true
	cache.refreshInterval = time.Hour
	cache.mu.Unlock()

	cache.refresh(kctx)
	payload := cache.Get(query)
	if !strings.Contains(strings.ToLower(payload.Freshness.Error), "authentication expired") {
		t.Fatalf("expected auth-expiry message, got %q", payload.Freshness.Error)
	}
	if !strings.Contains(payload.Freshness.Error, "tsh login") {
		t.Fatalf("expected tsh relogin hint, got %q", payload.Freshness.Error)
	}
	if !strings.Contains(payload.Freshness.Error, "mc1-lab1") {
		t.Fatalf("expected context in relogin prompt, got %q", payload.Freshness.Error)
	}
}

func TestNamespaceCacheKeysByContext(t *testing.T) {
	fetcher := &contextFetcher{}
	cache := New(context.Background(), fetcher, nil)

	_ = cache.Get(protocol.NamespaceListQuery{KubeContext: "dev"})
	devPayload := waitForCondition(t, 250*time.Millisecond, func() (protocol.NamespaceListPayload, bool) {
		next := cache.Get(protocol.NamespaceListQuery{KubeContext: "dev"})
		return next, next.Freshness.State == protocol.FreshnessStateLive
	})

	_ = cache.Get(protocol.NamespaceListQuery{KubeContext: "prod"})
	prodPayload := waitForCondition(t, 250*time.Millisecond, func() (protocol.NamespaceListPayload, bool) {
		next := cache.Get(protocol.NamespaceListQuery{KubeContext: "prod"})
		return next, next.Freshness.State == protocol.FreshnessStateLive
	})

	if len(devPayload.Namespaces) < 2 || len(prodPayload.Namespaces) < 2 {
		t.Fatalf("expected at least 2 namespaces per payload")
	}
	if devPayload.Namespaces[1] == prodPayload.Namespaces[1] {
		t.Fatalf("expected per-context values, got %q", devPayload.Namespaces[1])
	}
}

func waitForCondition(
	t *testing.T,
	timeout time.Duration,
	check func() (protocol.NamespaceListPayload, bool),
) protocol.NamespaceListPayload {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		payload, ok := check()
		if ok {
			return payload
		}
		time.Sleep(10 * time.Millisecond)
	}
	payload, _ := check()
	t.Fatalf("condition not met within %s (last state=%s source=%s)", timeout, payload.Freshness.State, payload.Freshness.Source)
	return protocol.NamespaceListPayload{}
}

type fetchResponse struct {
	namespaces []string
	err        error
}

type queueFetcher struct {
	mu        sync.Mutex
	responses []fetchResponse
}

func (f *queueFetcher) List(context.Context, string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.responses) == 0 {
		return nil, errors.New("no response configured")
	}

	resp := f.responses[0]
	if len(f.responses) > 1 {
		f.responses = f.responses[1:]
	}
	return append([]string(nil), resp.namespaces...), resp.err
}

type contextFetcher struct{}

func (f *contextFetcher) List(_ context.Context, kubeContext string) ([]string, error) {
	return []string{"default", kubeContext + "-ns"}, nil
}
