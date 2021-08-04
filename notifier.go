// Package notifier contains common data structures for notifications.
package notifier

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"text/tabwriter"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/jrpc2/jctx"
)

var (
	serverAddr = os.Getenv("NOTIFIER_ADDR") // see RegisterFlags
	authToken  = os.Getenv("NOTIFIER_TOKEN")

	debug *log.Logger
)

func init() {
	switch os.Getenv("NOTIFIER_DEBUG") {
	case "1", "t", "true", "yes", "on":
		debug = log.New(os.Stderr, "[client:debug] ", log.Lmicroseconds)
	}
}

// RegisterFlags installs a standard -server flag in the default flagset.
// This function should be called during init in a client main package.
func RegisterFlags() {
	flag.StringVar(&serverAddr, "server", serverAddr, "Server address")
}

// Dial connects to the flag-selected JSON-RPC server and returns a context and
// a client ready for use. The caller is responsible for closing the client.
func Dial(ctx context.Context) (context.Context, *jrpc2.Client, error) {
	// If an auth token is available, attach it to the context.
	if authToken != "" {
		var err error
		ctx, err = jctx.WithMetadata(ctx, Auth{Token: authToken})
		if err != nil {
			return ctx, nil, err
		}
	}

	// Dial the server: host:port is tcp, otherwise a Unix socket.
	addr, atype := jrpc2.Network(serverAddr)
	if atype == "unix" {
		addr = os.ExpandEnv(addr)
	}
	conn, err := net.Dial(atype, addr)
	if err != nil {
		return ctx, nil, fmt.Errorf("address %q: %v", addr, err)
	}

	cli := jrpc2.NewClient(channel.RawJSON(conn, conn), &jrpc2.ClientOptions{
		EncodeContext: jctx.Encode,
		Logger:        debug,
	})
	return ctx, cli, nil
}

// A PostRequest is a request to post a notification to the user.
type PostRequest struct {
	Title    string        `json:"title,omitempty"`
	Subtitle string        `json:"subtitle,omitempty"`
	Body     string        `json:"body,omitempty"`
	Audible  bool          `json:"audible,omitempty"`
	After    time.Duration `json:"after,omitempty"`
}

func (PostRequest) DisallowUnknownFields() {}

// A ClipSetRequest is sent to update the contents of the clipboard.
type ClipSetRequest struct {
	Data       []byte `json:"data"`           // the data to be stored
	Tag        string `json:"tag,omitempty"`  // the tag to assign the data
	Save       string `json:"save,omitempty"` // save active clip to this tag
	AllowEmpty bool   `json:"allowEmpty"`     // allow data to be empty
}

func (ClipSetRequest) DisallowUnknownFields() {}

// A ClipGetRequest is sent to query the contents of the clipboard.
type ClipGetRequest struct {
	Tag      string `json:"tag,omitempty"`      // the tag to assign the data
	Save     string `json:"save,omitempty"`     // save active clip to this tag
	Activate bool   `json:"activate,omitempty"` // make this clip active
}

func (ClipGetRequest) DisallowUnknownFields() {}

// A ClipClearRequest is sent to clear the contents of the clipboard.
type ClipClearRequest struct {
	Tag string `json:"tag,omitempty"` // the tag to clear or remove
}

func (ClipClearRequest) DisallowUnknownFields() {}

// A SayRequest is a request to speak a notification to the user.
type SayRequest struct {
	Text  string        `json:"text"`
	Voice string        `json:"voice,omitempty"`
	After time.Duration `json:"after,omitempty"`
}

func (SayRequest) DisallowUnknownFields() {}

// A TextRequest is a request to read a string from the user.
type TextRequest struct {
	Prompt  string `json:"prompt,omitempty"`
	Default string `json:"default,omitempty"`
	Hide    bool   `json:"hide,omitempty"`
}

func (TextRequest) DisallowUnknownFields() {}

// UserCancelled is the code returned when a user cancels a text request.
var UserCancelled = code.Register(-29999, "user cancelled request")

// A KeyGenRequest is a request to generate a password.
type KeyGenRequest struct {
	Host   string  `json:"host,omitempty"`   // host or site label
	Strict bool    `json:"strict,omitempty"` // report error if label is not known
	Copy   bool    `json:"copy,omitempty"`   // copy to clipboard
	Format *string `json:"format,omitempty"` // render using this format
	Length *int    `json:"length,omitempty"` // generated passphrase length
	Punct  *bool   `json:"punct,omitempty"`  // use punctuation in passphrases
	Salt   *string `json:"salt,omitempty"`   // salt for passphrase generation
}

func (KeyGenRequest) DisallowUnknownFields() {}

// A KeyGenReply is the response from the key generator.
type KeyGenReply struct {
	// If the key was copied, the "key" field will be omitted.
	Key   string `json:"key,omitempty"`
	Hash  string `json:"hash"`
	Label string `json:"label"`
}

func (KeyGenReply) DisallowUnknownFields() {}

// A SiteRequest is a request for site data.
type SiteRequest struct {
	Host   string `json:"host,omitempty"`
	Strict bool   `json:"strict,omitempty"`
	Full   bool   `json:"full,omitempty"`
}

func (SiteRequest) DisallowUnknownFields() {}

// An EditRequest is a request to edit the contents of a file.
type EditRequest struct {
	// The base name of the file to edit.
	Name string `json:"name,omitempty"`

	// The current contents of the file.
	Content []byte `json:"content,omitempty"`
}

func (EditRequest) DisallowUnknownFields() {}

// An EditNotesRequest is a request to edit the contents of a notes file.
type EditNotesRequest struct {
	// The name tag of the notes file to edit.
	Tag string `json:"tag,omitempty"`

	// An optional note category, e.g., "meetings".
	Category string `json:"category,omitempty"`

	// Which version of the notes to edit. If it is "new", a new version is
	// created for this base name.  If it is "" or "latest", the latest
	// matching version for this base name is edited.  Otherwise, this must be
	// a date in YYYY-MM-DD format.
	Version string `json:"version,omitempty"`

	// If true, return as soon as the editor starts rather than waiting for it
	// to terminate.
	Background bool `json:"background,omitempty"`
}

func (EditNotesRequest) DisallowUnknownFields() {}

// A ListNotesRequest is a request to list the available notes.
type ListNotesRequest struct {
	// List files matching this name tag (match all, if empty)
	Tag string `json:"tag,omitempty"`

	// List files in this note category, e.g., "meetings".  If empty, list files
	// in all categories.
	Category string `json:"category,omitempty"`

	// List files matching this version (globs OK, e.g., "2018-11-*").
	Version string `json:"version,omitempty"`
}

func (ListNotesRequest) DisallowUnknownFields() {}

// A Note describes an editable note.
type Note struct {
	Tag      string `json:"tag,omitempty"`
	Version  string `json:"version,omitempty"`
	Suffix   string `json:"suffix,omitempty"`
	Category string `json:"category,omitempty"`
	Path     string `json:"path,omitempty"`
}

// A NoteWithText reports a note and its content.
type NoteWithText struct {
	*Note
	Text []byte `json:"text"`
}

func (NoteWithText) DisallowUnknownFields() {}

// NoteLess reports whether a should be ordered prior to b, first by tag and
// then by version.
func NoteLess(a, b *Note) bool {
	if a.Tag == b.Tag {
		if a.Version == b.Version {
			return a.Category < b.Category
		}
		return a.Version < b.Version
	}
	return a.Tag < b.Tag
}

// Columns calls write with a tabwriter directed to w, and flushes its output
// when write returns.
func Columns(w io.Writer, write func(io.Writer)) {
	tw := tabwriter.NewWriter(w, 0, 8, 1, ' ', 0)
	defer tw.Flush()
	write(tw)
}
