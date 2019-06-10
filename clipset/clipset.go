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
	"os"
	"strings"

	"bitbucket.org/creachadair/fileinput"
	"bitbucket.org/creachadair/jrpc2"
	"github.com/creachadair/notifier"
	"golang.org/x/crypto/ssh/terminal"
)

var (
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
)

func clipSet(ctx context.Context, cli *jrpc2.Client, req *notifier.ClipSetRequest) (result bool, err error) {
	err = cli.CallResult(ctx, "Clip.Set", req, &result)
	return
}

func clipGet(ctx context.Context, cli *jrpc2.Client, req *notifier.ClipGetRequest) (result []byte, err error) {
	err = cli.CallResult(ctx, "Clip.Get", req, &result)
	return
}

func clipList(ctx context.Context, cli *jrpc2.Client) (result []string, err error) {
	err = cli.CallResult(ctx, "Clip.List", nil, &result)
	return
}

func clipClear(ctx context.Context, cli *jrpc2.Client, req *notifier.ClipClearRequest) (result bool, err error) {
	err = cli.CallResult(ctx, "Clip.Clear", req, &result)
	return
}

func init() { notifier.RegisterFlags() }

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
	ctx, cli, err := notifier.Dial(context.Background())
	if err != nil {
		log.Fatalf("Dial: %v", err)
	}
	defer cli.Close()

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
	in := fileinput.CatOrFile(context.Background(), flag.Args(), os.Stdin)
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

// loadOrder returns the keys of m, save that the special key "active" is
// always ordered last to ensure the active clip is set last, if it is defined
// at all.
func loadOrder(m map[string][]byte) []string {
	var keys []string
	active := false
	for key := range m {
		if key == "active" {
			active = true
			continue
		}
		keys = append(keys, key)
	}
	if active {
		return append(keys, "active")
	}
	return keys
}

func count(bs ...bool) (n int) {
	for _, b := range bs {
		if b {
			n++
		}
	}
	return
}
