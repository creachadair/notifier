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
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"bitbucket.org/creachadair/cmdutil/files"
	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/caller"
	"bitbucket.org/creachadair/jrpc2/channel"
	"bitbucket.org/creachadair/misctools/notifier"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	serverAddr = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")
	clipTag    = flag.String("tag", "", "Clipboard tag")
	saveTag    = flag.String("save", "", "Save tag")
	allowEmpty = flag.Bool("empty", false, "Allow empty clip contents")
	doActivate = flag.Bool("a", false, "Activate selected clip")
	doClear    = flag.Bool("clear", false, "Clear clipboard contents")
	doRead     = flag.Bool("read", false, "Read clipboard contents")
	doList     = flag.Bool("list", false, "List clipboard tags")
	doTee      = flag.Bool("tee", false, "Also copy input to stdout")

	clipSet = caller.New("Clip.Set", caller.Options{
		Params: (*notifier.ClipSetRequest)(nil),
		Result: false,
	}).(func(context.Context, *jrpc2.Client, *notifier.ClipSetRequest) (bool, error))
	clipGet = caller.New("Clip.Get", caller.Options{
		Params: (*notifier.ClipGetRequest)(nil),
		Result: []byte(nil),
	}).(func(context.Context, *jrpc2.Client, *notifier.ClipGetRequest) ([]byte, error))
	clipList = caller.New("Clip.List", caller.Options{
		Params: nil,
		Result: []string(nil),
	}).(func(context.Context, *jrpc2.Client) ([]string, error))
	clipClear = caller.New("Clip.Clear", caller.Options{
		Params: (*notifier.ClipClearRequest)(nil),
		Result: false,
	}).(func(context.Context, *jrpc2.Client, *notifier.ClipClearRequest) (bool, error))
)

func main() {
	flag.Parse()
	if *doList && (*doRead || *doClear) {
		log.Fatal("You may not combine -list with -read or -clear")
	}
	if *doRead || *doClear {
		if *clipTag == "" && flag.NArg() == 1 {
			*clipTag = flag.Arg(0)
		} else if flag.NArg() != 0 {
			log.Fatal("You may not specify arguments when -read or -clear is set")
		}
	}
	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(channel.JSON(conn, conn), nil)
	defer cli.Close()
	ctx := context.Background()

	if *doList {
		tags, err := clipList(ctx, cli)
		if err != nil {
			log.Fatalf("Listing tags: %v", err)
		} else if len(tags) != 0 {
			fmt.Println(strings.Join(tags, "\n"))
		}
		return
	}

	// Read falls through to clear, so we can handle both.
	if *doRead {
		data, err := clipGet(ctx, cli, &notifier.ClipGetRequest{
			Tag:      *clipTag,
			Save:     *saveTag,
			Activate: *doActivate,
		})
		if err != nil {
			log.Fatalf("Reading clipboard: %v", err)
		}
		os.Stdout.Write(data)

		// When printing to a terminal, ensure the output ends with a newline.
		if terminal.IsTerminal(int(os.Stdout.Fd())) && len(data) != 0 && !bytes.HasSuffix(data, []byte("\n")) {
			os.Stdout.Write([]byte("\n"))
		}
	}
	if *doClear {
		if _, err := clipClear(ctx, cli, &notifier.ClipClearRequest{
			Tag: *clipTag,
		}); err != nil {
			log.Fatalf("Clearing clipboard: %v", err)
		}
	}
	if *doRead || *doClear {
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
	if _, err := clipSet(ctx, cli, &notifier.ClipSetRequest{
		Data:       buf.Bytes(),
		Tag:        *clipTag,
		Save:       *saveTag,
		AllowEmpty: *allowEmpty,
	}); err != nil {
		log.Fatalf("Setting clipboard failed: %v", err)
	}
}
