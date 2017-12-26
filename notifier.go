// Program notifier implements a client and a server for posting notifications.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/server"
)

var (
	serverAddr   = flag.String("address", "", "Server address")
	doServe      = flag.Bool("serve", false, "Start a notification server")
	noteTitle    = flag.String("title", "", "Notification title")
	noteSubtitle = flag.String("subtitle", "", "Notification subtitle")
	noteBody     = flag.String("body", "", "Notification body")
	noteAudible  = flag.Bool("audible", false, "Whether notification should be audible")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %s -address <addr> [-serve | -body <text>]

When run with -serve set true, start a server to deliver notifications.
Otherwise, connect to the specified address and post a notification.
In the latter mode, a -body is also required.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	if *serverAddr == "" {
		log.Fatal("A non-empty --address is required")
	} else if *doServe {
		runServer(*serverAddr)
	} else if *noteBody == "" {
		log.Fatal("A notification --body is required")
	}

	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	defer conn.Close()

	cli := jrpc2.NewClient(conn, nil)
	if _, err := postNote(cli, &postReq{
		Title:    *noteTitle,
		Subtitle: *noteSubtitle,
		Body:     *noteBody,
		Audible:  *noteAudible,
	}); err != nil {
		log.Fatalf("Posting notification failed: %v", err)
	}
}

const notifyPost = "Notify.Post"

func runServer(addr string) {
	lst, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Listen: %v", err)
	}
	if err := server.Loop(server.Listener(lst), jrpc2.MapAssigner{
		notifyPost: jrpc2.NewMethod(handlePostNote),
	}, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
	os.Exit(0)
}

type postReq struct {
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
	Body     string `json:"body"`
	Audible  bool   `json:"audible,omitempty"`
}

func handlePostNote(ctx context.Context, req *postReq) (bool, error) {
	if req.Body == "" {
		return false, jrpc2.Errorf(jrpc2.E_InvalidParams, "missing notification body")
	}
	program := []string{fmt.Sprintf("display notification %q", req.Body)}
	if t := req.Title; t != "" {
		program = append(program, fmt.Sprintf("with title %q", t))
	}
	if t := req.Subtitle; t != "" {
		program = append(program, fmt.Sprintf("subtitle %q", t))
	}
	if req.Audible {
		program = append(program, `sound name "Ping"`)
	}
	cmd := exec.Command("osascript")
	cmd.Stdin = strings.NewReader(strings.Join(program, " "))
	err := cmd.Run()
	return err == nil, err
}

var postNote = jrpc2.NewCaller(notifyPost,
	(*postReq)(nil), false).(func(*jrpc2.Client, *postReq) (bool, error))
