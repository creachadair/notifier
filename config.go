package notifier

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bitbucket.org/creachadair/jrpc2/jauth"
	"bitbucket.org/creachadair/jrpc2/jctx"
	"bitbucket.org/creachadair/shell"
	yaml "gopkg.in/yaml.v2"
)

// A NoteCategory describes the configuration settings for a notes category.
type NoteCategory struct {
	Name   string // the name of the category
	Dir    string // the direcory where notes are stored
	Suffix string // the default file suffix for this category
}

// Config stores settings for the various notifier services.
type Config struct {
	Address  string
	DebugLog bool `yaml:"debugLog"`

	Auth AuthConfig `yaml:"auth,omitempty"`

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
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.UnmarshalStrict(data, cfg)
}

// EditFile edits a file using the editor specified by c.
func (c *Config) EditFile(ctx context.Context, path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) && c.Edit.TouchNew {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		f.Close()
	}
	args, _ := shell.Split(c.Edit.Command)
	bin, rest := args[0], args[1:]
	return exec.CommandContext(ctx, bin, append(rest, path)...).Run()
}

// AuthConfig specifies a mapping of usernames to authorization rules.
type AuthConfig map[string]ACL

// An ACL represents a set of access rights protected by a key.  If the list of
// rules is empty, all methods are accessible.
type ACL struct {
	Key   string   `json:"key"`
	Rules []string `json:"acl,omitempty"`
}

func (a ACL) checkMethod(method string) bool {
	for _, rule := range a.Rules {
		ok, err := filepath.Match(rule, method)
		if err == nil && ok {
			return true
		}
	}
	return len(a.Rules) == 0
}

// CheckAuth checks whether the specified request is authorized.
func (a AuthConfig) CheckAuth(ctx context.Context, method string, params []byte) error {
	if a == nil || strings.HasPrefix(method, "rpc.") {
		return nil
	}
	raw, ok := jctx.AuthToken(ctx)
	if !ok {
		return errors.New("no authorization token")
	}
	tok, err := jauth.ParseToken(raw)
	if err != nil {
		return jauth.ErrInvalidToken
	}
	acl, ok := a[tok.User]
	if !ok {
		return errors.New("unknown user")
	} else if !acl.checkMethod(method) {
		return errors.New("method not allowed")
	}

	return jauth.User{
		Name: tok.User,
		Key:  []byte(acl.Key),
	}.VerifyParsed(tok, method, params)
}
