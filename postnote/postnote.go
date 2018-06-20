// Program postnote sends a notification request to a noteserver.
//
// Usage:
//    postnote -server :8080 This is the notification
//
package main

import (
	"context"
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
	waitTime     = flag.Duration("after", 0, "Wait this long before posting")
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
	cli := jrpc2.NewClient(channel.RawJSON(conn, conn), nil)
	defer cli.Close()
	ctx := context.Background()

	if err := cli.Notify(ctx, "Notify.Post", &notifier.PostRequest{
		Title:    title,
		Subtitle: *noteSubtitle,
		Body:     body,
		Audible:  *noteAudible,
		After:    *waitTime,
	}); err != nil {
		log.Fatalf("Posting notification failed: %v", err)
	}
}
