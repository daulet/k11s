package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dzhanguzin/k11s/internal/config"
	"github.com/dzhanguzin/k11s/internal/protocol"
)

type BootstrapResult struct {
	Spawned   bool
	Restarted bool
	Handshake protocol.HandshakeResponse
	Metrics   BootstrapMetrics
}

type BootstrapMetrics struct {
	ConnectDuration   time.Duration
	HandshakeDuration time.Duration
	BootstrapDuration time.Duration
}

type HandshakeMetrics struct {
	ConnectDuration   time.Duration
	HandshakeDuration time.Duration
}

type IncompatibleRPCError struct {
	ClientRPC string
	DaemonRPC string
	Message   string
}

func (e *IncompatibleRPCError) Error() string {
	return fmt.Sprintf("daemon/client rpc mismatch (client=%s daemon=%s): %s", e.ClientRPC, e.DaemonRPC, e.Message)
}

type DaemonUnavailableError struct {
	Cause error
}

func (e *DaemonUnavailableError) Error() string {
	return fmt.Sprintf("daemon unavailable: %v", e.Cause)
}

func (e *DaemonUnavailableError) Unwrap() error {
	return e.Cause
}

func Bootstrap(ctx context.Context, cfg config.Config, clientVersion string) (BootstrapResult, error) {
	bootstrapStart := time.Now()
	resp, handshakeMetrics, err := handshake(ctx, cfg, clientVersion)
	if err == nil {
		if shouldUpgradeDaemon(clientVersion, resp.DaemonVersion) {
			result, restartErr := restartDaemon(ctx, cfg, clientVersion, resp)
			if restartErr != nil {
				return BootstrapResult{}, restartErr
			}
			result.Metrics.BootstrapDuration = time.Since(bootstrapStart)
			return result, nil
		}
		return BootstrapResult{
			Spawned:   false,
			Restarted: false,
			Handshake: resp,
			Metrics: BootstrapMetrics{
				ConnectDuration:   handshakeMetrics.ConnectDuration,
				HandshakeDuration: handshakeMetrics.HandshakeDuration,
				BootstrapDuration: time.Since(bootstrapStart),
			},
		}, nil
	}

	var incompatibleErr *IncompatibleRPCError
	if errors.As(err, &incompatibleErr) {
		return BootstrapResult{}, err
	}

	var unavailableErr *DaemonUnavailableError
	if !errors.As(err, &unavailableErr) {
		return BootstrapResult{}, err
	}

	if err := spawnDaemon(cfg.SocketPath); err != nil {
		return BootstrapResult{}, fmt.Errorf("start daemon: %w", err)
	}

	resp, handshakeMetrics, err = waitForDaemonReady(ctx, cfg, clientVersion)
	if err != nil {
		return BootstrapResult{}, err
	}

	return BootstrapResult{
		Spawned:   true,
		Restarted: false,
		Handshake: resp,
		Metrics: BootstrapMetrics{
			ConnectDuration:   handshakeMetrics.ConnectDuration,
			HandshakeDuration: handshakeMetrics.HandshakeDuration,
			BootstrapDuration: time.Since(bootstrapStart),
		},
	}, nil
}

func restartDaemon(ctx context.Context, cfg config.Config, clientVersion string, current protocol.HandshakeResponse) (BootstrapResult, error) {
	if err := requestDaemonShutdown(ctx, cfg, clientVersion); err != nil {
		if signalErr := signalDaemonProcess(current.PID); signalErr != nil {
			return BootstrapResult{}, fmt.Errorf(
				"request graceful daemon shutdown (pid=%d): %v (signal fallback failed: %v)",
				current.PID,
				err,
				signalErr,
			)
		}
	}

	if err := waitForDaemonExit(ctx, cfg, clientVersion); err != nil {
		return BootstrapResult{}, err
	}

	if err := spawnDaemon(cfg.SocketPath); err != nil {
		return BootstrapResult{}, fmt.Errorf("start upgraded daemon: %w", err)
	}

	resp, handshakeMetrics, err := waitForDaemonReady(ctx, cfg, clientVersion)
	if err != nil {
		return BootstrapResult{}, err
	}
	if shouldUpgradeDaemon(clientVersion, resp.DaemonVersion) {
		return BootstrapResult{}, fmt.Errorf(
			"daemon restart completed but version mismatch remains: client=%s daemon=%s",
			clientVersion,
			resp.DaemonVersion,
		)
	}

	return BootstrapResult{
		Spawned:   true,
		Restarted: true,
		Handshake: resp,
		Metrics: BootstrapMetrics{
			ConnectDuration:   handshakeMetrics.ConnectDuration,
			HandshakeDuration: handshakeMetrics.HandshakeDuration,
		},
	}, nil
}

func waitForDaemonReady(ctx context.Context, cfg config.Config, clientVersion string) (protocol.HandshakeResponse, HandshakeMetrics, error) {
	deadline := time.Now().Add(cfg.SpawnTimeout)
	for {
		resp, handshakeMetrics, err := handshake(ctx, cfg, clientVersion)
		if err == nil {
			return resp, handshakeMetrics, nil
		}
		var unavailableErr *DaemonUnavailableError
		if !errors.As(err, &unavailableErr) {
			return protocol.HandshakeResponse{}, HandshakeMetrics{}, err
		}
		if time.Now().After(deadline) {
			return protocol.HandshakeResponse{}, HandshakeMetrics{}, fmt.Errorf("daemon did not become ready within %s: %w", cfg.SpawnTimeout, err)
		}
		time.Sleep(cfg.RetryInterval)
	}
}

func requestDaemonShutdown(ctx context.Context, cfg config.Config, clientVersion string) error {
	dialer := &net.Dialer{Timeout: cfg.ConnectTimeout}
	conn, err := dialer.DialContext(ctx, "unix", cfg.SocketPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(cfg.ConnectTimeout))

	req := protocol.HandshakeRequest{
		ClientName:    "k11s",
		ClientVersion: clientVersion,
		RPCVersion:    cfg.RPCVersion,
		Intent:        protocol.IntentShutdown,
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return fmt.Errorf("send shutdown request: %w", err)
	}

	var resp protocol.HandshakeResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return fmt.Errorf("receive shutdown response: %w", err)
	}
	if !resp.Compatible {
		return fmt.Errorf("daemon refused shutdown request: %s", resp.Message)
	}
	if !resp.ShuttingDown {
		return errors.New("daemon did not acknowledge shutdown")
	}
	return nil
}

func waitForDaemonExit(ctx context.Context, cfg config.Config, clientVersion string) error {
	deadline := time.Now().Add(cfg.SpawnTimeout)
	for time.Now().Before(deadline) {
		_, _, err := handshake(ctx, cfg, clientVersion)
		var unavailableErr *DaemonUnavailableError
		if errors.As(err, &unavailableErr) {
			return nil
		}
		time.Sleep(cfg.RetryInterval)
	}
	return fmt.Errorf("daemon did not stop within %s", cfg.SpawnTimeout)
}

func shouldUpgradeDaemon(clientVersion string, daemonVersion string) bool {
	if clientVersion == "" || daemonVersion == "" {
		return false
	}
	return clientVersion != daemonVersion
}

func signalDaemonProcess(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid daemon pid: %d", pid)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}

func handshake(ctx context.Context, cfg config.Config, clientVersion string) (protocol.HandshakeResponse, HandshakeMetrics, error) {
	dialer := &net.Dialer{Timeout: cfg.ConnectTimeout}
	connectStart := time.Now()
	conn, err := dialer.DialContext(ctx, "unix", cfg.SocketPath)
	metrics := HandshakeMetrics{ConnectDuration: time.Since(connectStart)}
	if err != nil {
		return protocol.HandshakeResponse{}, metrics, &DaemonUnavailableError{Cause: err}
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(cfg.ConnectTimeout))
	handshakeStart := time.Now()

	req := protocol.HandshakeRequest{
		ClientName:    "k11s",
		ClientVersion: clientVersion,
		RPCVersion:    cfg.RPCVersion,
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		metrics.HandshakeDuration = time.Since(handshakeStart)
		return protocol.HandshakeResponse{}, metrics, fmt.Errorf("send handshake request: %w", err)
	}

	var resp protocol.HandshakeResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		metrics.HandshakeDuration = time.Since(handshakeStart)
		return protocol.HandshakeResponse{}, metrics, fmt.Errorf("receive handshake response: %w", err)
	}
	metrics.HandshakeDuration = time.Since(handshakeStart)

	if !resp.Compatible {
		return protocol.HandshakeResponse{}, metrics, &IncompatibleRPCError{
			ClientRPC: cfg.RPCVersion,
			DaemonRPC: resp.RPCVersion,
			Message:   resp.Message,
		}
	}

	return resp, metrics, nil
}

func spawnDaemon(socketPath string) error {
	daemonPath, err := resolveDaemonPath()
	if err != nil {
		return err
	}

	cmd := exec.Command(daemonPath, "--socket", socketPath)
	cmd.Env = append(os.Environ(), "K11S_SOCKET="+socketPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", daemonPath, err)
	}
	return cmd.Process.Release()
}

func resolveDaemonPath() (string, error) {
	if overridePath := os.Getenv("K11SD_PATH"); overridePath != "" {
		return overridePath, nil
	}

	if currentExe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(currentExe), "k11sd")
		if isExecutable(candidate) {
			return candidate, nil
		}
	}

	if daemonPath, err := exec.LookPath("k11sd"); err == nil {
		return daemonPath, nil
	}

	return "", errors.New("k11sd not found; set K11SD_PATH or place k11sd in PATH")
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Mode()&0o111 != 0
}
