// Program postnote sends a notification request to a noteserver.
//
// Usage:
//    postnote -server :8080 This is the notification
//
package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strings"

	"bitbucket.org/creachadair/jrpc2"
)

var (
	serverAddr   = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")
	noteTitle    = flag.String("title", "", "Notification title")
	noteSubtitle = flag.String("subtitle", "", "Notification subtitle")
	noteAudible  = flag.Bool("audible", false, "Whether notification should be audible")

	postNote = jrpc2.NewCaller("Notify.Post",
		(*postReq)(nil), false).(func(*jrpc2.Client, *postReq) (bool, error))
)

func main() {
	flag.Parse()
	if *noteTitle == "" && flag.NArg() == 0 {
		log.Fatal("A notification --title or body is required")
	}

	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(conn, nil)
	defer cli.Close()

	if _, err := postNote(cli, &postReq{
		Title:    *noteTitle,
		Subtitle: *noteSubtitle,
		Body:     strings.Join(flag.Args(), " "),
		Audible:  *noteAudible,
	}); err != nil {
		log.Fatalf("Posting notification failed: %v", err)
	}
}

type postReq struct {
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
	Body     string `json:"body"`
	Audible  bool   `json:"audible,omitempty"`
}
