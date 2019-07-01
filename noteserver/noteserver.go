// Program noteserver implements a server for posting notifications.
//
// Usage:
//    noteserver -address :8080
//
package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/jctx"
	"github.com/creachadair/jrpc2/metrics"
	"github.com/creachadair/jrpc2/server"
	"github.com/creachadair/notifier"

	// Install service plugins.
	_ "github.com/creachadair/notifier/noteserver/clipper"
	_ "github.com/creachadair/notifier/noteserver/keygen"
	_ "github.com/creachadair/notifier/noteserver/notes"
	_ "github.com/creachadair/notifier/noteserver/poster"
	_ "github.com/creachadair/notifier/noteserver/user"
)

var (
	cfg notifier.Config
	lw  *log.Logger

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
		lw = log.New(os.Stderr, "[noteserver] ", log.LstdFlags)
	}

	atype := "tcp"
	if !strings.Contains(cfg.Address, ":") {
		atype = "unix"

		// Expand variables in a socket path, and unlink a stale socket in case
		// one was left behind by a previous run.
		cfg.Address = os.ExpandEnv(cfg.Address)
		_ = os.Remove(cfg.Address) // it's fine if this fails
	}
	lst, err := net.Listen(atype, cfg.Address)
	if err != nil {
		log.Fatalf("Listen: %v", err)
	}
	if err := server.Loop(lst, notifier.PluginAssigner(&cfg), &server.LoopOptions{
		ServerOptions: &jrpc2.ServerOptions{
			Logger:        lw,
			Metrics:       metrics.New(),
			StartTime:     time.Now().In(time.UTC),
			DecodeContext: jctx.Decode,
			CheckRequest:  cfg.CheckRequest,
		},
	}); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
