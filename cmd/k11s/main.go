package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/dzhanguzin/k11s/internal/buildinfo"
	"github.com/dzhanguzin/k11s/internal/client"
	"github.com/dzhanguzin/k11s/internal/config"
	"github.com/dzhanguzin/k11s/internal/protocol"
)

func main() {
	log.SetFlags(0)

	socketOverride := flag.String("socket", "", "override daemon socket path")
	kubeContextOverride := flag.String("context", "", "override restored kube context and persist it")
	namespaceOverride := flag.String("namespace", "", "override restored namespace and persist it")
	resourceOverride := flag.String("resource", "", "override restored resource and persist it")
	filterOverride := flag.String("filter", "", "override restored filter and persist it")
	selectionOverride := flag.String("selection", "", "override restored selection and persist it")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if *socketOverride != "" {
		cfg.SocketPath = *socketOverride
	}

	fmt.Println(buildinfo.Banner("k11s", cfg.RPCVersion))
	fmt.Printf("bootstrap: socket=%s\n", cfg.SocketPath)

	result, err := client.Bootstrap(context.Background(), cfg, buildinfo.Version)
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}

	startMode := "warm"
	if result.Restarted {
		startMode = "upgrade"
	} else if result.Spawned {
		startMode = "cold"
	}

	fmt.Printf(
		"bootstrap: %s start complete; daemon_pid=%d daemon_version=%s rpc=%s\n",
		startMode,
		result.Handshake.PID,
		result.Handshake.DaemonVersion,
		result.Handshake.RPCVersion,
	)

	sessionState, err := client.GetSession(context.Background(), cfg, buildinfo.Version)
	if err != nil {
		log.Fatalf("load session: %v", err)
	}

	updated := applySessionOverrides(
		&sessionState,
		*kubeContextOverride,
		*namespaceOverride,
		*resourceOverride,
		*filterOverride,
		*selectionOverride,
	)
	if updated {
		if err := client.SaveSession(context.Background(), cfg, buildinfo.Version, sessionState); err != nil {
			log.Fatalf("save session: %v", err)
		}
	}

	fmt.Printf(
		"session: context=%q namespace=%q resource=%q filter=%q selection=%q\n",
		sessionState.KubeContext,
		sessionState.Namespace,
		sessionState.Resource,
		sessionState.Filter,
		sessionState.Selection,
	)
	fmt.Println("ui: placeholder ready (session restored)")
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
