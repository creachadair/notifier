// Program usertext requests text from the user.
//
// Usage:
//    usertext -server :8080
//
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/misctools/notifier"
)

var (
	serverAddr  = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")
	promptText  = flag.String("prompt", "", "Prompt string (required)")
	defaultText = flag.String("default", "", "Default answer")
	hiddenText  = flag.Bool("hide", false, "Hide text entry")

	userText = jrpc2.NewCaller("User.Text", (*notifier.TextRequest)(nil), "").(func(*jrpc2.Client, *notifier.TextRequest) (string, error))
)

func main() {
	flag.Parse()
	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(conn, nil)
	defer cli.Close()

	text, err := userText(cli, &notifier.TextRequest{
		Prompt:  *promptText,
		Default: *defaultText,
		Hide:    *hiddenText,
	})
	if err == nil {
		fmt.Println(text)
	} else {
		log.Fatal(err)
	}
}
