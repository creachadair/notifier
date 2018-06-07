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
	"bitbucket.org/creachadair/jrpc2/caller"
	"bitbucket.org/creachadair/jrpc2/channel"
	"bitbucket.org/creachadair/misctools/notifier"
)

var (
	serverAddr = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")

	userEdit = caller.New("User.Edit", caller.Options{
		Params: (*notifier.EditRequest)(nil),
		Result: []byte(nil),
	}).(func(context.Context, *jrpc2.Client, *notifier.EditRequest) ([]byte, error))
)

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
	cli := jrpc2.NewClient(channel.JSON(conn, conn), nil)
	defer cli.Close()
	ctx := context.Background()

	output, err := userEdit(ctx, cli, &notifier.EditRequest{
		Name:    filepath.Base(path),
		Content: input,
	})
	if err != nil {
		log.Fatalf("Error editing: %v", err)
	} else if bytes.Equal(input, output) {
		fmt.Fprintln(os.Stderr, "(unchanged)")
	} else if err := ioutil.WriteFile(path, output, 0644); err != nil {
		log.Fatalf("Writing output: %v", err)
	}
}
