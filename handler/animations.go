package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/teanup/ascii-gallery/ascii"
	"github.com/teanup/ascii-gallery/model"
	"github.com/teanup/ascii-gallery/store"
)

const maxUploadSize = 10 << 20 // 10 MiB

// AnimationsHandler exposes REST endpoints for animation CRUD.
type AnimationsHandler struct {
	Store *store.Store
}

// NewAnimationsHandler creates an AnimationsHandler.
func NewAnimationsHandler(s *store.Store) *AnimationsHandler {
	return &AnimationsHandler{Store: s}
}

// ListAnimations returns a JSON array of animation summaries.
func (h *AnimationsHandler) ListAnimations(w http.ResponseWriter, r *http.Request) {
	ids, err := h.Store.List()
	if err != nil {
		http.Error(w, "failed to list animations", http.StatusInternalServerError)
		return
	}

	type summary struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		Delay       int    `json:"delay"`
	}

	summaries := make([]summary, 0, len(ids))
	for _, id := range ids {
		anim, err := h.Store.Load(id)
		if err != nil {
			continue
		}
		summaries = append(summaries, summary{
			ID:          anim.ID,
			Name:        anim.Name,
			Description: anim.Description,
			Width:       anim.Width,
			Height:      anim.Height,
			Delay:       anim.Delay,
		})
	}

	writeJSON(w, http.StatusOK, summaries)
}

// GetAnimation streams the ASCII animation for curl clients, or returns JSON metadata.
func (h *AnimationsHandler) GetAnimation(w http.ResponseWriter, r *http.Request) {
	id := sanitizeID(r.PathValue("id"))
	if id == "" {
		http.Error(w, "invalid animation id", http.StatusBadRequest)
		return
	}

	anim, err := h.Store.Load(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Check if the client is curl.
	ua := r.UserAgent()
	if strings.HasPrefix(ua, "curl/") {
		streamAnimation(w, anim)
		return
	}

	// Return metadata as JSON (without the frame data).
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":          anim.ID,
		"name":        anim.Name,
		"description": anim.Description,
		"width":       anim.Width,
		"height":      anim.Height,
		"delay":       anim.Delay,
		"frames":      len(anim.Frames),
	})
}

// streamAnimation writes the animation as an infinite ASCII stream.
func streamAnimation(w http.ResponseWriter, anim *model.Animation) {
	// Set headers for streaming.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	ascii.RenderLoop(w, anim.Frames, anim.Delay, anim.Height)
}

// CreateAnimation uploads a GIF and stores it as an ASCII animation.
func (h *AnimationsHandler) CreateAnimation(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "file too large or invalid multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read the GIF data.
	gifData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusBadRequest)
		return
	}

	// Determine target dimensions.
	userWidth, _ := strconv.Atoi(r.FormValue("width"))
	userHeight, _ := strconv.Atoi(r.FormValue("height"))

	// Read GIF header to get source dimensions.
	srcW, srcH, err := gifDimensions(bytes.NewReader(gifData))
	if err != nil {
		http.Error(w, "failed to read gif dimensions", http.StatusUnprocessableEntity)
		return
	}

	targetWidth, targetHeight := ascii.ComputeDimensions(srcW, srcH, userWidth, userHeight)

	// Frame rate limits.
	maxFrames := 100
	if f := r.FormValue("max_frames"); f != "" {
		if v, err := strconv.Atoi(f); err == nil && v > 0 {
			maxFrames = v
		}
	}
	minDelayMs := 50
	if d := r.FormValue("min_delay"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			minDelayMs = v
		}
	}

	// Convert GIF to ASCII.
	anim, err := ascii.ConvertGIF(bytes.NewReader(gifData), targetWidth, targetHeight, maxFrames, minDelayMs)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to convert gif: %v", err), http.StatusUnprocessableEntity)
		return
	}

	// Set metadata from form fields or defaults.
	rawID := r.FormValue("id")
	if rawID == "" {
		rawID = sanitizeID(stripExt(header.Filename))
	} else {
		rawID = sanitizeID(rawID)
	}
	if rawID == "" {
		http.Error(w, "invalid id: must contain at least one alphanumeric character", http.StatusBadRequest)
		return
	}
	anim.ID = h.uniqueID(rawID)

	anim.Name = sanitizeText(r.FormValue("name"), 128)
	if anim.Name == "" {
		anim.Name = stripExt(header.Filename)
	}
	anim.Description = sanitizeText(r.FormValue("description"), 1024)

	// Save to store.
	if err := h.Store.Save(anim); err != nil {
		http.Error(w, "failed to save animation", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, anim)
}

// UpdateAnimation replaces an existing animation with a new GIF upload.
func (h *AnimationsHandler) UpdateAnimation(w http.ResponseWriter, r *http.Request) {
	id := sanitizeID(r.PathValue("id"))
	if id == "" {
		http.Error(w, "invalid animation id", http.StatusBadRequest)
		return
	}

	// Check that the animation exists.
	if _, err := h.Store.Load(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "file too large or invalid multipart form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	gifData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusBadRequest)
		return
	}

	userWidth, _ := strconv.Atoi(r.FormValue("width"))
	userHeight, _ := strconv.Atoi(r.FormValue("height"))

	srcW, srcH, err := gifDimensions(bytes.NewReader(gifData))
	if err != nil {
		http.Error(w, "failed to read gif dimensions", http.StatusUnprocessableEntity)
		return
	}

	targetWidth, targetHeight := ascii.ComputeDimensions(srcW, srcH, userWidth, userHeight)

	maxFrames := 100
	if f := r.FormValue("max_frames"); f != "" {
		if v, err := strconv.Atoi(f); err == nil && v > 0 {
			maxFrames = v
		}
	}
	minDelayMs := 50
	if d := r.FormValue("min_delay"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			minDelayMs = v
		}
	}

	anim, err := ascii.ConvertGIF(bytes.NewReader(gifData), targetWidth, targetHeight, maxFrames, minDelayMs)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to convert gif: %v", err), http.StatusUnprocessableEntity)
		return
	}

	// Preserve the original ID and update metadata from form.
	anim.ID = id
	if name := sanitizeText(r.FormValue("name"), 128); name != "" {
		anim.Name = name
	}
	if desc := sanitizeText(r.FormValue("description"), 1024); desc != "" {
		anim.Description = desc
	}

	if err := h.Store.Save(anim); err != nil {
		http.Error(w, "failed to save animation", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, anim)
}

// DeleteAnimation removes an animation.
func (h *AnimationsHandler) DeleteAnimation(w http.ResponseWriter, r *http.Request) {
	id := sanitizeID(r.PathValue("id"))
	if id == "" {
		http.Error(w, "invalid animation id", http.StatusBadRequest)
		return
	}

	if err := h.Store.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// nonIDChar matches anything that is NOT \w or dash.
var nonIDChar = regexp.MustCompile(`[^\w-]`)

// sanitizeID lowercases and keeps only \w and dash, then enforces \w([\w-]*\w)?.
func sanitizeID(s string) string {
	s = strings.ToLower(s)
	s = nonIDChar.ReplaceAllString(s, "")
	// Strip leading/trailing dashes.
	s = strings.Trim(s, "-")
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}

// uniqueID finds an unused ID by appending -N if the base is taken.
func (h *AnimationsHandler) uniqueID(base string) string {
	for suffix := 1; suffix < 1000; suffix++ {
		candidate := base
		if suffix > 1 {
			candidate = fmt.Sprintf("%s-%d", base, suffix-1)
		}
		if _, err := h.Store.Load(candidate); err != nil {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano())
}

// sanitizeText strips control characters and limits length.
// HTML-escaping is handled by the template engine.
func sanitizeText(s string, maxLen int) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 32 && r != 127 {
			b.WriteRune(r)
		}
	}
	s = strings.TrimSpace(b.String())
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

// stripExt returns the filename without its extension, sanitized to printable chars.
func stripExt(filename string) string {
	if idx := strings.LastIndex(filename, "."); idx > 0 {
		filename = filename[:idx]
	}
	// Strip any remaining non-printable chars.
	var b strings.Builder
	for _, r := range filename {
		if r >= 32 && r != 127 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// gifDimensions reads the GIF header to extract width and height without
// decoding the full image data.
func gifDimensions(r io.Reader) (width, height int, err error) {
	// Read the 10-byte GIF header: 6 bytes signature + 4 bytes dimensions
	header := make([]byte, 10)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, 0, fmt.Errorf("read gif header: %w", err)
	}

	// Validate GIF signature
	sig := string(header[:6])
	if sig != "GIF87a" && sig != "GIF89a" {
		return 0, 0, fmt.Errorf("not a valid gif: %s", sig)
	}

	// Little-endian 16-bit width and height
	width = int(header[6]) | int(header[7])<<8
	height = int(header[8]) | int(header[9])<<8
	return width, height, nil
}
