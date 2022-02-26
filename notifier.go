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
)

var (
	serverAddr = os.Getenv("NOTIFIER_ADDR") // see RegisterFlags

	debug jrpc2.Logger
)

func init() {
	switch os.Getenv("NOTIFIER_DEBUG") {
	case "1", "t", "true", "yes", "on":
		debug = jrpc2.StdLogger(log.New(os.Stderr, "[client:debug] ", log.Lmicroseconds))
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
	// Dial the server: host:port is tcp, otherwise a Unix socket.
	atype, addr := jrpc2.Network(serverAddr)
	if atype == "unix" {
		addr = os.ExpandEnv(addr)
	}
	conn, err := net.Dial(atype, addr)
	if err != nil {
		return ctx, nil, fmt.Errorf("address %q: %v", addr, err)
	}

	cli := jrpc2.NewClient(channel.Line(conn, conn), &jrpc2.ClientOptions{
		Logger: debug,
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

// An EditRequest is a request to edit the contents of a file.
type EditRequest struct {
	// The base name of the file to edit.
	Name string `json:"name,omitempty"`

	// The current contents of the file.
	Content []byte `json:"content,omitempty"`
}

func (EditRequest) DisallowUnknownFields() {}

// Columns calls write with a tabwriter directed to w, and flushes its output
// when write returns.
func Columns(w io.Writer, write func(io.Writer)) {
	tw := tabwriter.NewWriter(w, 0, 8, 1, ' ', 0)
	defer tw.Flush()
	write(tw)
}
