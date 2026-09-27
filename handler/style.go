package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/teanup/ascii-gallery/model"
)

// ANSI sequences used to style curl-facing text output.
const (
	styleReset = "\033[0m"

	styleTitle    = "\033[38;5;141;1m" // bold purple
	styleSubtitle = "\033[38;5;141m"   // purple
	styleCommand  = "\033[38;5;147m"   // blue
	styleMeta     = "\033[38;5;244m"   // gray
	styleSuccess  = "\033[38;5;34m"    // green
	styleError    = "\033[38;5;160;1m" // bold red
)

// isCurl reports whether the request comes from curl.
func isCurl(ua string) bool {
	return strings.HasPrefix(ua, "curl/")
}

// paint wraps s in an style sequence, resetting afterwards.
func paint(s, code string) string {
	return code + s + styleReset
}

// title renders a heading.
func title(s string) string { return paint(s, styleTitle) }

// subtitle renders secondary text.
func subtitle(s string) string { return paint(s, styleSubtitle) }

// success renders a positive outcome keyword.
func success(s string) string { return paint(s, styleSuccess) }

// command renders a runnable shell command.
func command(s string) string { return paint(s, styleCommand) }

// errorLine renders an error message.
func errorLine(msg string) string {
	return paint("Error:", styleError) + " " + msg + "\n"
}

// animShort renders an animation reference as "name (id)".
func animShort(anim *model.Animation) string {
	return title(anim.Name) + " (" + anim.ID + ")"
}

// animLong renders an animation with its metadata and play command.
func animLong(anim *model.Animation, baseURL string) string {
	meta := fmt.Sprintf("%dx%d, %d frames, %dms", anim.Width, anim.Height, len(anim.Frames), anim.Delay)
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", animShort(anim))
	fmt.Fprintf(&b, "  %s\n", paint(meta, styleMeta))
	if anim.Description != "" {
		fmt.Fprintf(&b, "  %s\n", anim.Description)
	}
	fmt.Fprintf(&b, "  %s\n", command("curl "+baseURL+"/anim/"+anim.ID))
	return b.String()
}

// writeText writes a plain-text response with the given status.
func writeText(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprint(w, body)
}

// writeError writes a styled error message for curl clients.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeText(w, status, errorLine(msg))
}
