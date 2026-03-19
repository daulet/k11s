package namespaces

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
	defaultRefreshInterval = 5 * time.Second
	defaultStaleAfter      = 20 * time.Second
	defaultFetchTimeout    = 2 * time.Minute
)

type Fetcher interface {
	List(ctx context.Context, kubeContext string) ([]string, error)
}

type Cache struct {
	ctx context.Context

	mu              sync.Mutex
	entries         map[string]*entry
	fetcher         Fetcher
	logger          *log.Logger
	now             func() time.Time
	refreshInterval time.Duration
	staleAfter      time.Duration
	fetchTimeout    time.Duration
}

type entry struct {
	namespaces []string
	lastSync   time.Time
	refreshing bool
	lastErr    string
}

func New(ctx context.Context, fetcher Fetcher, logger *log.Logger) *Cache {
	if fetcher == nil {
		fetcher = noopFetcher{}
	}

	return &Cache{
		ctx:             ctx,
		entries:         map[string]*entry{},
		fetcher:         fetcher,
		logger:          logger,
		now:             time.Now,
		refreshInterval: defaultRefreshInterval,
		staleAfter:      defaultStaleAfter,
		fetchTimeout:    defaultFetchTimeout,
	}
}

func (c *Cache) Get(query protocol.NamespaceListQuery) protocol.NamespaceListPayload {
	kubeContext := strings.TrimSpace(query.KubeContext)
	now := c.now()

	c.mu.Lock()
	ent, ok := c.entries[kubeContext]
	if !ok {
		ent = &entry{}
		c.entries[kubeContext] = ent
	}

	if c.shouldRefresh(ent, now) {
		ent.refreshing = true
		go c.refresh(kubeContext)
	}

	payload := c.buildPayload(kubeContext, ent, now)
	c.mu.Unlock()
	return payload
}

func (c *Cache) shouldRefresh(ent *entry, now time.Time) bool {
	if ent.refreshing {
		return false
	}
	if ent.lastSync.IsZero() {
		return true
	}
	return now.Sub(ent.lastSync) >= c.refreshInterval
}

func (c *Cache) refresh(kubeContext string) {
	ctx, cancel := context.WithTimeout(c.ctx, c.fetchTimeout)
	defer cancel()

	namespaces, err := c.fetcher.List(ctx, kubeContext)

	c.mu.Lock()
	ent, ok := c.entries[kubeContext]
	if !ok {
		c.mu.Unlock()
		return
	}
	ent.refreshing = false

	if err != nil {
		ent.lastErr = cacheutil.FriendlyKubeAccessError(err, kubeContext)
		c.mu.Unlock()
		if c.logger != nil {
			c.logger.Printf("namespace refresh failed (ctx=%s): %v", kubeContext, err)
		}
		return
	}

	ent.lastErr = ""
	ent.lastSync = c.now()
	ent.namespaces = append([]string(nil), namespaces...)
	c.mu.Unlock()
}

func (c *Cache) buildPayload(kubeContext string, ent *entry, now time.Time) protocol.NamespaceListPayload {
	meta := protocol.FreshnessMeta{
		State:              protocol.FreshnessStateCatchingUp,
		SnapshotTimeUnixMs: 0,
		AgeMs:              0,
		WatchHealthy:       ent.lastErr == "",
		Source:             "cache-cold",
		Error:              ent.lastErr,
	}

	if !ent.lastSync.IsZero() {
		age := now.Sub(ent.lastSync)
		meta.SnapshotTimeUnixMs = ent.lastSync.UnixMilli()
		meta.AgeMs = age.Milliseconds()
		meta.WatchHealthy = ent.lastErr == ""
		meta.State = protocol.FreshnessStateLive
		meta.Source = "cache"

		if ent.refreshing {
			meta.State = protocol.FreshnessStateCatchingUp
			meta.Source = "cache-refreshing"
		}
		if age >= c.staleAfter || ent.lastErr != "" {
			meta.State = protocol.FreshnessStateStale
			meta.Source = "cache-stale"
			if ent.refreshing {
				meta.State = protocol.FreshnessStateCatchingUp
				meta.Source = "cache-recovering"
			}
		}
	} else if ent.lastErr != "" {
		meta.State = protocol.FreshnessStateStale
		meta.Source = "cache-cold-error"
		meta.WatchHealthy = false
	}

	return protocol.NamespaceListPayload{
		KubeContext: kubeContext,
		Namespaces:  append([]string(nil), ent.namespaces...),
		Freshness:   meta,
	}
}

type noopFetcher struct{}

func (noopFetcher) List(context.Context, string) ([]string, error) {
	return nil, errors.New("namespace fetcher is not configured")
}
