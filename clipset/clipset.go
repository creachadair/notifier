// Program clipset sends a clipboard set request to a noteserver.
//
// Usage:
//    echo "message" | clipset -server :8080
//
package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net"
	"os"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/misctools/notifier"
)

var (
	serverAddr = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")
	allowEmpty = flag.Bool("empty", false, "Allow empty clip contents")

	clipSet = jrpc2.NewCaller("Clip.Set", (*notifier.ClipRequest)(nil),
		false).(func(*jrpc2.Client, *notifier.ClipRequest) (bool, error))
)

func main() {
	flag.Parse()
	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(conn, nil)
	defer cli.Close()

	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("Reading stdin: %v", err)
	}

	if _, err := clipSet(cli, &notifier.ClipRequest{
		Data:       data,
		AllowEmpty: *allowEmpty,
	}); err != nil {
		log.Fatalf("Setting clipboard failed: %v", err)
	}
}
