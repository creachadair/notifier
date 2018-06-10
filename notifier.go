// Package notifier contains common data structures for notifications.
package notifier

import (
	"time"

	"bitbucket.org/creachadair/jrpc2/code"
)

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
	Host   string  `json:"host,omitempty"`
	Copy   bool    `json:"copy,omitempty"`
	Format *string `json:"format,omitempty"`
	Length *int    `json:"length,omitempty"`
	Punct  *bool   `json:"punct,omitempty"`
	Salt   *string `json:"salt,omitempty"`
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
