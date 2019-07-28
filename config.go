package notifier

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"bitbucket.org/creachadair/shell"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/jrpc2/jctx"
	yaml "gopkg.in/yaml.v3"
)

// NotAuthorized is an error code returned for unauthorized requests.
var NotAuthorized = code.Register(-29997, "request not authorized")

// Auth is used to encode an authorization token.
type Auth struct {
	Token string `json:"token"`
}

// A NoteCategory describes the configuration settings for a notes category.
type NoteCategory struct {
	Name   string `json:"name"`             // the name of the category
	Dir    string `json:"dir"`              // the direcory where notes are stored
	Suffix string `json:"suffix,omitempty"` // the default file suffix for this category
}

// FilePath constructs a filepath for the specified base, version, and
// extension relative to the directory of the specified category.
func (c *NoteCategory) FilePath(base, version, ext string) string {
	if ext == "" {
		ext = c.Suffix
	}
	name := fmt.Sprintf("%s-%s%s", base, version, ext)
	return filepath.Join(os.ExpandEnv(c.Dir), name)
}

// Config stores settings for the various notifier services.
type Config struct {
	Address  string
	DebugLog bool `yaml:"debugLog"`
	Token    string

	// Settings for the clipboard service.
	Clip struct {
		SaveFile string `yaml:"saveFile"`
	}

	// Settings for the editor service.
	Edit struct {
		Command  string
		TouchNew bool `yaml:"touchNew"`
	}

	// Settings for the notes service.
	Notes struct {
		Categories []*NoteCategory
	}

	// Settings for the key generation service.
	Key struct {
		ConfigFile string `yaml:"configFile"`
	}

	// Settings for the notification service.
	Notify struct {
		Sound string
		Voice string
	}
}

// LoadConfig loads a configuration from the file at path into *cfg.
func LoadConfig(path string, cfg *Config) error {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	return dec.Decode(cfg)
}

// EditFile edits a file using the editor specified by c.
func (c *Config) EditFile(ctx context.Context, path string) error {
	cmd, err := c.EditFileCmd(ctx, path)
	if err != nil {
		return err
	}
	return cmd.Run()
}

// EditFileCmd returns a command to edit the specified file using the editor
// specified by c. The caller must run or start the command.
func (c *Config) EditFileCmd(ctx context.Context, path string) (*exec.Cmd, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) && c.Edit.TouchNew {
		f, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		f.Close()
	}
	args, _ := shell.Split(c.Edit.Command)
	bin, rest := args[0], args[1:]
	return exec.CommandContext(ctx, bin, append(rest, path)...), nil
}

// CheckRequest verifies that the specified request is authorized.  If no token
// is set, all requests are accepted.
func (c *Config) CheckRequest(ctx context.Context, req *jrpc2.Request) error {
	if c == nil || c.Token == "" || req.Method() == "rpc.serverInfo" {
		return nil // accept
	}
	var auth Auth
	err := jctx.UnmarshalMetadata(ctx, &auth)
	if err != nil || auth.Token != c.Token {
		return NotAuthorized.Err()
	}
	return nil
}
