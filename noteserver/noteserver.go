// Program noteserver implements a server for posting notifications.
//
// Usage:
//    noteserver -address :8080
//
package main

import (
	"bytes"
	"context"
	"encoding/json"
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
	"regexp"
	"sort"
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
	cfg notifier.Config
	lw  *log.Logger

	// ResourceNotFound is returned when a requested resource is not found.
	ResourceNotFound = code.Register(-29998, "resource not found")

	configPath = flag.String("config", "", "Configuration file path (overrides other flags)")
)

func init() {
	flag.StringVar(&cfg.Address, "address", os.Getenv("NOTIFIER_ADDR"), "Server address")
	flag.StringVar(&cfg.Edit.Command, "editor", os.Getenv("EDITOR"), "Editor command line")
	flag.StringVar(&cfg.Note.Sound, "sound", "Glass", "Sound name to use for audible notifications")
	flag.StringVar(&cfg.Note.Voice, "voice", "Moira", "Voice name to use for voice notifications")
	flag.StringVar(&cfg.Key.ConfigFile, "keyconfig", "", "Config file to load for key requests")
	flag.StringVar(&cfg.Clip.SaveFile, "clips", "", "Storage file for named clips")
	flag.BoolVar(&cfg.DebugLog, "log", false, "Enable debug logging")
}

func main() {
	flag.Parse()
	if err := notifier.LoadConfig(*configPath, &cfg); err != nil {
		log.Fatalf("Loading configuration: %v", err)
	}
	if cfg.Address == "" {
		log.Fatal("A non-empty --address is required")
	} else if cfg.DebugLog {
		lw = log.New(os.Stderr, "[noteserver] ", log.LstdFlags)
	}
	c := &clipper{
		store: os.ExpandEnv(cfg.Clip.SaveFile),
		saved: make(map[string][]byte),
	}
	if err := c.loadFromFile(); err != nil {
		log.Fatalf("Loading clips: %v", err)
	}

	lst, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		log.Fatalf("Listen: %v", err)
	}
	if err := server.Loop(lst, jrpc2.ServiceMapper{
		"Notify": jrpc2.MapAssigner{
			"Post": jrpc2.NewHandler(handlePostNote),
			"Say":  jrpc2.NewHandler(handleSayNote),
		},
		"Clip": jrpc2.NewService(c),
		"User": jrpc2.MapAssigner{
			"Edit": jrpc2.NewHandler(handleEdit),
			"Text": jrpc2.NewHandler(handleText),
		},
		"Notes": jrpc2.MapAssigner{
			"Edit": jrpc2.NewHandler(handleEditNotes),
			"List": jrpc2.NewHandler(handleListNotes),
		},
		"Key": jrpc2.NewService(newKeygen(os.ExpandEnv(cfg.Key.ConfigFile))),
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
		program = append(program, fmt.Sprintf("sound name %q", cfg.Note.Sound))
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
	cmd := exec.CommandContext(ctx, "say", "-v", cfg.Note.Voice)
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
	if cfg.Edit.Command == "" {
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
	} else if err := editFile(ctx, path, cfg.Edit.TouchNew); err != nil {
		return nil, err
	}
	return ioutil.ReadFile(path)
}

func handleEditNotes(ctx context.Context, req *notifier.EditNotesRequest) error {
	if cfg.Edit.Command == "" {
		return errors.New("no editor is defined")
	} else if cfg.Edit.NotesDir == "" {
		return errors.New("no notes directory is defined")
	} else if req.Tag == "" {
		return jrpc2.Errorf(code.InvalidParams, "missing base note name")
	} else if strings.Contains(req.Tag, "/") {
		return jrpc2.Errorf(code.InvalidParams, "base may not contain '/'")
	} else if strings.Contains(req.Category, "/") {
		return jrpc2.Errorf(code.InvalidParams, "category may not contain '/'")
	}

	var version string
	if req.Version == "" {
		version = time.Now().Format("20060102")
	} else if req.Version == "latest" {
		old, err := latestNote(req.Tag, req.Category)
		if err != nil {
			return err
		}
		version = strings.Replace(old.Version, "-", "", -1)
	} else if t, err := time.Parse("2006-01-02", req.Version); err != nil {
		return jrpc2.Errorf(code.InvalidParams, "invalid version: %v", err)
	} else {
		version = t.Format("20060102")
	}

	name := fmt.Sprintf("%s-%s.txt", req.Tag, version)
	path := filepath.Join(os.ExpandEnv(cfg.Edit.NotesDir), req.Category, name)
	log.Printf("Editing notes file %q...", path)
	return editFile(ctx, path, cfg.Edit.TouchNew)
}

var noteName = regexp.MustCompile(`(.*)-([0-9]{4})([0-9]{2})([0-9]{2})\.txt$`)

func handleListNotes(ctx context.Context, req *notifier.ListNotesRequest) ([]*notifier.Note, error) {
	if cfg.Edit.NotesDir == "" {
		return nil, errors.New("no notes directory is defined")
	}
	return listNotes(req.Tag, req.Category)
}

func latestNote(tag, category string) (*notifier.Note, error) {
	old, err := listNotes(tag, category)
	if err != nil {
		return nil, err
	} else if len(old) == 0 {
		return nil, fmt.Errorf("no notes matching %q", tag)
	}
	sort.Slice(old, func(i, j int) bool {
		return notifier.NoteLess(old[j], old[i])
	})
	return old[0], nil
}

func listNotes(tag, category string) ([]*notifier.Note, error) {
	if tag == "" {
		tag = "*-????????.txt"
	} else {
		tag += "-????????.txt"
	}
	pattern := filepath.Join(os.ExpandEnv(cfg.Edit.NotesDir), category, tag)
	names, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	var rsp []*notifier.Note
	for _, name := range names {
		m := noteName.FindStringSubmatch(filepath.Base(name))
		if m == nil {
			continue
		}
		rsp = append(rsp, &notifier.Note{
			Tag:     m[1],
			Version: fmt.Sprintf("%s-%s-%s", m[2], m[3], m[4]),
		})
	}
	return rsp, nil
}

func editFile(ctx context.Context, path string, create bool) error {
	if _, err := os.Stat(path); os.IsNotExist(err) && create {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		f.Close()
	}
	args, _ := shell.Split(cfg.Edit.Command)
	bin, rest := args[0], args[1:]
	return exec.CommandContext(ctx, bin, append(rest, path)...).Run()
}

// systemClip is a special-case clipset tag that identifies the currently
// active system clipboard contents. It appears in clip listings, but is not
// stored in the server memory.
const systemClip = "active"

type clipper struct {
	store string

	sync.Mutex
	saved map[string][]byte
}

// saveToFile writes the contents of c.saved to the output file, if one is set.
// The caller must hold the lock on c.
func (c *clipper) saveToFile() error {
	if c.store == "" {
		return nil
	}
	out, err := json.Marshal(c.saved)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(c.store, out, 0644)
}

// loadFromFile loads the contents of c.saved and merges it with the current data.
// The caller must hold the lock on c.
func (c *clipper) loadFromFile() error {
	if c.store == "" {
		return nil
	}
	bits, err := ioutil.ReadFile(c.store)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	m := make(map[string][]byte)
	if err := json.Unmarshal(bits, &m); err != nil {
		return err
	}
	for key, val := range m {
		c.saved[key] = val
	}
	return nil
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
	c.saveToFile()
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
	return ok, c.saveToFile()
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
		Prompt: fmt.Sprintf("Secret key for %q", site.Host),
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
		return site.Host + "\t" + wordhash.String(pw), setClip(ctx, []byte(pw))
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
