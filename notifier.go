// Package notifier contains common data structures for notifications.
package notifier

// A PostRequest is a request to post a notification to the user.
type PostRequest struct {
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
	Body     string `json:"body,omitempty"`
	Audible  bool   `json:"audible,omitempty"`
}

// A ClipRequest is sent to update the contents of the clipboard.
type ClipRequest struct {
	Data []byte `json:"data"`
}
