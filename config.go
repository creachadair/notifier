package notifier

import (
	"context"
	"os"
	"os/exec"

	"bitbucket.org/creachadair/shell"
	yaml "gopkg.in/yaml.v3"
)

// Config stores settings for the various notifier services.
type Config struct {
	Address  string
	DebugLog bool `yaml:"debugLog"`

	// Settings for the clipboard service.
	Clip struct {
		SaveFile string `yaml:"saveFile"`
	}

	// Settings for the editor service.
	Edit struct {
		Command  string
		TouchNew bool `yaml:"touchNew"`
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
