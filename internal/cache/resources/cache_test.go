package resources

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/daulet/k11s/internal/protocol"
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

func TestCacheKeysIncludeFilter(t *testing.T) {
	fetcher := &filterAwareFetcher{}
	cache := New(context.Background(), fetcher, nil)

	_ = cache.Get(protocol.ResourceListQuery{
		KubeContext: "dev",
		Resource:    "crs",
		Namespace:   "all",
		Filter:      "widgets.example.com",
	})
	widgetsPayload := waitForCondition(t, 250*time.Millisecond, func() (protocol.ResourceListPayload, bool) {
		next := cache.Get(protocol.ResourceListQuery{
			KubeContext: "dev",
			Resource:    "crs",
			Namespace:   "all",
			Filter:      "widgets.example.com",
		})
		return next, next.Freshness.State == protocol.FreshnessStateLive && len(next.Items) == 1
	})

	_ = cache.Get(protocol.ResourceListQuery{
		KubeContext: "dev",
		Resource:    "crs",
		Namespace:   "all",
		Filter:      "gadgets.example.com",
	})
	gadgetsPayload := waitForCondition(t, 250*time.Millisecond, func() (protocol.ResourceListPayload, bool) {
		next := cache.Get(protocol.ResourceListQuery{
			KubeContext: "dev",
			Resource:    "crs",
			Namespace:   "all",
			Filter:      "gadgets.example.com",
		})
		return next, next.Freshness.State == protocol.FreshnessStateLive && len(next.Items) == 1
	})

	if widgetsPayload.Items[0].Name == gadgetsPayload.Items[0].Name {
		t.Fatalf("expected separate cache entries per filter, got %q", widgetsPayload.Items[0].Name)
	}
}

func TestCacheUsesWatcherWhenAvailable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetcher := newChannelWatchFetcher()
	cache := New(ctx, fetcher, nil)
	query := protocol.ResourceListQuery{Resource: "pods", Namespace: "default"}

	initial := cache.Get(query)
	if initial.Freshness.State != protocol.FreshnessStateCatchingUp {
		t.Fatalf("expected cold state CATCHING_UP, got %s", initial.Freshness.State)
	}

	fetcher.push([]protocol.ResourceItem{
		{Name: "api-watch-0", Namespace: "default", Status: "Running"},
	})

	payload := waitForCondition(t, 350*time.Millisecond, func() (protocol.ResourceListPayload, bool) {
		next := cache.Get(query)
		return next, next.Freshness.State == protocol.FreshnessStateLive && len(next.Items) == 1
	})

	if payload.Items[0].Name != "api-watch-0" {
		t.Fatalf("expected watch item api-watch-0, got %#v", payload.Items)
	}
	if payload.Freshness.Source != "watch-cache" {
		t.Fatalf("expected watch source, got %q", payload.Freshness.Source)
	}
	if fetcher.watchCallCount() == 0 {
		t.Fatalf("expected watcher to be called")
	}
	if fetcher.listCallCount() != 0 {
		t.Fatalf("expected poll list path not to be used, listCalls=%d", fetcher.listCallCount())
	}
}

func TestCacheDetailColdStartIsCatchingUp(t *testing.T) {
	fetcher := &queueFetcher{
		responses: []fetchResponse{
			{
				items: []protocol.ResourceItem{
					{Name: "api", Namespace: "default", Status: "Running"},
				},
			},
		},
	}
	cache := New(context.Background(), fetcher, nil)

	detail := cache.GetDetail(protocol.ResourceDetailQuery{
		Resource:  "pods",
		Namespace: "default",
		Name:      "api",
	})
	if detail.Found {
		t.Fatalf("expected cold-start detail to be unresolved before initial sync")
	}
	if detail.Freshness.State != protocol.FreshnessStateCatchingUp {
		t.Fatalf("expected CATCHING_UP on cold detail, got %s", detail.Freshness.State)
	}
}

func TestCacheDetailResolvesWithItemNamespaceInAllScope(t *testing.T) {
	fetcher := &queueFetcher{
		responses: []fetchResponse{
			{
				items: []protocol.ResourceItem{
					{Name: "api", Namespace: "default", Status: "Running"},
					{Name: "api", Namespace: "payments", Status: "Running"},
				},
			},
		},
	}
	cache := New(context.Background(), fetcher, nil)
	query := protocol.ResourceListQuery{Resource: "pods", Namespace: "all"}
	_ = cache.Get(query)
	_ = waitForCondition(t, 250*time.Millisecond, func() (protocol.ResourceListPayload, bool) {
		next := cache.Get(query)
		return next, next.Freshness.State == protocol.FreshnessStateLive && len(next.Items) == 2
	})

	detail := cache.GetDetail(protocol.ResourceDetailQuery{
		Resource:      "pods",
		Namespace:     "all",
		Name:          "api",
		ItemNamespace: "payments",
	})
	if !detail.Found || detail.Item == nil {
		t.Fatalf("expected detail item to be found")
	}
	if detail.Item.Namespace != "payments" {
		t.Fatalf("expected detail namespace payments, got %q", detail.Item.Namespace)
	}
	if detail.Freshness.State != protocol.FreshnessStateLive {
		t.Fatalf("expected live detail freshness, got %s", detail.Freshness.State)
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

type filterAwareFetcher struct{}

func (f *filterAwareFetcher) List(ctx context.Context, query protocol.ResourceListQuery) ([]protocol.ResourceItem, error) {
	name := strings.TrimSpace(query.Filter)
	if name == "" {
		name = "no-filter"
	}
	return []protocol.ResourceItem{
		{Name: name + "-item", Namespace: query.Namespace, Status: "Running"},
	}, nil
}

type channelWatchFetcher struct {
	mu        sync.Mutex
	updates   chan []protocol.ResourceItem
	watchRuns int
	listRuns  int
}

func newChannelWatchFetcher() *channelWatchFetcher {
	return &channelWatchFetcher{
		updates: make(chan []protocol.ResourceItem, 8),
	}
}

func (f *channelWatchFetcher) push(items []protocol.ResourceItem) {
	f.updates <- append([]protocol.ResourceItem(nil), items...)
}

func (f *channelWatchFetcher) watchCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.watchRuns
}

func (f *channelWatchFetcher) listCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.listRuns
}

func (f *channelWatchFetcher) List(context.Context, protocol.ResourceListQuery) ([]protocol.ResourceItem, error) {
	f.mu.Lock()
	f.listRuns++
	f.mu.Unlock()
	return nil, errors.New("list should not be used when watcher is available")
}

func (f *channelWatchFetcher) Watch(
	ctx context.Context,
	_ protocol.ResourceListQuery,
	onUpdate func(items []protocol.ResourceItem),
	_ func(error),
) error {
	f.mu.Lock()
	f.watchRuns++
	f.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return nil
		case update := <-f.updates:
			onUpdate(update)
		}
	}
}
