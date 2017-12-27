// Program noteserver implements a server for posting notifications.
//
// Usage:
//    noteserver -address :8080
//
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/server"
	"bitbucket.org/creachadair/misctools/notifier"
)

var (
	serverAddr = flag.String("address", os.Getenv("NOTIFIER_ADDR"), "Server address")
	soundName  = flag.String("sound", "Glass", "Sound name to use for audible notifications")
)

func main() {
	flag.Parse()
	if *serverAddr == "" {
		log.Fatal("A non-empty --address is required")
	}
	lst, err := net.Listen("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Listen: %v", err)
	}
	if err := server.Loop(server.Listener(lst), jrpc2.MapAssigner{
		"Notify.Post": jrpc2.NewMethod(handlePostNote),
		"Clip.Set":    jrpc2.NewMethod(handleClipSet),
	}, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handlePostNote(ctx context.Context, req *notifier.PostRequest) (bool, error) {
	if req.Body == "" && req.Title == "" {
		return false, jrpc2.Errorf(jrpc2.E_InvalidParams, "missing notification body and title")
	}
	program := []string{
		fmt.Sprintf("display notification %q", req.Body),
		fmt.Sprintf("with title %q", req.Title),
	}
	if t := req.Subtitle; t != "" {
		program = append(program, fmt.Sprintf("subtitle %q", t))
	}
	if req.Audible {
		program = append(program, fmt.Sprintf("sound name %q", *soundName))
	}
	cmd := exec.Command("osascript")
	cmd.Stdin = strings.NewReader(strings.Join(program, " "))
	err := cmd.Run()
	return err == nil, err
}

func handleClipSet(ctx context.Context, req *notifier.ClipRequest) (bool, error) {
	if len(req.Data) == 0 && !req.AllowEmpty {
		return false, jrpc2.Errorf(jrpc2.E_InvalidParams, "empty clip data")
	}
	cmd := exec.Command("pbcopy")
	cmd.Stdin = bytes.NewReader(req.Data)
	err := cmd.Run()
	return err == nil, err
}
