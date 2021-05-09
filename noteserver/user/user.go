// Package user exports a service to read input from the user.
package user

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/notifier"
)

func init() { notifier.RegisterPlugin("User", new(input)) }

type input struct {
	cfg *notifier.Config
}

// Init implements part of notifier.Plugin.
func (u *input) Init(cfg *notifier.Config) error {
	u.cfg = cfg
	return nil
}

// Update implements part of notifier.Plugin.
func (*input) Update() error { return nil }

// Assigner implements part of notifier.Plugin.
func (u *input) Assigner() jrpc2.Assigner {
	return handler.Map{
		"Text": handler.New(u.Text),
		"Edit": handler.New(u.Edit),
	}
}

// Text prompts the user for textual input.
func (u *input) Text(ctx context.Context, req *notifier.TextRequest) (string, error) {
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
			return "", jrpc2.Errorf(notifier.UserCancelled, "user cancelled request")
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

// Edit opens the designated editor for a file.
func (u *input) Edit(ctx context.Context, req *notifier.EditRequest) ([]byte, error) {
	if u.cfg.Edit.Command == "" {
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
	} else if err := u.cfg.EditFile(ctx, path); err != nil {
		return nil, err
	}
	return ioutil.ReadFile(path)
}
