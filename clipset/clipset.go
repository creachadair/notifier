// Program clipset sends a clipboard set request to a noteserver.
//
// Usage:
//    echo "message" | clipset -server :8080
//
package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"net"
	"os"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/misctools/notifier"
)

var (
	serverAddr = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")
	allowEmpty = flag.Bool("empty", false, "Allow empty clip contents")
	doRead     = flag.Bool("read", false, "Read clipboard contents")
	doTee      = flag.Bool("tee", false, "Also copy input to stdout")

	clipSet = jrpc2.NewCaller("Clip.Set", (*notifier.ClipRequest)(nil),
		false).(func(*jrpc2.Client, *notifier.ClipRequest) (bool, error))
	clipGet = jrpc2.NewCaller("Clip.Get", nil, []byte(nil)).(func(*jrpc2.Client) ([]byte, error))
)

func main() {
	flag.Parse()
	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(conn, nil)
	defer cli.Close()

	if *doRead {
		data, err := clipGet(cli)
		if err != nil {
			log.Fatalf("Reading clipboard: %v", err)
		}
		os.Stdout.Write(data)
		return
	}

	var buf bytes.Buffer
	var w io.Writer = &buf
	if *doTee {
		w = io.MultiWriter(&buf, os.Stdout)
	}
	if _, err := io.Copy(w, os.Stdin); err != nil {
		log.Fatalf("Reading stdin: %v", err)
	}
	if _, err := clipSet(cli, &notifier.ClipRequest{
		Data:       buf.Bytes(),
		AllowEmpty: *allowEmpty,
	}); err != nil {
		log.Fatalf("Setting clipboard failed: %v", err)
	}
}
