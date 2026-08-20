package ascii

import (
	"fmt"
	"io"
	"time"
)

const (
	ansiHideCursor = "\033[?25l"
	ansiShowCursor = "\033[?25h"
)

// RenderLoop streams an animation to w in an infinite loop.
// It hides the cursor, writes each frame, then remove the previous frame
// before writing the next one in place.
//
// The loop runs until w returns an error (e.g., client disconnect).
func RenderLoop(w io.Writer, frames []string, delay int, height int) {
	// Hide cursor on start.
	fmt.Fprint(w, ansiHideCursor)

	// Delay is in milliseconds.
	frameDuration := time.Duration(delay) * time.Millisecond
	cursorUp := fmt.Sprintf("\033[%dF", height)
	deleteLines := fmt.Sprintf("\033[%dM", height)

	for {
		for _, encoded := range frames {
			frame, err := DecompressString(encoded)
			if err != nil {
				return
			}

			// Write the frame.
			_, err = io.WriteString(w, frame)
			if err != nil {
				fmt.Fprint(w, ansiShowCursor)
				return
			}

			// Flush so the frame is visible before the sleep.
			if flusher, ok := w.(interface{ Flush() }); ok {
				flusher.Flush()
			}

			time.Sleep(frameDuration)

			// Move cursor up to the start of the frame and delete those lines.
			_, err = io.WriteString(w, cursorUp)
			if err != nil {
				fmt.Fprint(w, ansiShowCursor)
				return
			}
			_, err = io.WriteString(w, deleteLines)
			if err != nil {
				fmt.Fprint(w, ansiShowCursor)
				return
			}
		}
	}
}
