// Package notifier contains common data structures for notifications.
package notifier

import "bitbucket.org/creachadair/jrpc2"

// A PostRequest is a request to post a notification to the user.
type PostRequest struct {
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
	Body     string `json:"body,omitempty"`
	Audible  bool   `json:"audible,omitempty"`
}

// A ClipRequest is sent to update the contents of the clipboard.
type ClipRequest struct {
	Data       []byte `json:"data"`
	AllowEmpty bool   `json:"allowEmpty"`
}

// A SayRequest is a request to speak a notification to the user.
type SayRequest struct {
	Text string `json:"text"`
}

// A TextRequest is a request to read a string from the user.
type TextRequest struct {
	Prompt  string `json:"prompt,omitempty"`
	Default string `json:"default,omitempty"`
	Hide    bool   `json:"hide,omitempty"`
}

// E_UserCancelled is the code returned when a user cancels a text request.
var E_UserCancelled = jrpc2.RegisterCode(-29999, "user cancelled request")
