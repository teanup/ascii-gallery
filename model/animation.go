package model

// Animation represents a GIF converted to ASCII art frames.
// It is serialized as JSON for persistent storage.
// Delay is in milliseconds.
type Animation struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	Delay       int      `json:"delay"`
	Frames      []string `json:"frames"`
}
