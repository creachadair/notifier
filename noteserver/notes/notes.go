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
	"bitbucket.org/creachadair/jrpc2/handler"
	"bitbucket.org/creachadair/notifier"
)

func init() { notifier.RegisterPlugin("Notes", new(notes)) }

type notes struct {
	cfg *notifier.Config
}

// Init implements part of notifier.Plugin.
func (n *notes) Init(cfg *notifier.Config) error {
	if len(cfg.Notes.Categories) == 0 {
		return notifier.ErrNotApplicable
	}
	for _, cat := range cfg.Notes.Categories {
		if cat.Suffix == "" {
			cat.Suffix = ".txt"
		}
	}
	n.cfg = cfg
	return nil
}

// Update implements part of notifier.Plugin.
func (*notes) Update() error { return nil }

// Assigner implements part of notifier.Plugin.
func (n *notes) Assigner() jrpc2.Assigner { return handler.NewService(n) }

func (n *notes) Edit(ctx context.Context, req *notifier.EditNotesRequest) error {
	if n.cfg.Edit.Command == "" {
		return errors.New("no editor is defined")
	}
	path, err := n.findNotePath(req)
	if err != nil {
		return err
	}

	// N.B. Call the editor with a background context, so that it does not
	// terminate when this request returns.
	cmd, err := n.cfg.EditFileCmd(context.Background(), path)
	if err != nil {
		return err
	} else if err := cmd.Start(); err != nil {
		return err
	}
	log.Printf("[pid=%d] Editing notes file %q...", cmd.Process.Pid, path)
	if req.Background {
		go func() {
			log.Printf("[pid=%d] Editor (async) exited: %v", cmd.Process.Pid, cmd.Wait())
		}()
		return nil
	}
	err = cmd.Wait()
	log.Printf("[pid=%d] Editor (sync) exited: %v", cmd.Process.Pid, err)
	return err
}

func (n *notes) List(ctx context.Context, req *notifier.ListNotesRequest) ([]*notifier.Note, error) {
	cats := n.findCategories(req.Category)
	if cats == nil {
		return nil, jrpc2.Errorf(code.InvalidParams, "invalid category: %q", req.Category)
	}
	base, ext := splitExt(req.Tag)
	ns, err := n.filterAndSort(base, req.Version, ext, cats)
	if err != nil {
		return nil, err
	}
	return ns, nil
}

func (n *notes) Read(ctx context.Context, req *notifier.EditNotesRequest) (*notifier.NoteWithText, error) {
	note, err := n.findNote(req)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadFile(note.Path)
	if err != nil {
		return nil, err
	}
	return &notifier.NoteWithText{
		Note: note,
		Text: data,
	}, nil
}

func (n *notes) Categories(ctx context.Context) ([]*notifier.NoteCategory, error) {
	return n.cfg.Notes.Categories, nil
}

var noteName = regexp.MustCompile(`(.*)-([0-9]{4})([0-9]{2})([0-9]{2})\.\w+$`)

func (n *notes) findNotePath(req *notifier.EditNotesRequest) (string, error) {
	note, err := n.findNote(req)
	if err != nil {
		return "", err
	}
	return note.Path, nil
}

func (n *notes) findNote(req *notifier.EditNotesRequest) (*notifier.Note, error) {
	cats := n.findCategories(req.Category)
	if cats == nil {
		return nil, jrpc2.Errorf(code.InvalidParams, "invalid category: %q", req.Category)
	} else if req.Tag == "" {
		return nil, jrpc2.Errorf(code.InvalidParams, "missing base note name")
	} else if strings.Contains(req.Tag, "/") {
		return nil, jrpc2.Errorf(code.InvalidParams, "tag may not contain '/'")
	}

	base, ext := splitExt(req.Tag)

	// Case 1: Finding the path for a new note.  In this case, the category must
	// be uniquely specified.
	if req.Version == "new" {
		if len(cats) != 1 {
			return nil, errors.New("no category specified for new note")
		}
		version := time.Now().Format("20060102")
		path := cats[0].FilePath(base, version, ext)
		return &notifier.Note{
			Tag:      base,
			Version:  version,
			Suffix:   filepath.Ext(path),
			Category: cats[0].Name,
			Path:     path,
		}, nil
	}

	// Case 2: Finding the path for the latest version. We can search all
	// categories if there isn't one specified.
	if req.Version == "" || req.Version == "latest" {
		ns, err := n.filterAndSort(base, "", ext, cats)
		if err != nil {
			return nil, err
		} else if len(ns) == 0 {
			return nil, fmt.Errorf("no notes matching %q", req.Tag)
		}
		return ns[len(ns)-1], nil
	}

	// Case 3: Finding the path for a specific version.
	if _, err := time.Parse("2006-01-02", req.Version); err != nil {
		return nil, jrpc2.Errorf(code.InvalidParams, "invalid version: %v", err)
	}
	ns, err := n.filterAndSort(base, req.Version, ext, cats)
	if err != nil {
		return nil, err
	} else if len(ns) == 0 {
		return nil, fmt.Errorf("no notes matching version %s of %q", req.Version, req.Tag)
	} else if len(ns) > 1 {
		return nil, fmt.Errorf("multiple notes (%d) matching version %s of %q",
			len(ns), req.Version, req.Tag)
	}
	return ns[len(ns)-1], nil
}

func (n *notes) filterAndSort(tag, version, suffix string, cats []*notifier.NoteCategory) ([]*notifier.Note, error) {
	var match []*notifier.Note
	for _, cat := range cats {
		nc, err := n.listNotes(tag, cat)
		if err != nil {
			return nil, err
		}
		for _, note := range nc {
			if version != "" {
				if ok, err := filepath.Match(version, note.Version); err == nil && !ok {
					continue
				}
			}
			if suffix != "" && note.Suffix != suffix {
				continue
			}
			match = append(match, note)
		}
	}
	sort.Slice(match, func(i, j int) bool {
		return notifier.NoteLess(match[i], match[j])
	})
	return match, nil
}

func (n *notes) findCategories(name string) []*notifier.NoteCategory {
	if name == "" {
		return n.cfg.Notes.Categories
	}
	for _, cat := range n.cfg.Notes.Categories {
		if cat.Name == name {
			return []*notifier.NoteCategory{cat}
		}
	}
	return nil
}

func (n *notes) listNotes(tag string, cat *notifier.NoteCategory) ([]*notifier.Note, error) {
	base, ext := splitExt(tag)
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
		rsp = append(rsp, &notifier.Note{
			Tag:      m[1],
			Version:  fmt.Sprintf("%s-%s-%s", m[2], m[3], m[4]),
			Suffix:   filepath.Ext(name),
			Category: cat.Name,
			Path:     name,
		})
	}
	return rsp, nil
}

func splitExt(name string) (base, ext string) {
	ext = filepath.Ext(name)
	return strings.TrimSuffix(name, ext), ext
}
