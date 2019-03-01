package notifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/jctx"
	"bitbucket.org/creachadair/notifier/noteserver/jauth"
	"bitbucket.org/creachadair/shell"
	yaml "gopkg.in/yaml.v2"
)

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
		t := strings.TrimPrefix(rule, "-")
		ok, err := filepath.Match(t, method)
		if err == nil && ok {
			return t == rule
		}
	}
	return len(a.Rules) == 0
}

// CheckAuth checks whether the specified request is authorized.
func (a AuthConfig) CheckAuth(ctx context.Context, req *jrpc2.Request) error {
	method := req.Method()
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
	var params json.RawMessage
	req.UnmarshalParams(&params)
	return jauth.User{
		Name: tok.User,
		Key:  []byte(acl.Key),
	}.VerifyParsed(tok, method, params)
}
