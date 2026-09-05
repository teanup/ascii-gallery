package ascii

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// failWriter fails on the failAt-th Write, then allows subsequent writes.
// This terminates the infinite RenderLoop while letting the final
// cursor-show sequence be written.
type failWriter struct {
	buf    bytes.Buffer
	writes int
	failAt int
}

func (w *failWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		return 0, errTestEnd
	}
	return w.buf.Write(p)
}

var errTestEnd = errors.New("test end")

func TestRenderLoop(t *testing.T) {
	frames := []string{
		"frame one\n",
		"frame two\n",
	}
	// Compress the frames as RenderLoop expects encoded input.
	encoded := make([]string, len(frames))
	for i, f := range frames {
		c, err := compressString(f)
		if err != nil {
			t.Fatalf("compress frame: %v", err)
		}
		encoded[i] = c
	}

	// failAt=5: hide-cursor, frame1, cursorUp, deleteLines, then frame2 fails.
	w := &failWriter{failAt: 5}
	RenderLoop(w, encoded, 1, 2)

	out := w.buf.String()
	if !strings.HasPrefix(out, ansiHideCursor) {
		t.Error("expected cursor-hide at start")
	}
	if !strings.Contains(out, "frame one") {
		t.Error("expected first frame content")
	}
	if !strings.Contains(out, "\033[2F") {
		t.Error("expected cursor-up sequence for height 2")
	}
	if !strings.Contains(out, "\033[2M") {
		t.Error("expected delete-lines sequence for height 2")
	}
	if !strings.HasSuffix(out, ansiShowCursor) {
		t.Error("expected cursor-show at end")
	}
}

func TestRenderLoopStopsOnBadFrame(t *testing.T) {
	// An invalid encoded frame should stop the loop without writing cursor-show.
	w := &failWriter{failAt: 100}
	RenderLoop(w, []string{"not valid base64!!!"}, 1, 2)
	if w.buf.Len() == 0 {
		t.Error("expected at least the cursor-hide sequence")
	}
	if strings.Contains(w.buf.String(), ansiShowCursor) {
		t.Error("did not expect cursor-show when stopping on bad frame")
	}
}
