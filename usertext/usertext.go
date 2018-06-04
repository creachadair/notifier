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
	"bitbucket.org/creachadair/jrpc2/caller"
	"bitbucket.org/creachadair/jrpc2/channel"
	"bitbucket.org/creachadair/misctools/notifier"
)

var (
	serverAddr  = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")
	defaultText = flag.String("default", "", "Default answer")
	hiddenText  = flag.Bool("hidden", false, "Request hidden text entry")

	userText = caller.New("User.Text", caller.Options{
		Params: (*notifier.TextRequest)(nil),
		Result: "",
	}).(func(context.Context, *jrpc2.Client, *notifier.TextRequest) (string, error))
)

func main() {
	flag.Parse()
	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(channel.JSON(conn, conn), nil)
	defer cli.Close()
	ctx := context.Background()

	text, err := userText(ctx, cli, &notifier.TextRequest{
		Prompt:  strings.Join(flag.Args(), " "),
		Default: *defaultText,
		Hide:    *hiddenText,
	})
	if err == nil {
		fmt.Println(text)
	} else if e, ok := err.(*jrpc2.Error); ok && e.Code == notifier.E_UserCancelled {
		os.Exit(2)
	} else {
		log.Fatal(err)
	}
}
