// Program usernote requests editing of a notes file.
//
// Usage:
//    usernote -server :8080 <tag>
//
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"

	"bitbucket.org/creachadair/notifier"
)

var (
	noteCategory = flag.String("c", "", "Category label (optional)")
	noteVersion  = flag.String("v", "", `Version to edit ("", "latest", "new", "2006-01-02")`)
	doList       = flag.Bool("list", false, "List matching notes")
	doPath       = flag.Bool("path", false, "Print the path of the specified note")
	doRead       = flag.Bool("read", false, "Read the specified note text")
	doCategories = flag.Bool("cats", false, "List known categories")
)

func init() { notifier.RegisterFlags() }

func main() {
	flag.Parse()
	if flag.NArg() != 1 && !*doList && !*doCategories {
		log.Fatalf("Usage: %s <tag>", filepath.Base(os.Args[0]))
	} else if *doList && *doCategories {
		log.Fatal("You may not specify both -list and -categories")
	}

	ctx, cli, err := notifier.Dial(context.Background())
	if err != nil {
		log.Fatalf("Dial: %v", err)
	}
	defer cli.Close()

	if *doList {
		var rsp []*notifier.Note
		var tag string
		if flag.NArg() != 0 {
			tag = flag.Arg(0)
		}
		if err := cli.CallResult(ctx, "Notes.List", &notifier.ListNotesRequest{
			Tag:      tag,
			Category: *noteCategory,
			Version:  *noteVersion,
		}, &rsp); err != nil {
			log.Fatalf("Error listing notes: %v", err)
		}
		sort.Slice(rsp, func(i, j int) bool {
			return notifier.NoteLess(rsp[i], rsp[j])
		})
		notifier.Columns(os.Stdout, func(w io.Writer) {
			for _, note := range rsp {
				fmt.Fprint(w, note.Tag+note.Suffix, "\t", note.Version, "\n")
			}
		})

	} else if *doCategories {
		var cats []*notifier.NoteCategory
		if err := cli.CallResult(ctx, "Notes.Categories", nil, &cats); err != nil {
			log.Fatalf("Error listing categories: %v", err)
		}
		notifier.Columns(os.Stdout, func(w io.Writer) {
			for _, cat := range cats {
				fmt.Fprintf(w, "%s\t%s\n", cat.Name, cat.Dir)
			}
		})

	} else if *doRead || *doPath {
		var note notifier.NoteWithText
		if err := cli.CallResult(ctx, "Notes.Read", &notifier.EditNotesRequest{
			Tag:        flag.Arg(0),
			Category:   *noteCategory,
			Version:    *noteVersion,
			Background: true,
		}, &note); err != nil {
			log.Fatalf("Error reading note: %v", err)
		}
		if *doPath {
			fmt.Println(note.Path)
		}
		if *doRead {
			fmt.Println(string(note.Text))
		}

	} else if _, err := cli.Call(ctx, "Notes.Edit", &notifier.EditNotesRequest{
		Tag:      flag.Arg(0),
		Category: *noteCategory,
		Version:  *noteVersion,
	}); err != nil {
		log.Fatalf("Error editing note: %v", err)
	}
}
