// Program voicenote sends a voice notification request to a noteserver.
//
// Usage:
//
//	voicenote -server :8080 This is the notification
package main

import (
	"context"
	"flag"
	"log"
	"strings"

	"github.com/creachadair/notifier"
)

var waitTime = flag.Duration("after", 0, "Wait this long before speaking")

func init() { notifier.RegisterFlags() }

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatal("You must provide a non-empty notification text")
	}

	ctx, cli, err := notifier.Dial(context.Background())
	if err != nil {
		log.Fatalf("Dial: %v", err)
	}
	defer cli.Close()

	if err := cli.Notify(ctx, "Notify.Say", &notifier.SayRequest{
		Text:  strings.Join(flag.Args(), " "),
		After: *waitTime,
	}); err != nil {
		log.Fatalf("Sending notification failed: %v", err)
	}
}
