package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/dzhanguzin/k11s/internal/buildinfo"
	"github.com/dzhanguzin/k11s/internal/client"
	"github.com/dzhanguzin/k11s/internal/config"
	"github.com/dzhanguzin/k11s/internal/perf"
	"github.com/dzhanguzin/k11s/internal/protocol"
)

func main() {
	log.SetFlags(0)
	processStart := time.Now()

	if len(os.Args) >= 3 && os.Args[1] == "debug" && os.Args[2] == "perf" {
		if err := runDebugPerf(processStart, os.Args[3:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return
			}
			log.Fatalf("debug perf failed: %v", err)
		}
		return
	}

	if err := runStartup(processStart, os.Args[1:], false); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatalf("startup failed: %v", err)
	}
}

type startupOptions struct {
	socketOverride      string
	kubeContextOverride string
	namespaceOverride   string
	resourceOverride    string
	filterOverride      string
	selectionOverride   string
}

type startupState struct {
	Config      config.Config
	Bootstrap   client.BootstrapResult
	Session     protocol.SessionState
	Recorder    *perf.Recorder
	ProcessTime time.Time
}

func runStartup(processStart time.Time, args []string, includePerfOutput bool) error {
	opts, jsonOnly, err := parseStartupFlags(args, includePerfOutput, os.Stderr)
	if err != nil {
		return err
	}

	state, err := bootstrapAndRestore(processStart, opts)
	if err != nil {
		return err
	}

	outputWriter := io.Writer(os.Stdout)
	if includePerfOutput && jsonOnly {
		outputWriter = io.Discard
	}

	firstPaintDuration := renderStartupOutput(outputWriter, state)
	state.Recorder.AddDuration("first_paint", firstPaintDuration)

	if includePerfOutput {
		report := state.Recorder.Snapshot()
		if !jsonOnly {
			fmt.Print(perf.FormatReport(report))
		}

		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal perf report: %w", err)
		}
		fmt.Println(string(raw))
	}

	return nil
}

func runDebugPerf(processStart time.Time, args []string) error {
	return runStartup(processStart, args, true)
}

func parseStartupFlags(args []string, includePerfFlags bool, stderr io.Writer) (startupOptions, bool, error) {
	fs := flag.NewFlagSet("k11s", flag.ContinueOnError)
	fs.SetOutput(stderr)

	socketOverride := fs.String("socket", "", "override daemon socket path")
	kubeContextOverride := fs.String("context", "", "override restored kube context and persist it")
	namespaceOverride := fs.String("namespace", "", "override restored namespace and persist it")
	resourceOverride := fs.String("resource", "", "override restored resource and persist it")
	filterOverride := fs.String("filter", "", "override restored filter and persist it")
	selectionOverride := fs.String("selection", "", "override restored selection and persist it")

	jsonOnly := false
	if includePerfFlags {
		jsonOnlyFlag := fs.Bool("json-only", false, "print JSON report only")
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return startupOptions{}, false, err
			}
			return startupOptions{}, false, fmt.Errorf("parse flags: %w", err)
		}
		jsonOnly = *jsonOnlyFlag
	} else {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return startupOptions{}, false, err
			}
			return startupOptions{}, false, fmt.Errorf("parse flags: %w", err)
		}
	}

	return startupOptions{
		socketOverride:      *socketOverride,
		kubeContextOverride: *kubeContextOverride,
		namespaceOverride:   *namespaceOverride,
		resourceOverride:    *resourceOverride,
		filterOverride:      *filterOverride,
		selectionOverride:   *selectionOverride,
	}, jsonOnly, nil
}

func bootstrapAndRestore(processStart time.Time, opts startupOptions) (startupState, error) {
	cfg, err := config.Load()
	if err != nil {
		return startupState{}, fmt.Errorf("load config: %w", err)
	}
	if opts.socketOverride != "" {
		cfg.SocketPath = opts.socketOverride
	}

	recorder := perf.NewRecorder(processStart)
	recorder.AddDuration("process_start", 0)

	result, err := client.Bootstrap(context.Background(), cfg, buildinfo.Version)
	if err != nil {
		return startupState{}, fmt.Errorf("bootstrap daemon: %w", err)
	}
	recorder.AddDuration("daemon_connect", result.Metrics.ConnectDuration)
	recorder.AddDuration("handshake", result.Metrics.HandshakeDuration)

	sessionLoadStart := time.Now()
	sessionState, err := client.GetSession(context.Background(), cfg, buildinfo.Version)
	recorder.AddDuration("session_load", time.Since(sessionLoadStart))
	if err != nil {
		return startupState{}, fmt.Errorf("load session: %w", err)
	}

	updated := applySessionOverrides(
		&sessionState,
		opts.kubeContextOverride,
		opts.namespaceOverride,
		opts.resourceOverride,
		opts.filterOverride,
		opts.selectionOverride,
	)
	if updated {
		if err := client.SaveSession(context.Background(), cfg, buildinfo.Version, sessionState); err != nil {
			return startupState{}, fmt.Errorf("save session: %w", err)
		}
	}

	return startupState{
		Config:      cfg,
		Bootstrap:   result,
		Session:     sessionState,
		Recorder:    recorder,
		ProcessTime: processStart,
	}, nil
}

func renderStartupOutput(out io.Writer, state startupState) time.Duration {
	startMode := "warm"
	if state.Bootstrap.Restarted {
		startMode = "upgrade"
	} else if state.Bootstrap.Spawned {
		startMode = "cold"
	}

	firstPaintStart := time.Now()
	fmt.Fprintln(out, buildinfo.Banner("k11s", state.Config.RPCVersion))
	fmt.Fprintf(out, "bootstrap: socket=%s\n", state.Config.SocketPath)
	fmt.Fprintf(
		out,
		"bootstrap: %s start complete; daemon_pid=%d daemon_version=%s rpc=%s\n",
		startMode,
		state.Bootstrap.Handshake.PID,
		state.Bootstrap.Handshake.DaemonVersion,
		state.Bootstrap.Handshake.RPCVersion,
	)
	fmt.Fprintf(
		out,
		"session: context=%q namespace=%q resource=%q filter=%q selection=%q\n",
		state.Session.KubeContext,
		state.Session.Namespace,
		state.Session.Resource,
		state.Session.Filter,
		state.Session.Selection,
	)
	fmt.Fprintln(out, "ui: placeholder ready (session restored)")
	return time.Since(firstPaintStart)
}

func applySessionOverrides(
	state *protocol.SessionState,
	kubeContext string,
	namespace string,
	resource string,
	filter string,
	selection string,
) bool {
	updated := false

	if kubeContext != "" {
		state.KubeContext = kubeContext
		updated = true
	}
	if namespace != "" {
		state.Namespace = namespace
		updated = true
	}
	if resource != "" {
		state.Resource = resource
		updated = true
	}
	if filter != "" {
		state.Filter = filter
		updated = true
	}
	if selection != "" {
		state.Selection = selection
		updated = true
	}

	return updated
}
