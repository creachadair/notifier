// Program useredit requests editing of a file.
//
// Usage:
//    useredit -server :8080 filename
//
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/channel"
	"bitbucket.org/creachadair/notifier"
)

var serverAddr = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatalf("Usage: %s <filename>", filepath.Base(os.Args[0]))
	}
	path := flag.Arg(0)
	input, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Editing new file %q\n", path)
	} else if err != nil {
		log.Fatalf("Reading input: %v", err)
	}

	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(channel.RawJSON(conn, conn), nil)
	defer cli.Close()
	ctx := context.Background()

	var output []byte
	if err := cli.CallResult(ctx, "User.Edit", &notifier.EditRequest{
		Name:    filepath.Base(path),
		Content: input,
	}, &output); err != nil {
		log.Fatalf("Error editing: %v", err)
	} else if bytes.Equal(input, output) {
		fmt.Fprintln(os.Stderr, "(unchanged)")
	} else if err := ioutil.WriteFile(path, output, 0644); err != nil {
		log.Fatalf("Writing output: %v", err)
	}
}
