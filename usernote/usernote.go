// Program usernote requests editing of a notes file.
//
// Usage:
//    usernotes -server :8080 <tag>
//
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"path/filepath"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/channel"
	"bitbucket.org/creachadair/misctools/notifier"
)

var (
	serverAddr   = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")
	noteCategory = flag.String("c", "", "Category label (optional)")
	noteVersion  = flag.String("version", "", "Version to edit")
)

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatalf("Usage: %s <tag>", filepath.Base(os.Args[0]))
	}
	tag := flag.Arg(0)

	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(channel.RawJSON(conn, conn), nil)
	defer cli.Close()
	ctx := context.Background()

	if err := cli.Notify(ctx, "Notes.Edit", &notifier.EditNotesRequest{
		Tag:      tag,
		Category: *noteCategory,
		Version:  *noteVersion,
	}); err != nil {
		log.Fatalf("Error editing note: %v", err)
	}
}
