package resources

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/dzhanguzin/k11s/internal/protocol"
)

const (
	defaultRefreshInterval = 3 * time.Second
	defaultStaleAfter      = 12 * time.Second
	defaultFetchTimeout    = 1500 * time.Millisecond
)

type Fetcher interface {
	List(ctx context.Context, query protocol.ResourceListQuery) ([]protocol.ResourceItem, error)
}

type Cache struct {
	ctx context.Context

	mu              sync.Mutex
	entries         map[cacheKey]*cacheEntry
	fetcher         Fetcher
	logger          *log.Logger
	now             func() time.Time
	refreshInterval time.Duration
	staleAfter      time.Duration
	fetchTimeout    time.Duration
}

type cacheKey struct {
	kubeContext string
	namespace   string
	resource    string
}

type cacheEntry struct {
	items      []protocol.ResourceItem
	lastSync   time.Time
	refreshing bool
	lastErr    string
}

func New(ctx context.Context, fetcher Fetcher, logger *log.Logger) *Cache {
	if ctx == nil {
		ctx = context.Background()
	}
	if fetcher == nil {
		fetcher = noopFetcher{}
	}

	return &Cache{
		ctx:             ctx,
		entries:         map[cacheKey]*cacheEntry{},
		fetcher:         fetcher,
		logger:          logger,
		now:             time.Now,
		refreshInterval: defaultRefreshInterval,
		staleAfter:      defaultStaleAfter,
		fetchTimeout:    defaultFetchTimeout,
	}
}

func (c *Cache) Get(query protocol.ResourceListQuery) protocol.ResourceListPayload {
	query = normalizeQuery(query)
	now := c.now()
	key := cacheKey{
		kubeContext: query.KubeContext,
		namespace:   query.Namespace,
		resource:    query.Resource,
	}

	c.mu.Lock()
	entry, ok := c.entries[key]
	if !ok {
		entry = &cacheEntry{}
		c.entries[key] = entry
	}

	if c.shouldRefresh(entry, now) {
		entry.refreshing = true
		go c.refresh(key, query)
	}

	payload := c.buildPayload(query, entry, now)
	c.mu.Unlock()

	return payload
}

func (c *Cache) shouldRefresh(entry *cacheEntry, now time.Time) bool {
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
		entry.lastErr = err.Error()
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

func (c *Cache) buildPayload(
	query protocol.ResourceListQuery,
	entry *cacheEntry,
	now time.Time,
) protocol.ResourceListPayload {
	meta := protocol.FreshnessMeta{
		State:              protocol.FreshnessStateCatchingUp,
		SnapshotTimeUnixMs: 0,
		AgeMs:              0,
		WatchHealthy:       entry.lastErr == "",
		Source:             "cache-cold",
	}

	if !entry.lastSync.IsZero() {
		age := now.Sub(entry.lastSync)
		meta.SnapshotTimeUnixMs = entry.lastSync.UnixMilli()
		meta.AgeMs = age.Milliseconds()
		meta.WatchHealthy = entry.lastErr == ""
		meta.State = protocol.FreshnessStateLive
		meta.Source = "cache"

		if entry.refreshing {
			meta.State = protocol.FreshnessStateCatchingUp
			meta.Source = "cache-refreshing"
		}
		if age >= c.staleAfter || entry.lastErr != "" {
			meta.State = protocol.FreshnessStateStale
			meta.Source = "cache-stale"
			if entry.refreshing {
				meta.State = protocol.FreshnessStateCatchingUp
				meta.Source = "cache-recovering"
			}
		}
	} else if entry.lastErr != "" {
		meta.State = protocol.FreshnessStateStale
		meta.Source = "cache-cold-error"
		meta.WatchHealthy = false
	}

	if query.SimulateStale {
		snapshot := now.Add(-3 * time.Minute)
		meta.State = protocol.FreshnessStateStale
		meta.SnapshotTimeUnixMs = snapshot.UnixMilli()
		meta.AgeMs = now.Sub(snapshot).Milliseconds()
		meta.WatchHealthy = false
		meta.Source = "cache-simulated-stale"
	}

	return protocol.ResourceListPayload{
		Resource:  query.Resource,
		Namespace: query.Namespace,
		Items:     append([]protocol.ResourceItem(nil), entry.items...),
		Freshness: meta,
	}
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
	return query
}

type noopFetcher struct{}

func (noopFetcher) List(context.Context, protocol.ResourceListQuery) ([]protocol.ResourceItem, error) {
	return nil, errors.New("resource fetcher is not configured")
}
