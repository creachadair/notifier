package notifier

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/jrpc2/handler"
)

// ResourceNotFound is returned when a requested resource is not found.
var ResourceNotFound = code.Register(-29998, "resource not found")

// ErrNotApplicable is returned by a plugin's Init function if the plugin
// cannot be used with the given configuration.
var ErrNotApplicable = errors.New("plugin is not applicable")

// A Plugin exposes a set of methods.
type Plugin interface {
	// Init is called once before any other methods of the plugin are used, with
	// a pointer to the shared configuration.
	Init(*Config) error

	// Update may be called periodically to give the plugin an opportunity to
	// update its state.
	Update() error

	// Assigner returns an assigner for handlers.
	Assigner() jrpc2.Assigner
}

var plugins = make(map[string]Plugin)
var setup sync.Once

// RegisterPlugin registers a plugin. This function will panic if the same name
// is registered multiple times.
func RegisterPlugin(name string, p Plugin) {
	if old, ok := plugins[name]; ok {
		log.Panicf("Duplicate registration for plugin %q: %v, %v", name, old, p)
	} else if p == nil {
		log.Panicf("Invalid nil plugin for %q", name)
	}
	plugins[name] = p
}

// PluginAssigner returns a jrpc2.Assigner that exports the methods of all the
// registered plugins.
func PluginAssigner(cfg *Config) jrpc2.Assigner {
	svc := make(handler.ServiceMap)
	for name, plug := range plugins {
		if err := plug.Init(cfg); err == ErrNotApplicable {
			log.Printf("Skipping inapplicable plugin %q", name)
		} else if err != nil {
			log.Panicf("Initializing plugin %q: %v", name, err)
		} else {
			svc[name] = plug.Assigner()
		}
	}

	// Set up a signal handler for SIGHUP, which causes the plugins to be
	// updated.
	setup.Do(func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGHUP)
		go func() {
			for range ch {
				for name, plug := range plugins {
					name, plug := name, plug
					go func() {
						if err := plug.Update(); err != nil {
							log.Printf("ERROR updating plugin %q: %v", name, err)
						}
					}()
				}
			}
		}()
	})
	return svc
}

// PromptForText requests a string of text from the user.
func PromptForText(ctx context.Context, req *TextRequest) (string, error) {
	if req.Prompt == "" {
		return "", jrpc2.Errorf(code.InvalidParams, "missing prompt string")
	}

	// Ask osascript to send error text to stdout to simplify error plumbing.
	cmd := exec.CommandContext(ctx, "osascript", "-s", "ho")
	cmd.Stdin = strings.NewReader(fmt.Sprintf(`display dialog %q default answer %q hidden answer %v`,
		req.Prompt, req.Default, req.Hide))
	raw, err := cmd.Output()
	out := strings.TrimRight(string(raw), "\n")
	if err != nil {
		if strings.Contains(out, "User canceled") {
			return "", jrpc2.Errorf(UserCancelled, "user cancelled request")
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

// SetSystemClipboard sets the contents of the system clipboard to data.
func SetSystemClipboard(ctx context.Context, data []byte) error {
	cmd := exec.CommandContext(ctx, "pbcopy", "-pboard", "general")
	cmd.Stdin = bytes.NewReader(data)
	return cmd.Run()
}
