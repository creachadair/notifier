// Package clipper exports a service that manages the system clipboard, and
// provides named ancillary clipboard storage.
package clipper

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/atomicfile"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/notifier"
)

func init() { notifier.RegisterPlugin("Clip", new(clipper)) }

// systemClip is a special-case clipset tag that identifies the currently
// active system clipboard contents. It appears in clip listings, but is not
// stored in the server memory.
const systemClip = "active"

type clipper struct {
	store string

	sync.Mutex
	saved map[string][]byte
}

// Init implements part of notifier.Plugin.
func (c *clipper) Init(cfg *notifier.Config) error {
	c.store = os.ExpandEnv(cfg.Clip.SaveFile)
	c.saved = make(map[string][]byte)
	if err := c.loadFromFile(); err != nil {
		return fmt.Errorf("loading saved clips: %v", err)
	}
	return nil
}

// Update implements part of notifier.Plugin.
func (*clipper) Update() error { return nil }

// Assigner implements part of notifier.Plugin.
func (c *clipper) Assigner() jrpc2.Assigner { return handler.NewService(c) }

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
	return atomicfile.WriteData(c.store, out, 0600)
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

	if err := notifier.SetSystemClipboard(ctx, req.Data); err != nil {
		return false, err
	}

	// If a tag was provided, save the new clip under that tag.
	// If a save tag was provided, save the existing clip under that tag.
	// The systemClip tag is a special case for the system clipboard.
	c.Lock()
	if req.Tag != "" && req.Tag != systemClip {
		c.saved[req.Tag] = req.Data
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
		return nil, jrpc2.Errorf(notifier.ResourceNotFound, "tag %q not found", req.Tag)
	} else if req.Activate {
		if req.Save != "" {
			active, err := getClip(ctx)
			if err != nil {
				return nil, err
			}
			c.saved[req.Save] = active
		}
		if err := notifier.SetSystemClipboard(ctx, data); err != nil {
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
		err := notifier.SetSystemClipboard(ctx, nil)
		return err == nil, err
	}
	c.Lock()
	defer c.Unlock()
	_, ok := c.saved[req.Tag]
	delete(c.saved, req.Tag)
	return ok, c.saveToFile()
}

func getClip(ctx context.Context) ([]byte, error) {
	return exec.CommandContext(ctx, "pbpaste", "-pboard", "general").Output()
}
