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
	"strings"

	"bitbucket.org/creachadair/notifier"
)

var (
	noteTitle    = flag.String("title", "", "Notification title")
	noteSubtitle = flag.String("subtitle", "", "Notification subtitle")
	noteAudible  = flag.Bool("audible", false, "Whether notification should be audible")
	waitTime     = flag.Duration("after", 0, "Wait this long before posting")
)

func init() { notifier.RegisterFlags() }

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

	ctx, cli, err := notifier.Dial(context.Background())
	if err != nil {
		log.Fatalf("Dial: %v", err)
	}
	defer cli.Close()

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
