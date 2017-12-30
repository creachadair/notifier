// Program clipset sends a clipboard set request to a noteserver.
//
// Usage:
//    echo "message" | clipset -server :8080
//
package main

import (
	"bytes"
	"context"
	"flag"
	"io"
	"log"
	"net"
	"os"

	"bitbucket.org/creachadair/cmdutil/files"
	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/misctools/notifier"
)

var (
	serverAddr = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")
	clipTag    = flag.String("tag", "", "Clipboard tag")
	saveTag    = flag.String("save", "", "Save tag")
	allowEmpty = flag.Bool("empty", false, "Allow empty clip contents")
	doActivate = flag.Bool("a", false, "Activate selected clip")
	doRead     = flag.Bool("read", false, "Read clipboard contents")
	doTee      = flag.Bool("tee", false, "Also copy input to stdout")

	clipSet = jrpc2.NewCaller("Clip.Set", (*notifier.ClipSetRequest)(nil),
		false).(func(*jrpc2.Client, *notifier.ClipSetRequest) (bool, error))
	clipGet = jrpc2.NewCaller("Clip.Get", (*notifier.ClipGetRequest)(nil),
		[]byte(nil)).(func(*jrpc2.Client, *notifier.ClipGetRequest) ([]byte, error))
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
		data, err := clipGet(cli, &notifier.ClipGetRequest{
			Tag:      *clipTag,
			Activate: *doActivate,
		})
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
	in := files.CatOrFile(context.Background(), flag.Args(), os.Stdin)
	if _, err := io.Copy(w, in); err != nil {
		log.Fatalf("Reading stdin: %v", err)
	}
	if _, err := clipSet(cli, &notifier.ClipSetRequest{
		Data:       buf.Bytes(),
		Tag:        *clipTag,
		Save:       *saveTag,
		AllowEmpty: *allowEmpty,
	}); err != nil {
		log.Fatalf("Setting clipboard failed: %v", err)
	}
}
