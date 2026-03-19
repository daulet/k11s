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
}

type cacheKey struct {
	kubeContext string
	namespace   string
	resource    string
	filter      string
}

type cacheEntry struct {
	items      []protocol.ResourceItem
	lastSync   time.Time
	refreshing bool
	lastErr    string
	watching   bool
}

func New(ctx context.Context, fetcher Fetcher, logger *log.Logger) *Cache {
	if fetcher == nil {
		fetcher = noopFetcher{}
	}

	var watcher Watcher
	if typedWatcher, ok := fetcher.(Watcher); ok {
		watcher = typedWatcher
	}

	return &Cache{
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
	}
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

	if c.watcher != nil && !entry.watching {
		entry.watching = true
		entry.refreshing = true
		go c.watch(key, query)
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

	if c.watcher != nil && !entry.watching {
		entry.watching = true
		entry.refreshing = true
		go c.watch(key, listQuery)
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

func (c *Cache) watch(key cacheKey, query protocol.ResourceListQuery) {
	if c.watcher == nil {
		return
	}

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
		err := c.watcher.Watch(c.ctx, query, onUpdate, onError)
		if err != nil && !errors.Is(err, context.Canceled) {
			onError(err)
		}

		if c.ctx.Err() != nil || errors.Is(err, context.Canceled) {
			c.mu.Lock()
			if entry, ok := c.entries[key]; ok {
				entry.refreshing = false
			}
			c.mu.Unlock()
			return
		}

		select {
		case <-c.ctx.Done():
			c.mu.Lock()
			if entry, ok := c.entries[key]; ok {
				entry.refreshing = false
			}
			c.mu.Unlock()
			return
		case <-time.After(c.watchRetryDelay):
		}
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
