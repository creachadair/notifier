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
	"time"

	"bitbucket.org/creachadair/jrpc2"
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

	configPath = flag.String("config", "", "Configuration file path (overrides other flags)")
)

func init() {
	flag.StringVar(&cfg.Address, "address", os.Getenv("NOTIFIER_ADDR"), "Server address")
	flag.StringVar(&cfg.Edit.Command, "editor", os.Getenv("EDITOR"), "Editor command line")
	flag.StringVar(&cfg.Note.Sound, "sound", "Glass", "Sound name to use for audible notifications")
	flag.StringVar(&cfg.Note.Voice, "voice", "Moira", "Voice name to use for voice notifications")
	flag.BoolVar(&cfg.DebugLog, "log", false, "Enable debug logging")
}

func main() {
	flag.Parse()
	if err := notifier.LoadConfig(*configPath, &cfg); err != nil {
		log.Fatalf("Loading configuration: %v", err)
	}
	if cfg.Address == "" {
		log.Fatal("A non-empty --address is required")
	} else if cfg.DebugLog {
		lw = log.New(os.Stderr, "[noteserver] ", log.LstdFlags)
	}

	lst, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		log.Fatalf("Listen: %v", err)
	}
	if err := server.Loop(lst, notifier.PluginAssigner(&cfg), &server.LoopOptions{
		ServerOptions: &jrpc2.ServerOptions{
			Logger:    lw,
			Metrics:   metrics.New(),
			StartTime: time.Now().In(time.UTC),
		},
	}); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
