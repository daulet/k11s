package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/dzhanguzin/k11s/internal/protocol"
)

const (
	socketFileName         = "k11sd.sock"
	defaultConnectTimeout  = 150 * time.Millisecond
	defaultSpawnTimeout    = 2 * time.Second
	defaultRetryInterval   = 75 * time.Millisecond
	defaultWatchRetryDelay = 2 * time.Second // reserved for next phase
)

type Config struct {
	SocketPath      string
	ConnectTimeout  time.Duration
	SpawnTimeout    time.Duration
	RetryInterval   time.Duration
	WatchRetryDelay time.Duration
	RPCVersion      string
}

func Load() (Config, error) {
	socketPath := os.Getenv("K11S_SOCKET")
	if socketPath == "" {
		runtimeDir, err := resolveRuntimeDir()
		if err != nil {
			return Config{}, err
		}
		socketPath = filepath.Join(runtimeDir, socketFileName)
	}

	return Config{
		SocketPath:      socketPath,
		ConnectTimeout:  defaultConnectTimeout,
		SpawnTimeout:    defaultSpawnTimeout,
		RetryInterval:   defaultRetryInterval,
		WatchRetryDelay: defaultWatchRetryDelay,
		RPCVersion:      protocol.RPCVersion,
	}, nil
}

func EnsureSocketDir(socketPath string) error {
	dir := filepath.Dir(socketPath)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o700)
}

func resolveRuntimeDir() (string, error) {
	if xdgRuntime := os.Getenv("XDG_RUNTIME_DIR"); xdgRuntime != "" {
		return filepath.Join(xdgRuntime, "k11s"), nil
	}

	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		return filepath.Join(cacheDir, "k11s", "run"), nil
	}

	if tempDir := os.TempDir(); tempDir != "" {
		return filepath.Join(tempDir, "k11s"), nil
	}

	return "", errors.New("unable to resolve runtime directory for daemon socket")
}
