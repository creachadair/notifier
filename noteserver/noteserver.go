// Program noteserver implements a server for posting notifications.
//
// Usage:
//    noteserver -address :8080
//
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/code"
	"bitbucket.org/creachadair/jrpc2/metrics"
	"bitbucket.org/creachadair/jrpc2/server"
	"bitbucket.org/creachadair/keyfish/config"
	"bitbucket.org/creachadair/keyfish/wordhash"
	"bitbucket.org/creachadair/misctools/notifier"
	"bitbucket.org/creachadair/shell"
	"bitbucket.org/creachadair/stringset"
)

var (
	serverAddr = flag.String("address", os.Getenv("NOTIFIER_ADDR"), "Server address")
	editorCmd  = flag.String("editor", os.Getenv("EDITOR"), "Editor command line")
	soundName  = flag.String("sound", "Glass", "Sound name to use for audible notifications")
	voiceName  = flag.String("voice", "Moira", "Voice name to use for voice notifications")
	keyConfig  = flag.String("keyconfig", "", "Config file to load for key requests")
	debugLog   = flag.Bool("log", false, "Enable debug logging")

	lw *log.Logger

	// ResourceNotFound is returned when a requested resource is not found.
	ResourceNotFound = code.Register(-29998, "resource not found")
)

func main() {
	flag.Parse()
	if *serverAddr == "" {
		log.Fatal("A non-empty --address is required")
	} else if *debugLog {
		lw = log.New(os.Stderr, "[noteserver] ", log.LstdFlags)
	}

	lst, err := net.Listen("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Listen: %v", err)
	}
	if err := server.Loop(lst, jrpc2.ServiceMapper{
		"Notify": jrpc2.MapAssigner{
			"Post": jrpc2.NewMethod(handlePostNote),
			"Say":  jrpc2.NewMethod(handleSayNote),
		},
		"Clip": jrpc2.NewService(&clipper{
			saved: make(map[string][]byte),
		}),
		"User": jrpc2.MapAssigner{
			"Edit": jrpc2.NewMethod(handleEdit),
			"Text": jrpc2.NewMethod(handleText),
		},
		"Key": jrpc2.NewService(newKeygen(*keyConfig)),
	}, &server.LoopOptions{
		ServerOptions: &jrpc2.ServerOptions{
			Logger:  lw,
			Metrics: metrics.New(),
		},
	}); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handlePostNote(ctx context.Context, req *notifier.PostRequest) (bool, error) {
	if req.Body == "" && req.Title == "" {
		return false, jrpc2.Errorf(code.InvalidParams, "missing notification body and title")
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
	if wait := req.After; wait > 0 {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(req.After):
		}
	}
	err := cmd.Run()
	return err == nil, err
}

func handleSayNote(ctx context.Context, req *notifier.SayRequest) (bool, error) {
	if req.Text == "" {
		return false, jrpc2.Errorf(code.InvalidParams, "empty text")
	}
	if wait := req.After; wait > 0 {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(wait):
		}
	}
	cmd := exec.CommandContext(ctx, "say", "-v", *voiceName)
	cmd.Stdin = strings.NewReader(req.Text)
	err := cmd.Run()
	return err == nil, err
}

func handleText(ctx context.Context, req *notifier.TextRequest) (string, error) {
	if req.Prompt == "" {
		return "", jrpc2.Errorf(code.InvalidParams, "missing prompt string")
	}

	// Ask osascript to send error text to stdout to simplify error plumbing.
	cmd := exec.Command("osascript", "-s", "ho")
	cmd.Stdin = strings.NewReader(fmt.Sprintf(`display dialog %q default answer %q hidden answer %v`,
		req.Prompt, req.Default, req.Hide))
	raw, err := cmd.Output()
	out := strings.TrimRight(string(raw), "\n")
	if err != nil {
		if strings.Contains(out, "User canceled") {
			return "", notifier.UserCancelled
		}
		return "", err
	}

	// Parse the result out of the text delivered to stdout.
	const needle = "text returned:"
	if i := strings.Index(out, needle); i >= 0 {
		return out[i+len(needle):], nil
	}
	return "", jrpc2.Errorf(code.InternalError, "missing user input")
}

func handleEdit(ctx context.Context, req *notifier.EditRequest) ([]byte, error) {
	if *editorCmd == "" {
		return nil, errors.New("no editor is defined")
	} else if req.Name == "" {
		return nil, jrpc2.Errorf(code.InvalidParams, "missing file name")
	}

	// Store the file in a temporary directory so we have a place to point the
	// editor that will not conflict with other invocations. Use the name given
	// by the caller so the editor will display the "correct" name.
	tmp, err := ioutil.TempDir("", "User.Edit.")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp) // attempt to clean up
	path := filepath.Join(tmp, req.Name)
	if err := ioutil.WriteFile(path, req.Content, 0644); err != nil {
		return nil, err
	}
	args, _ := shell.Split(*editorCmd)
	bin, rest := args[0], args[1:]
	if err := exec.CommandContext(ctx, bin, append(rest, path)...).Run(); err != nil {
		return nil, err
	}
	return ioutil.ReadFile(path)
}

// systemClip is a special-case clipset tag that identifies the currently
// active system clipboard contents. It appears in clip listings, but is not
// stored in the server memory.
const systemClip = "active"

type clipper struct {
	sync.Mutex
	saved map[string][]byte
}

func (c *clipper) Set(ctx context.Context, req *notifier.ClipSetRequest) (bool, error) {
	if len(req.Data) == 0 && !req.AllowEmpty {
		return false, jrpc2.Errorf(code.InvalidParams, "empty clip data")
	} else if req.Tag != "" && req.Save == req.Tag {
		return false, jrpc2.Errorf(code.InvalidParams, "tag and save are equal")
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
	// The systemClip tag is a special case for the system clipboard.
	c.Lock()
	if req.Tag != "" && req.Tag != systemClip {
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
	if req.Tag == "" || req.Tag == systemClip {
		return getClip(ctx)
	} else if req.Activate && req.Tag == req.Save {
		return nil, jrpc2.Errorf(code.InvalidParams, "tag and save are equal")
	}
	c.Lock()
	defer c.Unlock()
	data, ok := c.saved[req.Tag]
	if !ok {
		return nil, jrpc2.Errorf(ResourceNotFound, "tag %q not found", req.Tag)
	} else if req.Activate {
		if req.Save != "" {
			active, err := getClip(ctx)
			if err != nil {
				return nil, err
			}
			c.saved[req.Save] = active
		}
		if err := setClip(ctx, data); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func (c *clipper) List(ctx context.Context) ([]string, error) {
	c.Lock()
	tags := stringset.FromKeys(c.saved)
	tags.Add(systemClip)
	c.Unlock()
	return tags.Elements(), nil
}

func (c *clipper) Clear(ctx context.Context, req *notifier.ClipClearRequest) (bool, error) {
	if req.Tag == "" || req.Tag == systemClip {
		err := setClip(ctx, nil)
		return err == nil, err
	}
	c.Lock()
	defer c.Unlock()
	_, ok := c.saved[req.Tag]
	delete(c.saved, req.Tag)
	return ok, nil
}

func setClip(ctx context.Context, data []byte) error {
	cmd := exec.CommandContext(ctx, "pbcopy")
	cmd.Stdin = bytes.NewReader(data)
	return cmd.Run()
}

func getClip(ctx context.Context) ([]byte, error) {
	return exec.CommandContext(ctx, "pbpaste").Output()
}

type keygen struct {
	μ      sync.Mutex
	cfg    *config.Config
	secret string
}

func newKeygen(path string) *keygen {
	cfg, err := loadKeyConfig(path)
	if err != nil {
		log.Fatalf("Creating key generator: %v", err)
	}
	gen := &keygen{cfg: cfg}

	// Set up a signal handler for SIGHUP, which causes the configuration file
	// to be reloaded.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		for range ch {
			// Reload the configuration file. Do this in a separate goroutine
			// so that we do not block the signal handler.
			go func() {
				nc, err := loadKeyConfig(path)
				if err != nil {
					log.Printf("ERROR: Reloading: %v", err)
				} else {
					log.Printf("Reloaded config from %q", path)
					gen.μ.Lock()
					gen.cfg = nc
					gen.μ.Unlock()
				}
			}()
		}
	}()
	return gen
}

func (k *keygen) site(host string) config.Site {
	k.μ.Lock()
	defer k.μ.Unlock()
	return k.cfg.Site(host)
}

func mergeSiteReq(site *config.Site, req *notifier.KeyGenRequest) {
	if req.Format != nil {
		site.Format = *req.Format
	}
	if req.Length != nil {
		site.Length = *req.Length
	}
	if req.Punct != nil {
		site.Punct = *req.Punct
	}
	if req.Salt != nil {
		site.Salt = *req.Salt
	}
}

func (k *keygen) Generate(ctx context.Context, req *notifier.KeyGenRequest) (string, error) {
	if req.Host == "" {
		return "", jrpc2.Errorf(code.InvalidParams, "missing host name")
	}
	const minLength = 6
	site := k.site(req.Host)
	mergeSiteReq(&site, req)
	if site.Length < minLength {
		return "", jrpc2.Errorf(code.InvalidParams, "invalid key length %d < %d", site.Length, minLength)
	} else if site.Format != "" && len(site.Format) < minLength {
		return "", jrpc2.Errorf(code.InvalidParams, "invalid format length %d < %d", len(site.Format), minLength)
	}

	secret, err := handleText(ctx, &notifier.TextRequest{
		Prompt: fmt.Sprintf("Secret key for %q", req.Host),
		Hide:   true,
	})
	if err != nil {
		return "", err
	}
	pctx := site.Context(secret)
	var pw string
	if fmt := site.Format; fmt != "" {
		pw = pctx.Format(site.Host, fmt)
	} else {
		pw = pctx.Password(site.Host, site.Length)
	}

	// If the user asked us to copy to the clipboard, return the verification
	// hash; otherwise return the passphrase itself.
	if req.Copy {
		return wordhash.String(pw), setClip(ctx, []byte(pw))
	}
	return pw, nil
}

func (k *keygen) List(ctx context.Context) ([]string, error) {
	k.μ.Lock()
	defer k.μ.Unlock()
	sites := stringset.FromKeys(k.cfg.Sites)
	return sites.Elements(), nil
}

func (k *keygen) Site(ctx context.Context, req *notifier.SiteRequest) (*config.Site, error) {
	if req.Host == "" {
		return nil, jrpc2.Errorf(code.InvalidParams, "missing host name")
	}
	site := k.site(req.Host)
	if !req.Full {
		site.Hints = nil
	}
	return &site, nil
}

func loadKeyConfig(path string) (*config.Config, error) {
	cfg := new(config.Config)
	if path == "" {
		cfg.Default.Length = 16
		return cfg, nil
	}
	if err := cfg.Load(path); err != nil {
		return nil, fmt.Errorf("loading key config: %v", err)
	}
	return cfg, nil
}
