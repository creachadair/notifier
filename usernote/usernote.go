// Program usernote requests editing of a notes file.
//
// Usage:
//    usernotes -server :8080 <tag>
//
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/channel"
	"bitbucket.org/creachadair/misctools/notifier"
)

var (
	serverAddr   = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")
	noteCategory = flag.String("c", "", "Category label (optional)")
	noteVersion  = flag.String("v", "", `Version to edit ("", "latest", "2006-01-02")`)
	doList       = flag.Bool("list", false, "List matching notes")
	doCategories = flag.Bool("cats", false, "List known categories")
)

func main() {
	flag.Parse()
	if flag.NArg() != 1 && !*doList {
		log.Fatalf("Usage: %s <tag>", filepath.Base(os.Args[0]))
	} else if *doList && *doCategories {
		log.Fatal("You may not specify both -list and -categories")
	}

	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(channel.RawJSON(conn, conn), nil)
	defer cli.Close()
	ctx := context.Background()

	if *doList {
		var rsp []*notifier.Note
		var tag string
		if flag.NArg() != 0 {
			tag = flag.Arg(0)
		}
		if err := cli.CallResult(ctx, "Notes.List", &notifier.ListNotesRequest{
			Tag:      tag,
			Category: *noteCategory,
		}, &rsp); err != nil {
			log.Fatalf("Error listing notes: %v", err)
		}
		sort.Slice(rsp, func(i, j int) bool {
			return notifier.NoteLess(rsp[i], rsp[j])
		})
		tw := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)
		for _, note := range rsp {
			fmt.Fprint(tw, note.Tag, "\t", note.Version, "\n")
		}
		tw.Flush()

	} else if *doCategories {
		var cats []string
		if err := cli.CallResult(ctx, "Notes.Categories", nil, &cats); err != nil {
			log.Fatalf("Error listing categories: %v", err)
		}
		fmt.Println(strings.Join(cats, "\n"))

	} else if err := cli.Notify(ctx, "Notes.Edit", &notifier.EditNotesRequest{
		Tag:      flag.Arg(0),
		Category: *noteCategory,
		Version:  *noteVersion,
	}); err != nil {
		log.Fatalf("Error editing note: %v", err)
	}
}