// Program voicenote sends a voice notification request to a noteserver.
//
// Usage:
//    voicenote -server :8080 This is the notification
//
package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strings"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/misctools/notifier"
)

var serverAddr = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatal("You must provide a non-empty notification text")
	}

	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(conn, nil)
	defer cli.Close()

	if _, err := cli.CallWait("Notify.Say", &notifier.SayRequest{
		Text: strings.Join(flag.Args(), " "),
	}); err != nil {
		log.Fatalf("Sending notification failed: %v", err)
	}
}
