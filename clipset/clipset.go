// Program clipset sends a clipboard set request to a noteserver.
//
// Usage:
//    echo "message" | clipset -server :8080
//
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sort"
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
	loadClips  = flag.String("load", "", "Load clip tags from JSON")
	allowEmpty = flag.Bool("empty", false, "Allow empty clip contents")
	doActivate = flag.Bool("a", false, "Activate selected clip")
	doClear    = flag.Bool("clear", false, "Clear clipboard contents")
	doDump     = flag.Bool("dump", false, "Dump all clips as JSON")
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

func count(bs ...bool) (n int) {
	for _, b := range bs {
		if b {
			n++
		}
	}
	return
}

func main() {
	flag.Parse()
	if count(*doList, (*doRead || *doClear), *loadClips != "", *doDump) > 1 {
		log.Fatal("The -list, -load, -dump, and -read/-clear flags are mutually exlusive")
	}
	if *doRead || *doClear || *loadClips != "" || *doDump {
		if (*doRead || *doClear) && *clipTag == "" && flag.NArg() == 1 {
			*clipTag = flag.Arg(0)
		} else if flag.NArg() != 0 {
			log.Fatal("You may not specify arguments with -read, -clear, -load, or -dump")
		}
	}
	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(channel.JSON(conn, conn), nil)
	defer cli.Close()
	ctx := context.Background()

	if *doList || *doDump {
		tags, err := clipList(ctx, cli)
		if err != nil {
			log.Fatalf("Listing tags: %v", err)
		}

		// Print a listing of the tag names, with -list.
		if *doList {
			if len(tags) > 0 {
				fmt.Println(strings.Join(tags, "\n"))
			}
			return
		}

		// Dump a JSON listing of the tag contents, with -dump.
		m := make(map[string][]byte)
		for _, tag := range tags {
			v, err := clipGet(ctx, cli, &notifier.ClipGetRequest{
				Tag: tag,
			})
			if err != nil {
				log.Fatalf("Reading tag %q: %v", tag, err)
			}
			m[tag] = v
		}
		out, err := json.Marshal(m)
		if err != nil {
			log.Fatalf("Encoding tag dump: %v", err)
		}
		fmt.Println(string(out))
		return
	}

	// Handle loading clips from a JSON dump.
	if *loadClips != "" {
		saved, err := ioutil.ReadFile(*loadClips)
		if err != nil {
			log.Fatalf("Opening tag dump: %v", err)
		}

		var m map[string][]byte
		if err := json.Unmarshal(saved, &m); err != nil {
			log.Fatalf("Decoding tag dump: %v", err)
		}
		for _, tag := range loadOrder(m) {
			data := m[tag]
			if _, err := clipSet(ctx, cli, &notifier.ClipSetRequest{
				Data:       data,
				Tag:        tag,
				AllowEmpty: true,
			}); err != nil {
				log.Fatalf("Setting tag %q: %v", tag, err)
			}
			fmt.Fprintf(os.Stderr, "Set clip %q (%d bytes)\n", tag, len(data))
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

// loadOrder returns the keys of m in sorted order, save that the special key
// "active" is always ordered last to ensure the active clip is set last, if it
// is defined at all.
func loadOrder(m map[string][]byte) []string {
	var keys []string
	for key := range m {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[j] == "active" || keys[i] < keys[j]
	})
	return keys
}
