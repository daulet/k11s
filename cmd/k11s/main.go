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

	"github.com/daulet/k11s/internal/buildinfo"
	"github.com/daulet/k11s/internal/client"
	"github.com/daulet/k11s/internal/config"
	"github.com/daulet/k11s/internal/kubeconfig"
	"github.com/daulet/k11s/internal/perf"
	"github.com/daulet/k11s/internal/protocol"
	"github.com/daulet/k11s/internal/ui"
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
	listFilterOverride  string
	selectionOverride   string
	simulateStale       bool
}

type startupState struct {
	Config               config.Config
	Bootstrap            client.BootstrapResult
	Session              protocol.SessionState
	ContextSuggestions   []string
	NamespaceSuggestions []string
	CRDSuggestions       []string
	ResourceList         protocol.ResourceListPayload
	SimulateStale        bool
	Recorder             *perf.Recorder
	ProcessTime          time.Time
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
	startMode := startupMode(state.Bootstrap)

	if includePerfOutput {
		outputWriter := io.Writer(os.Stdout)
		if jsonOnly {
			outputWriter = io.Discard
		}

		firstPaintDuration := renderStartupOutput(outputWriter, state, startMode)
		state.Recorder.AddDuration("first_paint", firstPaintDuration)

		report := state.Recorder.Snapshot()
		if !jsonOnly {
			fmt.Print(perf.FormatReport(report))
		}

		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal perf report: %w", err)
		}
		fmt.Println(string(raw))
		return nil
	}

	return runTUI(state, startMode)
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
	filterOverride := fs.String("filter", "", "override restored CRD filter and persist it")
	listFilterOverride := fs.String("list-filter", "", "override restored list filter and persist it")
	selectionOverride := fs.String("selection", "", "override restored selection and persist it")
	simulateStale := fs.Bool("simulate-stale", false, "simulate stale view freshness for status bar validation")

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
		listFilterOverride:  *listFilterOverride,
		selectionOverride:   *selectionOverride,
		simulateStale:       *simulateStale,
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
		opts.listFilterOverride,
		opts.selectionOverride,
	)
	if updated {
		if err := client.SaveSession(context.Background(), cfg, buildinfo.Version, sessionState); err != nil {
			return startupState{}, fmt.Errorf("save session: %w", err)
		}
	}

	contextSuggestions, err := kubeconfig.LoadContextNames()
	if err != nil {
		log.Printf("warning: unable to load kubeconfig contexts for autocomplete: %v", err)
		contextSuggestions = nil
	}
	namespaceList, err := client.ListNamespaces(
		context.Background(),
		cfg,
		buildinfo.Version,
		sessionState.KubeContext,
	)
	if err != nil {
		log.Printf("warning: unable to load namespaces for autocomplete: %v", err)
		namespaceList = protocol.NamespaceListPayload{}
	}
	crdSuggestions, err := client.ListCRDNames(
		context.Background(),
		cfg,
		buildinfo.Version,
		sessionState.KubeContext,
	)
	if err != nil {
		log.Printf("warning: unable to load crds for autocomplete: %v", err)
		crdSuggestions = nil
	}

	listLoadStart := time.Now()
	resourceList, err := client.ListResources(
		context.Background(),
		cfg,
		buildinfo.Version,
		protocol.ResourceListQuery{
			KubeContext:   sessionState.KubeContext,
			Resource:      sessionState.Resource,
			Namespace:     sessionState.Namespace,
			Filter:        sessionState.Filter,
			ListFilter:    sessionState.ListFilter,
			SimulateStale: opts.simulateStale,
		},
	)
	recorder.AddDuration("initial_list", time.Since(listLoadStart))
	if err != nil {
		return startupState{}, fmt.Errorf("load initial resource list: %w", err)
	}

	return startupState{
		Config:               cfg,
		Bootstrap:            result,
		Session:              sessionState,
		ContextSuggestions:   contextSuggestions,
		NamespaceSuggestions: namespaceList.Namespaces,
		CRDSuggestions:       crdSuggestions,
		ResourceList:         resourceList,
		SimulateStale:        opts.simulateStale,
		Recorder:             recorder,
		ProcessTime:          processStart,
	}, nil
}

func renderStartupOutput(out io.Writer, state startupState, startMode string) time.Duration {
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
		"session: context=%q namespace=%q resource=%q filter=%q list_filter=%q selection=%q\n",
		state.Session.KubeContext,
		state.Session.Namespace,
		state.Session.Resource,
		state.Session.Filter,
		state.Session.ListFilter,
		state.Session.Selection,
	)
	fmt.Fprintf(
		out,
		"status: %s\n",
		formatStatusBar(state.ResourceList.Freshness, enableColor()),
	)
	fmt.Fprintf(
		out,
		"view: resource=%q namespace=%q items=%d\n",
		state.ResourceList.Resource,
		state.ResourceList.Namespace,
		len(state.ResourceList.Items),
	)
	fmt.Fprintln(out, "ui: ready (session restored)")
	return time.Since(firstPaintStart)
}

func startupMode(result client.BootstrapResult) string {
	startMode := "warm"
	if result.Restarted {
		startMode = "upgrade"
	} else if result.Spawned {
		startMode = "cold"
	}
	return startMode
}

func runTUI(state startupState, startMode string) error {
	result, err := ui.Run(ui.Options{
		Session:              state.Session,
		ResourceList:         state.ResourceList,
		ContextSuggestions:   state.ContextSuggestions,
		NamespaceSuggestions: state.NamespaceSuggestions,
		CRDSuggestions:       state.CRDSuggestions,
		UseColor:             enableColor(),
		SimulateStale:        state.SimulateStale,
		LoadResourceList: func(ctx context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			return client.ListResources(ctx, state.Config, buildinfo.Version, query)
		},
		LoadResourceDetail: func(ctx context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			return client.GetResourceDetail(ctx, state.Config, buildinfo.Version, query)
		},
		LoadPodView: func(ctx context.Context, query protocol.PodViewQuery) (protocol.PodViewPayload, error) {
			return client.GetPodView(ctx, state.Config, buildinfo.Version, query)
		},
		LoadNamespaces: func(ctx context.Context, kubeContext string) (protocol.NamespaceListPayload, error) {
			return client.ListNamespaces(ctx, state.Config, buildinfo.Version, kubeContext)
		},
		LoadCRDs: func(ctx context.Context, kubeContext string) ([]string, error) {
			return client.ListCRDNames(ctx, state.Config, buildinfo.Version, kubeContext)
		},
		LoadAction: func(ctx context.Context, query protocol.ActionQuery) (protocol.ActionResult, error) {
			return client.ExecuteAction(ctx, state.Config, buildinfo.Version, query)
		},
		LoadLogs: func(ctx context.Context, query protocol.LogsQuery) (protocol.LogsPayload, error) {
			return client.GetLogs(ctx, state.Config, buildinfo.Version, query)
		},
	})
	if err != nil {
		return fmt.Errorf("run tui (%s): %w", startMode, err)
	}

	if result.Session != state.Session {
		if err := client.SaveSession(context.Background(), state.Config, buildinfo.Version, result.Session); err != nil {
			return fmt.Errorf("persist session after tui exit: %w", err)
		}
	}

	return nil
}

func enableColor() bool {
	return os.Getenv("NO_COLOR") == ""
}

func formatStatusBar(meta protocol.FreshnessMeta, useColor bool) string {
	stateLabel := string(meta.State)
	badge := "[" + stateLabel + "]"

	switch meta.State {
	case protocol.FreshnessStateLive:
		if useColor {
			badge = "\x1b[30;42m LIVE \x1b[0m"
		}
	case protocol.FreshnessStateCatchingUp:
		if useColor {
			badge = "\x1b[30;43m CATCHING_UP \x1b[0m"
		} else {
			badge = "[CATCHING_UP]"
		}
	case protocol.FreshnessStateStale:
		if useColor {
			badge = "\x1b[97;41m !!! STALE !!! \x1b[0m"
		} else {
			badge = "[!!! STALE !!!]"
		}
	default:
		if useColor {
			badge = "\x1b[37;45m UNKNOWN \x1b[0m"
		} else {
			badge = "[UNKNOWN]"
		}
	}

	text := fmt.Sprintf(
		"%s age=%s as_of=%s source=%s watch=%s",
		badge,
		formatAge(meta.AgeMs),
		formatSnapshot(meta.SnapshotTimeUnixMs),
		meta.Source,
		formatWatchHealth(meta.WatchHealthy),
	)
	if meta.Error != "" {
		return text + " err=" + meta.Error
	}
	return text
}

func formatAge(ageMs int64) string {
	if ageMs < 1000 {
		return fmt.Sprintf("%dms", ageMs)
	}
	if ageMs < 60_000 {
		return fmt.Sprintf("%ds", ageMs/1000)
	}
	return fmt.Sprintf("%dm%ds", ageMs/60_000, (ageMs%60_000)/1000)
}

func formatSnapshot(snapshotUnixMs int64) string {
	if snapshotUnixMs <= 0 {
		return "unknown"
	}
	return time.UnixMilli(snapshotUnixMs).UTC().Format(time.RFC3339)
}

func formatWatchHealth(healthy bool) string {
	if healthy {
		return "healthy"
	}
	return "degraded"
}

func applySessionOverrides(
	state *protocol.SessionState,
	kubeContext string,
	namespace string,
	resource string,
	filter string,
	listFilter string,
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
	if listFilter != "" {
		state.ListFilter = listFilter
		updated = true
	}
	if selection != "" {
		state.Selection = selection
		updated = true
	}

	return updated
}
