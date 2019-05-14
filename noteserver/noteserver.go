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

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/jctx"
	"bitbucket.org/creachadair/jrpc2/metrics"
	"bitbucket.org/creachadair/jrpc2/server"
	"bitbucket.org/creachadair/notifier"

	// Install service plugins.
	_ "bitbucket.org/creachadair/notifier/noteserver/clipper"
	_ "bitbucket.org/creachadair/notifier/noteserver/keygen"
	_ "bitbucket.org/creachadair/notifier/noteserver/notes"
	_ "bitbucket.org/creachadair/notifier/noteserver/poster"
	_ "bitbucket.org/creachadair/notifier/noteserver/user"
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
		os.Remove(cfg.Address) // unlink a stale socket
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
		},
	}); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
