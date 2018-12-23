package notifier

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"bitbucket.org/creachadair/jrpc2/jauth"
	"bitbucket.org/creachadair/jrpc2/jctx"
	"bitbucket.org/creachadair/shell"
	yaml "gopkg.in/yaml.v2"
)

// Config stores settings for the various notifier services.
type Config struct {
	Address  string `json:"address"`
	DebugLog bool   `json:"debugLog" yaml:"debugLog"`

	Auth AuthConfig `json:"auth,omitempty"`

	// Settings for the clipboard service.
	Clip struct {
		SaveFile string `json:"saveFile" yaml:"saveFile"`
	} `json:"clip"`

	// Settings for the editor service.
	Edit struct {
		Command  string `json:"command"`
		TouchNew bool   `json:"touchNew" yaml:"touchNew"`
	} `json:"edit"`

	// Settings for the notes service.
	Notes struct {
		NotesDir string `json:"notesDir" yaml:"notesDir"`
	} `json:"notes"`

	// Settings for the key generation service.
	Key struct {
		ConfigFile string `json:"configFile" yaml:"configFile"`
	} `json:"key"`

	// Settings for the notification service.
	Note struct {
		Sound string `json:"sound"`
		Voice string `json:"voice"`
	} `json:"note"`
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
	if a == nil {
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
