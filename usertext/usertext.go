// Program usertext requests text from the user.
//
// Usage:
//
//	usertext -server :8080
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/notifier"
)

var (
	defaultText = flag.String("default", "", "Default answer")
	hiddenText  = flag.Bool("hidden", false, "Request hidden text entry")
)

func init() { notifier.RegisterFlags() }

func main() {
	flag.Parse()
	ctx, cli, err := notifier.Dial(context.Background())
	if err != nil {
		log.Fatalf("Dial: %v", err)
	}
	defer cli.Close()

	var text string
	if err := cli.CallResult(ctx, "User.Text", &notifier.TextRequest{
		Prompt:  strings.Join(flag.Args(), " "),
		Default: *defaultText,
		Hide:    *hiddenText,
	}, &text); err == nil {
		fmt.Println(text)
	} else if code.FromError(err) == notifier.UserCancelled {
		os.Exit(2)
	} else {
		log.Fatal(err)
	}
}
