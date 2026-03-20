package resources

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	cacheutil "github.com/daulet/k11s/internal/cache"
	"github.com/daulet/k11s/internal/protocol"
)

const (
	defaultRefreshInterval = 3 * time.Second
	defaultStaleAfter      = 12 * time.Second
	defaultFetchTimeout    = 2 * time.Minute
	defaultWatchRetryDelay = 2 * time.Second
	defaultWatchIdleAfter  = 90 * time.Second
	defaultWatchSweepEvery = 10 * time.Second
)

type Fetcher interface {
	List(ctx context.Context, query protocol.ResourceListQuery) ([]protocol.ResourceItem, error)
}

type Watcher interface {
	Watch(
		ctx context.Context,
		query protocol.ResourceListQuery,
		onUpdate func(items []protocol.ResourceItem),
		onError func(error),
	) error
}

type Cache struct {
	ctx context.Context

	mu              sync.Mutex
	entries         map[cacheKey]*cacheEntry
	fetcher         Fetcher
	watcher         Watcher
	logger          *log.Logger
	now             func() time.Time
	refreshInterval time.Duration
	staleAfter      time.Duration
	fetchTimeout    time.Duration
	watchRetryDelay time.Duration
	watchIdleAfter  time.Duration
	watchSweepEvery time.Duration
}

type cacheKey struct {
	kubeContext string
	namespace   string
	resource    string
	filter      string
}

type cacheEntry struct {
	items       []protocol.ResourceItem
	lastSync    time.Time
	lastAccess  time.Time
	refreshing  bool
	lastErr     string
	watching    bool
	watchCancel context.CancelFunc
}

func New(ctx context.Context, fetcher Fetcher, logger *log.Logger) *Cache {
	if fetcher == nil {
		fetcher = noopFetcher{}
	}

	var watcher Watcher
	if typedWatcher, ok := fetcher.(Watcher); ok {
		watcher = typedWatcher
	}

	cache := &Cache{
		ctx:             ctx,
		entries:         map[cacheKey]*cacheEntry{},
		fetcher:         fetcher,
		watcher:         watcher,
		logger:          logger,
		now:             time.Now,
		refreshInterval: defaultRefreshInterval,
		staleAfter:      defaultStaleAfter,
		fetchTimeout:    defaultFetchTimeout,
		watchRetryDelay: defaultWatchRetryDelay,
		watchIdleAfter:  defaultWatchIdleAfter,
		watchSweepEvery: defaultWatchSweepEvery,
	}
	if cache.watcher != nil {
		go cache.reapIdleWatches()
	}
	return cache
}

func (c *Cache) Get(query protocol.ResourceListQuery) protocol.ResourceListPayload {
	query = normalizeQuery(query)
	now := c.now()
	key := cacheKey{
		kubeContext: query.KubeContext,
		namespace:   query.Namespace,
		resource:    query.Resource,
		filter:      query.Filter,
	}

	c.mu.Lock()
	entry, ok := c.entries[key]
	if !ok {
		entry = &cacheEntry{}
		c.entries[key] = entry
	}
	entry.lastAccess = now

	if c.watcher != nil && !entry.watching {
		entry.watching = true
		entry.refreshing = true
		watchCtx, watchCancel := context.WithCancel(c.ctx)
		entry.watchCancel = watchCancel
		go c.watch(watchCtx, key, query)
	} else if c.shouldRefresh(entry, now) {
		entry.refreshing = true
		go c.refresh(key, query)
	}

	payload := c.buildPayload(query, entry, now)
	c.mu.Unlock()

	return payload
}

func (c *Cache) GetDetail(query protocol.ResourceDetailQuery) protocol.ResourceDetailPayload {
	query = normalizeDetailQuery(query)
	now := c.now()
	key := cacheKey{
		kubeContext: query.KubeContext,
		namespace:   query.Namespace,
		resource:    query.Resource,
		filter:      query.Filter,
	}
	listQuery := protocol.ResourceListQuery{
		KubeContext:   query.KubeContext,
		Resource:      query.Resource,
		Namespace:     query.Namespace,
		Filter:        query.Filter,
		SimulateStale: query.SimulateStale,
	}

	c.mu.Lock()
	entry, ok := c.entries[key]
	if !ok {
		entry = &cacheEntry{}
		c.entries[key] = entry
	}
	entry.lastAccess = now

	if c.watcher != nil && !entry.watching {
		entry.watching = true
		entry.refreshing = true
		watchCtx, watchCancel := context.WithCancel(c.ctx)
		entry.watchCancel = watchCancel
		go c.watch(watchCtx, key, listQuery)
	} else if c.shouldRefresh(entry, now) {
		entry.refreshing = true
		go c.refresh(key, listQuery)
	}

	item, found := findDetailItem(entry.items, query)
	meta := c.buildFreshnessMeta(entry, now, query.SimulateStale)
	c.mu.Unlock()

	var itemCopy *protocol.ResourceItem
	itemNamespace := query.ItemNamespace
	if found {
		value := item
		itemCopy = &value
		if itemNamespace == "" {
			itemNamespace = item.Namespace
		}
	}

	return protocol.ResourceDetailPayload{
		Resource:      query.Resource,
		Namespace:     query.Namespace,
		ItemNamespace: itemNamespace,
		Name:          query.Name,
		Found:         found,
		Item:          itemCopy,
		Freshness:     meta,
	}
}

func (c *Cache) shouldRefresh(entry *cacheEntry, now time.Time) bool {
	if entry.watching {
		return false
	}
	if entry.refreshing {
		return false
	}
	if entry.lastSync.IsZero() {
		return true
	}
	return now.Sub(entry.lastSync) >= c.refreshInterval
}

func (c *Cache) refresh(key cacheKey, query protocol.ResourceListQuery) {
	ctx, cancel := context.WithTimeout(c.ctx, c.fetchTimeout)
	defer cancel()

	items, err := c.fetcher.List(ctx, query)

	c.mu.Lock()
	entry, ok := c.entries[key]
	if !ok {
		c.mu.Unlock()
		return
	}
	entry.refreshing = false

	if err != nil {
		entry.lastErr = cacheutil.FriendlyKubeAccessError(err, query.KubeContext)
		c.mu.Unlock()
		if c.logger != nil {
			c.logger.Printf(
				"resource refresh failed (ctx=%s ns=%s resource=%s): %v",
				query.KubeContext,
				query.Namespace,
				query.Resource,
				err,
			)
		}
		return
	}

	entry.lastErr = ""
	entry.lastSync = c.now()
	entry.items = append([]protocol.ResourceItem(nil), items...)
	c.mu.Unlock()
}

func (c *Cache) watch(watchCtx context.Context, key cacheKey, query protocol.ResourceListQuery) {
	if c.watcher == nil {
		return
	}
	defer c.markWatchStopped(key)

	onUpdate := func(items []protocol.ResourceItem) {
		c.mu.Lock()
		entry, ok := c.entries[key]
		if !ok {
			c.mu.Unlock()
			return
		}
		entry.items = append([]protocol.ResourceItem(nil), items...)
		entry.lastErr = ""
		entry.lastSync = c.now()
		entry.refreshing = false
		c.mu.Unlock()
	}

	onError := func(err error) {
		if err == nil {
			return
		}
		c.mu.Lock()
		entry, ok := c.entries[key]
		if !ok {
			c.mu.Unlock()
			return
		}
		entry.lastErr = cacheutil.FriendlyKubeAccessError(err, query.KubeContext)
		entry.refreshing = true
		c.mu.Unlock()

		if c.logger != nil {
			c.logger.Printf(
				"resource watch error (ctx=%s ns=%s resource=%s): %v",
				query.KubeContext,
				query.Namespace,
				query.Resource,
				err,
			)
		}
	}

	for {
		err := c.watcher.Watch(watchCtx, query, onUpdate, onError)
		if err != nil && !errors.Is(err, context.Canceled) && watchCtx.Err() == nil {
			onError(err)
		}

		if watchCtx.Err() != nil || errors.Is(err, context.Canceled) {
			return
		}

		select {
		case <-watchCtx.Done():
			return
		case <-time.After(c.watchRetryDelay):
		}
	}
}

func (c *Cache) markWatchStopped(key cacheKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return
	}
	entry.refreshing = false
	entry.watching = false
	entry.watchCancel = nil
}

func (c *Cache) reapIdleWatches() {
	if c.watchIdleAfter <= 0 || c.watchSweepEvery <= 0 {
		return
	}
	ticker := time.NewTicker(c.watchSweepEvery)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.cancelIdleWatches(c.now())
		}
	}
}

func (c *Cache) cancelIdleWatches(now time.Time) {
	if c.watchIdleAfter <= 0 {
		return
	}

	cancels := make([]context.CancelFunc, 0)
	c.mu.Lock()
	for _, entry := range c.entries {
		if !entry.watching || entry.watchCancel == nil {
			continue
		}
		if entry.lastAccess.IsZero() || now.Sub(entry.lastAccess) < c.watchIdleAfter {
			continue
		}
		cancels = append(cancels, entry.watchCancel)
		entry.watchCancel = nil
		entry.refreshing = false
	}
	c.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

func (c *Cache) buildPayload(
	query protocol.ResourceListQuery,
	entry *cacheEntry,
	now time.Time,
) protocol.ResourceListPayload {
	meta := c.buildFreshnessMeta(entry, now, query.SimulateStale)
	return protocol.ResourceListPayload{
		Resource:  query.Resource,
		Namespace: query.Namespace,
		Items:     append([]protocol.ResourceItem(nil), entry.items...),
		Freshness: meta,
	}
}

func (c *Cache) buildFreshnessMeta(entry *cacheEntry, now time.Time, simulateStale bool) protocol.FreshnessMeta {
	meta := protocol.FreshnessMeta{
		State:              protocol.FreshnessStateCatchingUp,
		SnapshotTimeUnixMs: 0,
		AgeMs:              0,
		WatchHealthy:       entry.lastErr == "" && !entry.lastSync.IsZero(),
		Source:             "cache-cold",
		Error:              entry.lastErr,
	}
	if entry.watching {
		meta.Source = "watch-cold"
	}

	if !entry.lastSync.IsZero() {
		age := now.Sub(entry.lastSync)
		meta.SnapshotTimeUnixMs = entry.lastSync.UnixMilli()
		meta.AgeMs = age.Milliseconds()
		meta.WatchHealthy = entry.lastErr == ""
		meta.State = protocol.FreshnessStateLive
		meta.Source = "cache"
		if entry.watching {
			meta.Source = "watch-cache"
		}

		if entry.refreshing {
			meta.State = protocol.FreshnessStateCatchingUp
			meta.Source = "cache-refreshing"
			if entry.watching {
				meta.Source = "watch-recovering"
			}
		}
		if age >= c.staleAfter || entry.lastErr != "" {
			meta.State = protocol.FreshnessStateStale
			meta.Source = "cache-stale"
			if entry.watching {
				meta.Source = "watch-stale"
			}
			if entry.refreshing {
				meta.State = protocol.FreshnessStateCatchingUp
				meta.Source = "cache-recovering"
				if entry.watching {
					meta.Source = "watch-recovering"
				}
			}
		}
	} else if entry.lastErr != "" {
		meta.State = protocol.FreshnessStateStale
		meta.Source = "cache-cold-error"
		meta.WatchHealthy = false
		if entry.watching {
			meta.Source = "watch-cold-error"
		}
	}

	if simulateStale {
		snapshot := now.Add(-3 * time.Minute)
		meta.State = protocol.FreshnessStateStale
		meta.SnapshotTimeUnixMs = snapshot.UnixMilli()
		meta.AgeMs = now.Sub(snapshot).Milliseconds()
		meta.WatchHealthy = false
		meta.Source = "cache-simulated-stale"
	}

	return meta
}

func normalizeQuery(query protocol.ResourceListQuery) protocol.ResourceListQuery {
	query.Resource = strings.TrimSpace(strings.ToLower(query.Resource))
	if query.Resource == "" {
		query.Resource = "pods"
	}

	query.Namespace = strings.TrimSpace(query.Namespace)
	if query.Namespace == "" {
		query.Namespace = "default"
	}

	query.KubeContext = strings.TrimSpace(query.KubeContext)
	query.Filter = strings.TrimSpace(query.Filter)
	return query
}

func normalizeDetailQuery(query protocol.ResourceDetailQuery) protocol.ResourceDetailQuery {
	query.Resource = strings.TrimSpace(strings.ToLower(query.Resource))
	if query.Resource == "" {
		query.Resource = "pods"
	}

	query.Namespace = strings.TrimSpace(query.Namespace)
	if query.Namespace == "" {
		query.Namespace = "default"
	}
	query.KubeContext = strings.TrimSpace(query.KubeContext)
	query.Filter = strings.TrimSpace(query.Filter)
	query.ItemNamespace = strings.TrimSpace(query.ItemNamespace)
	query.Name = strings.TrimSpace(query.Name)
	if query.Name == "" {
		return query
	}

	if query.ItemNamespace == "" {
		if ns, name, ok := strings.Cut(query.Name, "/"); ok {
			query.ItemNamespace = strings.TrimSpace(ns)
			query.Name = strings.TrimSpace(name)
		}
	}
	if query.ItemNamespace == "" && !strings.EqualFold(query.Namespace, "all") {
		query.ItemNamespace = query.Namespace
	}
	return query
}

func findDetailItem(items []protocol.ResourceItem, query protocol.ResourceDetailQuery) (protocol.ResourceItem, bool) {
	name := strings.TrimSpace(query.Name)
	if name == "" {
		return protocol.ResourceItem{}, false
	}
	itemNamespace := strings.TrimSpace(query.ItemNamespace)
	for _, item := range items {
		if item.Name != name {
			continue
		}
		if itemNamespace != "" && item.Namespace != itemNamespace {
			continue
		}
		return item, true
	}
	return protocol.ResourceItem{}, false
}

type noopFetcher struct{}

func (noopFetcher) List(context.Context, protocol.ResourceListQuery) ([]protocol.ResourceItem, error) {
	return nil, errors.New("resource fetcher is not configured")
}
