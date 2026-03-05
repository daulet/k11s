package resources

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dzhanguzin/k11s/internal/protocol"
)

func TestCacheColdStartTransitionsToLive(t *testing.T) {
	fetcher := &queueFetcher{
		responses: []fetchResponse{
			{
				items: []protocol.ResourceItem{
					{Name: "api-0", Namespace: "default", Status: "Running"},
				},
			},
		},
	}
	cache := New(context.Background(), fetcher, nil)

	initial := cache.Get(protocol.ResourceListQuery{Resource: "pods", Namespace: "default"})
	if initial.Freshness.State != protocol.FreshnessStateCatchingUp {
		t.Fatalf("expected cold state CATCHING_UP, got %s", initial.Freshness.State)
	}

	payload := waitForCondition(t, 250*time.Millisecond, func() (protocol.ResourceListPayload, bool) {
		next := cache.Get(protocol.ResourceListQuery{Resource: "pods", Namespace: "default"})
		return next, next.Freshness.State == protocol.FreshnessStateLive && len(next.Items) == 1
	})

	if got := payload.Items[0].Name; got != "api-0" {
		t.Fatalf("expected api-0 item, got %q", got)
	}
}

func TestCachePreservesItemsWhenRefreshFails(t *testing.T) {
	fetcher := &queueFetcher{
		responses: []fetchResponse{
			{
				items: []protocol.ResourceItem{
					{Name: "svc-a", Namespace: "default", Status: "ClusterIP"},
				},
			},
			{
				err: errors.New("apiserver timeout"),
			},
		},
	}
	cache := New(context.Background(), fetcher, nil)
	query := protocol.ResourceListQuery{Resource: "services", Namespace: "default"}

	_ = cache.Get(query)
	_ = waitForCondition(t, 250*time.Millisecond, func() (protocol.ResourceListPayload, bool) {
		next := cache.Get(query)
		return next, next.Freshness.State == protocol.FreshnessStateLive
	})

	normalized := normalizeQuery(query)
	key := cacheKey{
		kubeContext: normalized.KubeContext,
		namespace:   normalized.Namespace,
		resource:    normalized.Resource,
	}

	cache.mu.Lock()
	entry := cache.entries[key]
	entry.refreshing = true
	cache.refreshInterval = time.Hour
	cache.mu.Unlock()

	cache.refresh(key, normalized)

	payload := cache.Get(query)
	if payload.Freshness.State != protocol.FreshnessStateStale {
		t.Fatalf("expected stale state after refresh failure, got %s", payload.Freshness.State)
	}

	if len(payload.Items) != 1 {
		t.Fatalf("expected cached items to remain on failure, got %d", len(payload.Items))
	}
	if payload.Items[0].Name != "svc-a" {
		t.Fatalf("expected cached item svc-a, got %q", payload.Items[0].Name)
	}
}

func TestCacheKeysIncludeKubeContext(t *testing.T) {
	fetcher := &contextAwareFetcher{}
	cache := New(context.Background(), fetcher, nil)

	_ = cache.Get(protocol.ResourceListQuery{
		KubeContext: "dev",
		Resource:    "pods",
		Namespace:   "default",
	})
	devPayload := waitForCondition(t, 250*time.Millisecond, func() (protocol.ResourceListPayload, bool) {
		next := cache.Get(protocol.ResourceListQuery{
			KubeContext: "dev",
			Resource:    "pods",
			Namespace:   "default",
		})
		return next, next.Freshness.State == protocol.FreshnessStateLive
	})

	_ = cache.Get(protocol.ResourceListQuery{
		KubeContext: "prod",
		Resource:    "pods",
		Namespace:   "default",
	})
	prodPayload := waitForCondition(t, 250*time.Millisecond, func() (protocol.ResourceListPayload, bool) {
		next := cache.Get(protocol.ResourceListQuery{
			KubeContext: "prod",
			Resource:    "pods",
			Namespace:   "default",
		})
		return next, next.Freshness.State == protocol.FreshnessStateLive
	})

	if devPayload.Items[0].Name == prodPayload.Items[0].Name {
		t.Fatalf("expected separate cache entries per context, got %q", devPayload.Items[0].Name)
	}
}

func waitForCondition(
	t *testing.T,
	timeout time.Duration,
	check func() (protocol.ResourceListPayload, bool),
) protocol.ResourceListPayload {
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
	return protocol.ResourceListPayload{}
}

type fetchResponse struct {
	items []protocol.ResourceItem
	err   error
}

type queueFetcher struct {
	mu        sync.Mutex
	responses []fetchResponse
}

func (f *queueFetcher) List(context.Context, protocol.ResourceListQuery) ([]protocol.ResourceItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.responses) == 0 {
		return nil, errors.New("no response configured")
	}

	resp := f.responses[0]
	if len(f.responses) > 1 {
		f.responses = f.responses[1:]
	}
	return append([]protocol.ResourceItem(nil), resp.items...), resp.err
}

type contextAwareFetcher struct{}

func (f *contextAwareFetcher) List(ctx context.Context, query protocol.ResourceListQuery) ([]protocol.ResourceItem, error) {
	name := query.KubeContext + "-pod"
	if query.KubeContext == "" {
		name = "default-pod"
	}
	return []protocol.ResourceItem{
		{Name: name, Namespace: query.Namespace, Status: "Running"},
	}, nil
}
