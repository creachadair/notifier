// Program usertext requests text from the user.
//
// Usage:
//    usertext -server :8080
//
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/channel"
	"bitbucket.org/creachadair/misctools/notifier"
)

var (
	serverAddr  = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")
	defaultText = flag.String("default", "", "Default answer")
	hiddenText  = flag.Bool("hidden", false, "Request hidden text entry")
)

func main() {
	flag.Parse()
	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(channel.RawJSON(conn, conn), nil)
	defer cli.Close()
	ctx := context.Background()

	var text string
	if err := cli.CallResult(ctx, "User.Text", &notifier.TextRequest{
		Prompt:  strings.Join(flag.Args(), " "),
		Default: *defaultText,
		Hide:    *hiddenText,
	}, &text); err == nil {
		fmt.Println(text)
	} else if e, ok := err.(*jrpc2.Error); ok && e.Code() == notifier.UserCancelled {
		os.Exit(2)
	} else {
		log.Fatal(err)
	}
}
