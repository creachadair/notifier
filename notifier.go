// Package notifier contains common data structures for notifications.
package notifier

// A PostRequest is sent by postnote to a noteserver.
type PostRequest struct {
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
	Body     string `json:"body,omitempty"`
	Audible  bool   `json:"audible,omitempty"`
}
