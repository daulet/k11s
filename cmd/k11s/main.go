package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/dzhanguzin/k11s/internal/buildinfo"
	"github.com/dzhanguzin/k11s/internal/client"
	"github.com/dzhanguzin/k11s/internal/config"
)

func main() {
	log.SetFlags(0)

	socketOverride := flag.String("socket", "", "override daemon socket path")
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
	if result.Spawned {
		startMode = "cold"
	}

	fmt.Printf(
		"bootstrap: %s start complete; daemon_pid=%d daemon_version=%s rpc=%s\n",
		startMode,
		result.Handshake.PID,
		result.Handshake.DaemonVersion,
		result.Handshake.RPCVersion,
	)
	fmt.Println("ui: placeholder ready (next: session restore + first paint)")
}
