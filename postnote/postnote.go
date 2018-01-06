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
	"bitbucket.org/creachadair/jrpc2/channel"
	"bitbucket.org/creachadair/misctools/notifier"
)

var (
	serverAddr   = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")
	noteTitle    = flag.String("title", "", "Notification title")
	noteSubtitle = flag.String("subtitle", "", "Notification subtitle")
	noteAudible  = flag.Bool("audible", false, "Whether notification should be audible")

	postNote = jrpc2.NewCaller("Notify.Post", (*notifier.PostRequest)(nil),
		false).(func(*jrpc2.Client, *notifier.PostRequest) (bool, error))
)

func main() {
	flag.Parse()
	var title, body string
	if *noteTitle != "" {
		title = *noteTitle
		body = strings.Join(flag.Args(), " ")
	} else if flag.NArg() == 0 {
		log.Fatal("A notification title or body is required")
	} else {
		title = strings.Join(flag.Args(), " ")
	}

	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(channel.NewRaw(conn), nil)
	defer cli.Close()

	if _, err := postNote(cli, &notifier.PostRequest{
		Title:    title,
		Subtitle: *noteSubtitle,
		Body:     body,
		Audible:  *noteAudible,
	}); err != nil {
		log.Fatalf("Posting notification failed: %v", err)
	}
}
