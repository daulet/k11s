package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/daulet/k11s/internal/buildinfo"
	"github.com/daulet/k11s/internal/config"
	"github.com/daulet/k11s/internal/daemon"
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

	fmt.Println(buildinfo.Banner("k11sd", cfg.RPCVersion))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := daemon.Run(ctx, cfg, buildinfo.Version); err != nil {
		log.Fatalf("daemon exited with error: %v", err)
	}
}
