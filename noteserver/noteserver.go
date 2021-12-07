// Program noteserver implements a server for posting notifications.
//
// Usage:
//    noteserver -address :8080
//
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/jctx"
	"github.com/creachadair/jrpc2/metrics"
	"github.com/creachadair/jrpc2/server"
	"github.com/creachadair/notifier"

	// Install service plugins.
	_ "github.com/creachadair/notifier/noteserver/clipper"
	_ "github.com/creachadair/notifier/noteserver/keygen"
	_ "github.com/creachadair/notifier/noteserver/poster"
	_ "github.com/creachadair/notifier/noteserver/user"
)

var (
	cfg notifier.Config
	lw  jrpc2.Logger

	configPath = flag.String("config", "", "Configuration file path")
	serverAddr = flag.String("address", "", "Server address (overrides config)")
	debugLog   = flag.Bool("debuglog", false, "Enable debug logging (overrides config)")
)

func main() {
	flag.Parse()
	if *configPath == "" {
		log.Fatal("You must provide a non-empty -config file path")
	} else if err := notifier.LoadConfig(*configPath, &cfg); err != nil {
		log.Fatalf("Loading configuration: %v", err)
	}
	if *serverAddr != "" {
		cfg.Address = *serverAddr
	}
	if cfg.DebugLog || *debugLog {
		lw = jrpc2.StdLogger(log.New(os.Stderr, "[noteserver] ", log.LstdFlags))
	}

	atype, addr := jrpc2.Network(cfg.Address)
	if atype == "unix" {
		// Expand variables in a socket path, and unlink a stale socket in case
		// one was left behind by a previous run.
		addr = os.ExpandEnv(cfg.Address)
		_ = os.Remove(addr) // it's fine if this fails
	}
	lst, err := net.Listen(atype, addr)
	if err != nil {
		log.Fatalf("Listen: %v", err)
	}
	m := metrics.New()
	m.SetLabel("noteserver.pid", os.Getpid())

	ctx := context.Background()
	acc := server.NetAccepter(lst, channel.Line)
	service := server.Static(notifier.PluginAssigner(&cfg))
	if err := server.Loop(ctx, acc, service, &server.LoopOptions{
		ServerOptions: &jrpc2.ServerOptions{
			Logger:        lw,
			Metrics:       m,
			StartTime:     time.Now().In(time.UTC),
			DecodeContext: jctx.Decode,
		},
	}); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
