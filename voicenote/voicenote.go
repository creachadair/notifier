// Program voicenote sends a voice notification request to a noteserver.
//
// Usage:
//    voicenote -server :8080 This is the notification
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
	serverAddr = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")
	waitTime   = flag.Duration("after", 0, "Wait this long before speaking")
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatal("You must provide a non-empty notification text")
	}

	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(channel.RawJSON(conn, conn), nil)
	defer cli.Close()
	ctx := context.Background()

	if err := cli.Notify(ctx, "Notify.Say", &notifier.SayRequest{
		Text:  strings.Join(flag.Args(), " "),
		After: *waitTime,
	}); err != nil {
		log.Fatalf("Sending notification failed: %v", err)
	}
}
