package notifier

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"

	"bitbucket.org/creachadair/shell"
)

// Config stores settings for the various notifier services.
type Config struct {
	Address  string `json:"address"`
	DebugLog bool   `json:"debugLog"`

	// Settings for the clipboard service.
	Clip struct {
		SaveFile string `json:"saveFile"`
	} `json:"clip"`

	// Settings for the editor service.
	Edit struct {
		Command  string `json:"command"`
		TouchNew bool   `json:"touchNew"`
	} `json:"edit"`

	// Settings for the notes service.
	Notes struct {
		NotesDir string `json:"notesDir"`
	}

	// Settings for the key generation service.
	Key struct {
		ConfigFile string `json:"configFile"`
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
	return json.Unmarshal(data, cfg)
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
