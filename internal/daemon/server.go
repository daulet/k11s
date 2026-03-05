package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	namespacecache "github.com/daulet/k11s/internal/cache/namespaces"
	resourcecache "github.com/daulet/k11s/internal/cache/resources"
	"github.com/daulet/k11s/internal/config"
	"github.com/daulet/k11s/internal/kube"
	"github.com/daulet/k11s/internal/protocol"
	"github.com/daulet/k11s/internal/session"
)

func Run(ctx context.Context, cfg config.Config, daemonVersion string) error {
	if err := config.EnsureSocketDir(cfg.SocketPath); err != nil {
		return fmt.Errorf("prepare socket directory: %w", err)
	}
	if err := config.EnsureSessionDir(cfg.SessionPath); err != nil {
		return fmt.Errorf("prepare session directory: %w", err)
	}
	if err := removeStaleSocket(cfg.SocketPath, cfg.ConnectTimeout); err != nil {
		return err
	}

	listener, err := net.Listen("unix", cfg.SocketPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.SocketPath, err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(cfg.SocketPath)
	}()

	if err := os.Chmod(cfg.SocketPath, 0o600); err != nil {
		return fmt.Errorf("set socket permissions: %w", err)
	}

	logger := log.New(os.Stderr, "k11sd: ", log.LstdFlags)
	logger.Printf("listening on %s", cfg.SocketPath)
	store := session.NewStore(cfg.SessionPath)
	clientFactory := kube.NewClientFactory()
	resourceCache := resourcecache.New(ctx, kube.NewResourceFetcher(clientFactory), logger)
	namespaceCache := namespacecache.New(ctx, kube.NewNamespaceFetcher(clientFactory), logger)

	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			logger.Print("graceful shutdown requested")
			_ = listener.Close()
		})
	}

	go func() {
		<-ctx.Done()
		shutdown()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return nil
			}
			logger.Printf("accept error: %v", err)
			continue
		}

		go handleConn(conn, daemonVersion, shutdown, store, resourceCache, namespaceCache, logger)
	}
}

func handleConn(
	conn net.Conn,
	daemonVersion string,
	shutdown func(),
	store *session.Store,
	resourceCache *resourcecache.Cache,
	namespaceCache *namespacecache.Cache,
	logger *log.Logger,
) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	var req protocol.HandshakeRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(protocol.HandshakeResponse{
			Compatible:    false,
			DaemonVersion: daemonVersion,
			RPCVersion:    protocol.RPCVersion,
			PID:           os.Getpid(),
			Message:       fmt.Sprintf("decode handshake: %v", err),
		})
		return
	}

	switch req.Intent {
	case protocol.IntentShutdown:
		resp := protocol.BuildShutdownResponse(req, daemonVersion, os.Getpid())
		_ = json.NewEncoder(conn).Encode(resp)
		if resp.Compatible {
			shutdown()
		}
		return
	case protocol.IntentSessionGet:
		state, err := store.Load()
		if err != nil && !errors.Is(err, session.ErrCorruptSession) {
			_ = json.NewEncoder(conn).Encode(protocol.HandshakeResponse{
				Compatible:    false,
				DaemonVersion: daemonVersion,
				RPCVersion:    protocol.RPCVersion,
				PID:           os.Getpid(),
				Message:       fmt.Sprintf("load session: %v", err),
			})
			return
		}
		if errors.Is(err, session.ErrCorruptSession) {
			logger.Printf("session file corrupt, using defaults: %v", err)
		}
		resp := protocol.BuildSessionGetResponse(req, daemonVersion, os.Getpid(), state)
		_ = json.NewEncoder(conn).Encode(resp)
		return
	case protocol.IntentSessionSave:
		if req.Session == nil {
			_ = json.NewEncoder(conn).Encode(protocol.HandshakeResponse{
				Compatible:    false,
				DaemonVersion: daemonVersion,
				RPCVersion:    protocol.RPCVersion,
				PID:           os.Getpid(),
				Message:       "missing session payload",
			})
			return
		}
		if err := store.Save(*req.Session); err != nil {
			_ = json.NewEncoder(conn).Encode(protocol.HandshakeResponse{
				Compatible:    false,
				DaemonVersion: daemonVersion,
				RPCVersion:    protocol.RPCVersion,
				PID:           os.Getpid(),
				Message:       fmt.Sprintf("save session: %v", err),
			})
			return
		}
		resp := protocol.BuildSessionSaveResponse(req, daemonVersion, os.Getpid())
		_ = json.NewEncoder(conn).Encode(resp)
		return
	case protocol.IntentResourceList:
		if req.ListQuery == nil {
			_ = json.NewEncoder(conn).Encode(protocol.HandshakeResponse{
				Compatible:    false,
				DaemonVersion: daemonVersion,
				RPCVersion:    protocol.RPCVersion,
				PID:           os.Getpid(),
				Message:       "missing list query payload",
			})
			return
		}

		query := *req.ListQuery
		resource := strings.ToLower(strings.TrimSpace(query.Resource))
		if resource == "" {
			resource = "pods"
		}

		var payload protocol.ResourceListPayload
		if kube.IsCoreResource(resource) {
			payload = resourceCache.Get(query)
		} else {
			payload = buildPlaceholderResourceList(query)
		}
		resp := protocol.BuildResourceListResponse(req, daemonVersion, os.Getpid(), payload)
		_ = json.NewEncoder(conn).Encode(resp)
		return
	case protocol.IntentNamespaceList:
		if req.NamespaceQuery == nil {
			_ = json.NewEncoder(conn).Encode(protocol.HandshakeResponse{
				Compatible:    false,
				DaemonVersion: daemonVersion,
				RPCVersion:    protocol.RPCVersion,
				PID:           os.Getpid(),
				Message:       "missing namespace query payload",
			})
			return
		}
		payload := namespaceCache.Get(*req.NamespaceQuery)
		resp := protocol.BuildNamespaceListResponse(req, daemonVersion, os.Getpid(), payload)
		_ = json.NewEncoder(conn).Encode(resp)
		return
	}

	resp := protocol.BuildHandshakeResponse(req, daemonVersion, os.Getpid())
	_ = json.NewEncoder(conn).Encode(resp)
}

func buildPlaceholderResourceList(query protocol.ResourceListQuery) protocol.ResourceListPayload {
	namespace := query.Namespace
	if namespace == "" {
		namespace = "default"
	}

	now := time.Now()
	freshness := protocol.FreshnessMeta{
		State:              protocol.FreshnessStateLive,
		SnapshotTimeUnixMs: now.UnixMilli(),
		AgeMs:              0,
		WatchHealthy:       true,
		Source:             "cache",
	}

	if query.SimulateStale {
		snapshot := now.Add(-3 * time.Minute)
		freshness = protocol.FreshnessMeta{
			State:              protocol.FreshnessStateStale,
			SnapshotTimeUnixMs: snapshot.UnixMilli(),
			AgeMs:              now.Sub(snapshot).Milliseconds(),
			WatchHealthy:       false,
			Source:             "cache-stale",
		}
	}

	resource := query.Resource
	if resource == "" {
		resource = "pods"
	}

	items := []protocol.ResourceItem{
		{Name: "api-7d9b", Namespace: namespace, Status: "Running"},
		{Name: "worker-5f8a", Namespace: namespace, Status: "Running"},
		{Name: "db-migration-2821", Namespace: namespace, Status: "Completed"},
	}

	return protocol.ResourceListPayload{
		Resource:  resource,
		Namespace: namespace,
		Items:     items,
		Freshness: freshness,
	}
}

func removeStaleSocket(socketPath string, timeout time.Duration) error {
	info, err := os.Lstat(socketPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat socket path: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("path exists but is not a unix socket: %s", socketPath)
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.Dial("unix", socketPath)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("daemon already running on %s", socketPath)
	}

	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

	return nil
}
