package notifier

import (
	"encoding/json"
	"io/ioutil"
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
		Command string `json:"command"`
	} `json:"edit"`

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
