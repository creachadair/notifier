// Package notifier contains common data structures for notifications.
package notifier

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"bitbucket.org/creachadair/jrpc2"
	"bitbucket.org/creachadair/jrpc2/channel"
	"bitbucket.org/creachadair/jrpc2/code"
	"bitbucket.org/creachadair/jrpc2/jauth"
	"bitbucket.org/creachadair/jrpc2/jctx"
)

var (
	serverAddr = flag.String("server", os.Getenv("NOTIFIER_ADDR"), "Server address")
	authUser   = flag.String("authuser", os.Getenv("NOTIFIER_USER"), "Username for authorization")
	authKey    = os.Getenv("NOTIFIER_KEY")
)

// Dial connects to the flag-selected JSON-RPC server and returns a context and
// a client ready for use. The caller is responsible for closing the client.
func Dial(ctx context.Context) (context.Context, *jrpc2.Client, error) {
	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		return ctx, nil, fmt.Errorf("address %q: %v", *serverAddr, err)
	}
	cli := jrpc2.NewClient(channel.RawJSON(conn, conn), &jrpc2.ClientOptions{
		EncodeContext: jctx.Encode,
	})
	if *authUser != "" && authKey != "" {
		ctx = jctx.WithAuthorizer(ctx, jauth.User{
			Name: *authUser,
			Key:  []byte(authKey),
		}.Token)
	}
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

// A ClipSetRequest is sent to update the contents of the clipboard.
type ClipSetRequest struct {
	Data       []byte `json:"data"`           // the data to be stored
	Tag        string `json:"tag,omitempty"`  // the tag to assign the data
	Save       string `json:"save,omitempty"` // save active clip to this tag
	AllowEmpty bool   `json:"allowEmpty"`     // allow data to be empty
}

// A ClipGetRequest is sent to query the contents of the clipboard.
type ClipGetRequest struct {
	Tag      string `json:"tag,omitempty"`      // the tag to assign the data
	Save     string `json:"save,omitempty"`     // save active clip to this tag
	Activate bool   `json:"activate,omitempty"` // make this clip active
}

// A ClipClearRequest is sent to clear the contents of the clipboard.
type ClipClearRequest struct {
	Tag string `json:"tag,omitempty"` // the tag to clear or remove
}

// A SayRequest is a request to speak a notification to the user.
type SayRequest struct {
	Text  string        `json:"text"`
	Voice string        `json:"voice,omitempty"`
	After time.Duration `json:"after,omitempty"`
}

// A TextRequest is a request to read a string from the user.
type TextRequest struct {
	Prompt  string `json:"prompt,omitempty"`
	Default string `json:"default,omitempty"`
	Hide    bool   `json:"hide,omitempty"`
}

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

// A KeyGenReply is the response from the key generator.
type KeyGenReply struct {
	// If the key was copied, the "key" field will be omitted.
	Key   string `json:"key,omitempty"`
	Hash  string `json:"hash"`
	Label string `json:"label"`
}

// A SiteRequest is a request for site data.
type SiteRequest struct {
	Host string `json:"host,omitempty"`
	Full bool   `json:"full,omitempty"`
}

// An EditRequest is a request to edit the contents of a file.
type EditRequest struct {
	// The base name of the file to edit.
	Name string `json:"name,omitempty"`

	// The current contents of the file.
	Content []byte `json:"content,omitempty"`
}

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
}

// A ListNotesRequest is a request to list the available notes.
type ListNotesRequest struct {
	// List files matching this name tag (match all, if empty)
	Tag string `json:"tag,omitempty"`

	// List files in this note category, e.g., "meetings".
	Category string `json:"category,omitempty"`
}

// A Note describes an editable note.
type Note struct {
	Tag     string `json:"tag,omitempty"`
	Version string `json:"version,omitempty"`
}

// NoteLess reports whether a should be ordered prior to b, first by tag and
// then by version.
func NoteLess(a, b *Note) bool {
	if a.Tag == b.Tag {
		return a.Version < b.Version
	}
	return a.Tag < b.Tag
}
