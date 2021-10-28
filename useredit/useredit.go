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
	"log"
	"os"
	"path/filepath"

	"github.com/creachadair/notifier"
)

func init() { notifier.RegisterFlags() }

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatalf("Usage: %s <filename>", filepath.Base(os.Args[0]))
	}
	path := flag.Arg(0)
	input, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Editing new file %q\n", path)
	} else if err != nil {
		log.Fatalf("Reading input: %v", err)
	}

	ctx, cli, err := notifier.Dial(context.Background())
	if err != nil {
		log.Fatalf("Dial: %v", err)
	}
	defer cli.Close()

	var output []byte
	if err := cli.CallResult(ctx, "User.Edit", &notifier.EditRequest{
		Name:    filepath.Base(path),
		Content: input,
	}, &output); err != nil {
		log.Fatalf("Error editing: %v", err)
	} else if bytes.Equal(input, output) {
		fmt.Fprintln(os.Stderr, "(unchanged)")
	} else if err := os.WriteFile(path, output, 0644); err != nil {
		log.Fatalf("Writing output: %v", err)
	}
}
