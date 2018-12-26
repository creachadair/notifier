// Package notes exports a service that manages text notes files.
package notes

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/code"
	"bitbucket.org/creachadair/notifier"
)

func init() { notifier.RegisterPlugin("Notes", new(notes)) }

type notes struct {
	cfg *notifier.Config
}

// Init implements part of notifier.Plugin.
func (n *notes) Init(cfg *notifier.Config) error {
	if cfg.Notes.Default == nil && len(cfg.Notes.Categories) == 0 {
		return notifier.ErrNotApplicable
	}
	n.cfg = cfg
	return nil
}

// Update implements part of notifier.Plugin.
func (*notes) Update() error { return nil }

// Assigner implements part of notifier.Plugin.
func (n *notes) Assigner() jrpc2.Assigner { return jrpc2.NewService(n) }

func (n *notes) Edit(ctx context.Context, req *notifier.EditNotesRequest) error {
	if n.cfg.Edit.Command == "" {
		return errors.New("no editor is defined")
	}
	path, err := n.findNotePath(req)
	if err != nil {
		return err
	}
	log.Printf("Editing notes file %q...", path)
	return n.cfg.EditFile(ctx, path)
}

func (n *notes) List(ctx context.Context, req *notifier.ListNotesRequest) ([]*notifier.Note, error) {
	cat := n.findCategory(req.Category)
	if cat == nil {
		return nil, jrpc2.Errorf(code.InvalidParams, "invalid category: %q", req.Category)
	}
	return n.listNotes(req.Tag, cat)
}

func (n *notes) Read(ctx context.Context, req *notifier.EditNotesRequest) (string, error) {
	path, err := n.findNotePath(req)
	if err != nil {
		return "", err
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (n *notes) Categories(ctx context.Context) ([]string, error) {
	var cats []string
	for _, cat := range n.cfg.Notes.Categories {
		cats = append(cats, cat.Name)
	}
	sort.Strings(cats)
	return cats, nil
}

var noteName = regexp.MustCompile(`(.*)-([0-9]{4})([0-9]{2})([0-9]{2})\.\w+$`)

func (n *notes) findNotePath(req *notifier.EditNotesRequest) (string, error) {
	cat := n.findCategory(req.Category)
	if cat == nil {
		return "", jrpc2.Errorf(code.InvalidParams, "invalid category: %q", req.Category)
	} else if req.Tag == "" {
		return "", jrpc2.Errorf(code.InvalidParams, "missing base note name")
	} else if strings.Contains(req.Tag, "/") {
		return "", jrpc2.Errorf(code.InvalidParams, "tag may not contain '/'")
	}

	var version string
	tag := req.Tag
	if req.Version == "new" {
		version = time.Now().Format("20060102")
	} else if req.Version == "" || req.Version == "latest" {
		// If we found an existing version, override the specified tag so that we
		// get the actual file extension.
		old, err := n.latestNote(tag, cat)
		if err != nil {
			return "", err
		}
		tag = old.Tag
		version = strings.Replace(old.Version, "-", "", -1)
	} else if t, err := time.Parse("2006-01-02", req.Version); err != nil {
		return "", jrpc2.Errorf(code.InvalidParams, "invalid version: %v", err)
	} else {
		version = t.Format("20060102")
	}

	// Extract the file extension from the tag, e.g., base.txt, base.md.
	// Default to the config's extension or .txt if none was included.
	base, ext := splitExt(tag)
	if ext == "" {
		ext = cat.Suffix
		if ext == "" {
			ext = ".txt"
		}
	}
	name := fmt.Sprintf("%s-%s%s", base, version, ext)
	return filepath.Join(os.ExpandEnv(cat.Dir), name), nil
}

func (n *notes) latestNote(tag string, cat *notifier.NoteCategory) (*notifier.Note, error) {
	old, err := n.listNotes(tag, cat)
	if err != nil {
		return nil, err
	} else if len(old) == 0 {
		return nil, fmt.Errorf("no notes matching %q", tag)
	}
	sort.Slice(old, func(i, j int) bool {
		return notifier.NoteLess(old[j], old[i])
	})
	return old[0], nil
}

func (n *notes) findCategory(name string) *notifier.NoteCategory {
	if name == "" {
		return n.cfg.Notes.Default
	}
	for _, cat := range n.cfg.Notes.Categories {
		if cat.Name == name {
			return cat
		}
	}
	return nil
}

func (n *notes) listNotes(tag string, cat *notifier.NoteCategory) ([]*notifier.Note, error) {
	base, ext := splitExt(tag)
	if ext == "" {
		ext = cat.Suffix
	}
	tglob := base + "-????????" + ext
	if ext == "" {
		tglob += ".*"
	}
	if base == "" {
		tglob = "*" + tglob
	}

	pattern := filepath.Join(os.ExpandEnv(cat.Dir), tglob)
	names, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	var rsp []*notifier.Note
	for _, name := range names {
		m := noteName.FindStringSubmatch(filepath.Base(name))
		if m == nil {
			continue
		}
		tag := m[1]
		if ext := filepath.Ext(name); ext != ".txt" {
			tag += ext
		}
		rsp = append(rsp, &notifier.Note{
			Tag:     tag,
			Version: fmt.Sprintf("%s-%s-%s", m[2], m[3], m[4]),
		})
	}
	return rsp, nil
}

func splitExt(name string) (base, ext string) {
	ext = filepath.Ext(name)
	base = strings.TrimSuffix(name, ext)
	return
}
