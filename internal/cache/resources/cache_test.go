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
	if !strings.Contains(payload.Freshness.Error, "apiserver timeout") {
		t.Fatalf("expected freshness error to include refresh failure, got %q", payload.Freshness.Error)
	}
}

func TestCacheAuthExpiryErrorShowsReloginPrompt(t *testing.T) {
	fetcher := &queueFetcher{
		responses: []fetchResponse{
			{
				items: []protocol.ResourceItem{
					{Name: "svc-a", Namespace: "default", Status: "ClusterIP"},
				},
			},
			{
				err: errors.New(`Get "https://nv-stg-hw.teleport.sh:443/api/v1/pods?watch=true": getting credentials: exec: executable /usr/local/bin/tsh failed with exit code 1`),
			},
		},
	}
	cache := New(context.Background(), fetcher, nil)
	query := protocol.ResourceListQuery{
		KubeContext: "mc1-lab1",
		Resource:    "pods",
		Namespace:   "default",
	}

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

func TestListFilterAppliesWithoutRefetching(t *testing.T) {
	fetcher := &listFilterFetcher{}
	cache := New(context.Background(), fetcher, nil)

	query := protocol.ResourceListQuery{
		KubeContext: "dev",
		Resource:    "pods",
		Namespace:   "default",
	}
	_ = cache.Get(query)
	_ = waitForCondition(t, 250*time.Millisecond, func() (protocol.ResourceListPayload, bool) {
		next := cache.Get(query)
		return next, next.Freshness.State == protocol.FreshnessStateLive && len(next.Items) == 3
	})

	filtered := cache.Get(protocol.ResourceListQuery{
		KubeContext: "dev",
		Resource:    "pods",
		Namespace:   "default",
		ListFilter:  "node~rack-a*",
	})
	if len(filtered.Items) != 2 {
		t.Fatalf("expected two filtered pods, got %#v", filtered.Items)
	}
	if filtered.TotalItems != 3 {
		t.Fatalf("expected total item count before filtering to be 3, got %d", filtered.TotalItems)
	}
	if filtered.ListFilter != "node~rack-a*" {
		t.Fatalf("expected normalized list filter, got %q", filtered.ListFilter)
	}
	if fetcher.callCount() != 1 {
		t.Fatalf("expected filtered query to reuse warm cache, got %d fetches", fetcher.callCount())
	}
}

func TestListFilterIgnoredForNonPodResources(t *testing.T) {
	fetcher := &listFilterFetcher{}
	cache := New(context.Background(), fetcher, nil)

	query := protocol.ResourceListQuery{
		KubeContext: "dev",
		Resource:    "services",
		Namespace:   "default",
		ListFilter:  "node=rack-a-1",
	}
	_ = cache.Get(query)
	payload := waitForCondition(t, 250*time.Millisecond, func() (protocol.ResourceListPayload, bool) {
		next := cache.Get(query)
		return next, next.Freshness.State == protocol.FreshnessStateLive
	})

	if len(payload.Items) != 3 {
		t.Fatalf("expected non-pod resource to ignore node filter, got %#v", payload.Items)
	}
	if payload.ListFilter != "" {
		t.Fatalf("expected inactive list filter for non-pod resource, got %q", payload.ListFilter)
	}
}

func TestCoreResourceTabSwitchUsesWarmCacheWithoutRefetch(t *testing.T) {
	fetcher := &countingCoreFetcher{}
	cache := New(context.Background(), fetcher, nil)
	queries := []protocol.ResourceListQuery{
		{Resource: "pods", Namespace: "default"},
		{Resource: "services", Namespace: "default"},
		{Resource: "deployments", Namespace: "default"},
	}

	for _, query := range queries {
		_ = cache.Get(query)
		payload := waitForCondition(t, 250*time.Millisecond, func() (protocol.ResourceListPayload, bool) {
			next := cache.Get(query)
			return next, next.Freshness.State == protocol.FreshnessStateLive && len(next.Items) == 1
		})
		expectedName := query.Resource + "-item"
		if payload.Items[0].Name != expectedName {
			t.Fatalf("expected warm item %q, got %q", expectedName, payload.Items[0].Name)
		}
	}

	initialCalls := fetcher.callCount()
	if initialCalls < len(queries) {
		t.Fatalf("expected at least %d initial fetch calls, got %d", len(queries), initialCalls)
	}

	cache.mu.Lock()
	cache.refreshInterval = time.Hour
	cache.mu.Unlock()

	start := time.Now()
	for i := 0; i < 300; i++ {
		for _, query := range queries {
			payload := cache.Get(query)
			if payload.Freshness.State != protocol.FreshnessStateLive {
				t.Fatalf("expected LIVE cache payload for %q, got %s", query.Resource, payload.Freshness.State)
			}
			if len(payload.Items) != 1 {
				t.Fatalf("expected cached item for %q, got %d", query.Resource, len(payload.Items))
			}
		}
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("expected warm tab-switch cache path to stay fast, took %s", elapsed)
	}

	if got := fetcher.callCount(); got != initialCalls {
		t.Fatalf("expected no refetch during warm tab switches, initial=%d got=%d", initialCalls, got)
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

func TestCacheCancelsIdleWatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetcher := &blockingWatchFetcher{}
	cache := New(ctx, fetcher, nil)
	cache.watchIdleAfter = time.Nanosecond

	query := protocol.ResourceListQuery{Resource: "nodes", Namespace: "all"}
	_ = cache.Get(query)

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if fetcher.watchCallCount() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if fetcher.watchCallCount() == 0 {
		t.Fatalf("expected watcher to start")
	}

	cache.cancelIdleWatches(time.Now().Add(2 * time.Second))

	deadline = time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if fetcher.cancelCount() > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected idle watcher to be canceled")
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

type listFilterFetcher struct {
	mu    sync.Mutex
	calls int
}

func (f *listFilterFetcher) List(context.Context, protocol.ResourceListQuery) ([]protocol.ResourceItem, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()

	return []protocol.ResourceItem{
		{Name: "api-a", Namespace: "default", Status: "Running", Node: "rack-a-1"},
		{Name: "api-b", Namespace: "default", Status: "Running", Node: "rack-a-2"},
		{Name: "api-c", Namespace: "default", Status: "Running", Node: "rack-b-1"},
	}, nil
}

func (f *listFilterFetcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type countingCoreFetcher struct {
	mu    sync.Mutex
	calls int
}

func (f *countingCoreFetcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *countingCoreFetcher) List(_ context.Context, query protocol.ResourceListQuery) ([]protocol.ResourceItem, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()

	resource := strings.ToLower(strings.TrimSpace(query.Resource))
	if resource == "" {
		resource = "pods"
	}

	return []protocol.ResourceItem{
		{
			Name:      resource + "-item",
			Namespace: query.Namespace,
			Status:    "Running",
		},
	}, nil
}

type channelWatchFetcher struct {
	mu        sync.Mutex
	updates   chan []protocol.ResourceItem
	watchRuns int
	listRuns  int
}

type blockingWatchFetcher struct {
	mu      sync.Mutex
	watches int
	cancels int
}

func (f *blockingWatchFetcher) watchCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.watches
}

func (f *blockingWatchFetcher) cancelCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cancels
}

func (f *blockingWatchFetcher) List(context.Context, protocol.ResourceListQuery) ([]protocol.ResourceItem, error) {
	return nil, errors.New("list should not be used when watcher is available")
}

func (f *blockingWatchFetcher) Watch(
	ctx context.Context,
	_ protocol.ResourceListQuery,
	_ func(items []protocol.ResourceItem),
	_ func(error),
) error {
	f.mu.Lock()
	f.watches++
	f.mu.Unlock()

	<-ctx.Done()

	f.mu.Lock()
	f.cancels++
	f.mu.Unlock()
	return nil
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
