// Program noteserver implements a server for posting notifications.
//
// Usage:
//    noteserver -address :8080
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
	"os/exec"
	"strings"
	"sync"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/server"
	"bitbucket.org/creachadair/misctools/notifier"
)

var (
	serverAddr = flag.String("address", os.Getenv("NOTIFIER_ADDR"), "Server address")
	soundName  = flag.String("sound", "Glass", "Sound name to use for audible notifications")
	voiceName  = flag.String("voice", "Moira", "Voice name to use for voice notifications")
	debugLog   = flag.Bool("log", false, "Enable debug logging")

	lw io.Writer

	// E_NotFound is the code returned when a requested resource is not found.
	E_NotFound = jrpc2.RegisterCode(-29998, "resource not found")
)

func main() {
	flag.Parse()
	if *serverAddr == "" {
		log.Fatal("A non-empty --address is required")
	} else if *debugLog {
		lw = os.Stderr
	}

	lst, err := net.Listen("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Listen: %v", err)
	}
	if err := server.Loop(server.Listener(lst), jrpc2.ServiceMapper{
		"Notify": jrpc2.MapAssigner{
			"Post": jrpc2.NewMethod(handlePostNote),
			"Say":  jrpc2.NewMethod(handleSayNote),
		},
		"Clip": jrpc2.NewService(&clipper{
			saved: make(map[string][]byte),
		}),
		"User": jrpc2.MapAssigner{
			"Text": jrpc2.NewMethod(handleText),
		},
	}, &jrpc2.ServerOptions{LogWriter: lw}); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handlePostNote(ctx context.Context, req *notifier.PostRequest) (bool, error) {
	if req.Body == "" && req.Title == "" {
		return false, jrpc2.Errorf(jrpc2.E_InvalidParams, "missing notification body and title")
	}
	program := []string{
		fmt.Sprintf("display notification %q", req.Body),
		fmt.Sprintf("with title %q", req.Title),
	}
	if t := req.Subtitle; t != "" {
		program = append(program, fmt.Sprintf("subtitle %q", t))
	}
	if req.Audible {
		program = append(program, fmt.Sprintf("sound name %q", *soundName))
	}
	cmd := exec.CommandContext(ctx, "osascript")
	cmd.Stdin = strings.NewReader(strings.Join(program, " "))
	err := cmd.Run()
	return err == nil, err
}

func handleSayNote(ctx context.Context, req *notifier.SayRequest) (bool, error) {
	if req.Text == "" {
		return false, jrpc2.Errorf(jrpc2.E_InvalidParams, "empty text")
	}
	cmd := exec.CommandContext(ctx, "say", "-v", *voiceName)
	cmd.Stdin = strings.NewReader(req.Text)
	err := cmd.Run()
	return err == nil, err
}

func handleText(ctx context.Context, req *notifier.TextRequest) (string, error) {
	if req.Prompt == "" {
		return "", jrpc2.Errorf(jrpc2.E_InvalidParams, "missing prompt string")
	}

	// Ask osascript to send error text to stdout to simplify error plumbing.
	cmd := exec.Command("osascript", "-s", "ho")
	cmd.Stdin = strings.NewReader(fmt.Sprintf(`display dialog %q default answer %q hidden answer %v`,
		req.Prompt, req.Default, req.Hide))
	raw, err := cmd.Output()
	out := strings.TrimRight(string(raw), "\n")
	if err != nil {
		if strings.Contains(out, "User canceled") {
			return "", notifier.E_UserCancelled
		}
		return "", err
	}

	// Parse the result out of the text delivered to stdout.
	const needle = "text returned:"
	if i := strings.Index(out, needle); i >= 0 {
		return out[i+len(needle):], nil
	}
	return "", jrpc2.Errorf(jrpc2.E_InternalError, "missing user input")
}

type clipper struct {
	sync.Mutex
	saved map[string][]byte
}

func (c *clipper) Set(ctx context.Context, req *notifier.ClipSetRequest) (bool, error) {
	if len(req.Data) == 0 && !req.AllowEmpty {
		return false, jrpc2.Errorf(jrpc2.E_InvalidParams, "empty clip data")
	} else if req.Tag != "" && req.Save == req.Tag {
		return false, jrpc2.Errorf(jrpc2.E_InvalidParams, "tag and save are equal")
	}

	// If we were requested to save the existing clip, extract its data.
	var saved []byte
	if req.Save != "" {
		data, err := getClip(ctx)
		if err != nil {
			return false, err
		}
		saved = data
	}

	if err := setClip(ctx, req.Data); err != nil {
		return false, err
	}

	// If a tag was provided, save the new clip under that tag.
	// If a save tag was provided, save the existing clip under that tag.
	c.Lock()
	if req.Tag != "" {
		if len(req.Data) == 0 {
			delete(c.saved, req.Tag)
		} else {
			c.saved[req.Tag] = req.Data
		}
	}
	if req.Save != "" {
		c.saved[req.Save] = saved
	}
	c.Unlock()
	return true, nil
}

func (c *clipper) Get(ctx context.Context, req *notifier.ClipGetRequest) ([]byte, error) {
	if req.Tag != "" {
		c.Lock()
		data, ok := c.saved[req.Tag]
		c.Unlock()
		if !ok {
			return nil, jrpc2.Errorf(E_NotFound, "tag %q not found", req.Tag)
		} else if req.Activate {
			if err := setClip(ctx, data); err != nil {
				return nil, err
			}
		}
		return data, nil
	}
	return getClip(ctx)
}

func setClip(ctx context.Context, data []byte) error {
	cmd := exec.CommandContext(ctx, "pbcopy")
	cmd.Stdin = bytes.NewReader(data)
	return cmd.Run()
}

func getClip(ctx context.Context) ([]byte, error) {
	return exec.CommandContext(ctx, "pbpaste").Output()
}
