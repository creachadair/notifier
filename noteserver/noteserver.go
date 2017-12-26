// Program noteserver implements a server for posting notifications.
//
// Usage:
//    noteserver -address :8080
//
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/server"
)

var (
	serverAddr = flag.String("address", "", "Server address")
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
	}, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

type postReq struct {
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
	Body     string `json:"body"`
	Audible  bool   `json:"audible,omitempty"`
}

func handlePostNote(ctx context.Context, req *postReq) (bool, error) {
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
