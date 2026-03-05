package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/daulet/k11s/internal/protocol"
)

const (
	socketFileName         = "k11sd.sock"
	sessionFileName        = "session.json"
	defaultConnectTimeout  = 150 * time.Millisecond
	defaultSpawnTimeout    = 2 * time.Second
	defaultRetryInterval   = 75 * time.Millisecond
	defaultWatchRetryDelay = 2 * time.Second // reserved for next phase
)

type Config struct {
	SocketPath      string
	SessionPath     string
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

	sessionPath := os.Getenv("K11S_SESSION")
	if sessionPath == "" {
		stateDir, err := resolveStateDir()
		if err != nil {
			return Config{}, err
		}
		sessionPath = filepath.Join(stateDir, sessionFileName)
	}

	return Config{
		SocketPath:      socketPath,
		SessionPath:     sessionPath,
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

func EnsureSessionDir(sessionPath string) error {
	dir := filepath.Dir(sessionPath)
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

func resolveStateDir() (string, error) {
	if xdgState := os.Getenv("XDG_STATE_HOME"); xdgState != "" {
		return filepath.Join(xdgState, "k11s"), nil
	}

	if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
		return filepath.Join(configDir, "k11s"), nil
	}

	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		return filepath.Join(homeDir, ".k11s"), nil
	}

	return "", errors.New("unable to resolve persistent state directory")
}
