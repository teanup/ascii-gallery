package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/teanup/ascii-gallery/model"
	"github.com/teanup/ascii-gallery/store"
)

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Hello World", "helloworld"},
		{"My-Animation_2", "my-animation_2"},
		{"  spaces  ", "spaces"},
		{"---leading---", "leading"},
		{"trailing---", "trailing"},
		{"UPPER", "upper"},
		{"", ""},
		{"!!!", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := sanitizeID(tt.in); got != tt.want {
				t.Errorf("sanitizeID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeText(t *testing.T) {
	// Control characters (including tab) are stripped.
	if got := sanitizeText("  hello\tworld  ", 128); got != "helloworld" {
		t.Errorf("sanitizeText = %q, want %q", got, "helloworld")
	}
	if got := sanitizeText("a\x00b\x1fc", 128); got != "abc" {
		t.Errorf("sanitizeText strips control chars = %q, want %q", got, "abc")
	}
	long := strings.Repeat("x", 200)
	if got := sanitizeText(long, 128); len(got) != 128 {
		t.Errorf("sanitizeText length = %d, want 128", len(got))
	}
}

func TestGIFDimensions(t *testing.T) {
	// GIF89a header with width=320, height=200.
	header := []byte("GIF89a" + string([]byte{0x40, 0x01, 0xC8, 0x00}))
	w, h, err := gifDimensions(bytes.NewReader(header))
	if err != nil {
		t.Fatalf("gifDimensions: %v", err)
	}
	if w != 320 || h != 200 {
		t.Errorf("gifDimensions = (%d,%d), want (320,200)", w, h)
	}
}

func TestGIFDimensionsInvalid(t *testing.T) {
	if _, _, err := gifDimensions(bytes.NewReader([]byte("notagif"))); err == nil {
		t.Error("expected error for invalid gif signature")
	}
	if _, _, err := gifDimensions(bytes.NewReader([]byte("GIF89a"))); err == nil {
		t.Error("expected error for truncated header")
	}
}

// newTestServer builds a handler with a temp store and returns the mux.
func newTestServer(t *testing.T) *http.ServeMux {
	t.Helper()
	s, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	animHandler := NewAnimationsHandler(s, "http://localhost:8080")
	webHandler := NewWebHandler(s, "http://localhost:8080")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", webHandler.ServeHome)
	mux.HandleFunc("GET /anim", animHandler.ListAnimations)
	mux.HandleFunc("GET /anim/{id}", animHandler.GetAnimation)
	mux.HandleFunc("POST /anim", animHandler.CreateAnimation)
	mux.HandleFunc("PUT /anim/{id}", animHandler.UpdateAnimation)
	mux.HandleFunc("DELETE /anim/{id}", animHandler.DeleteAnimation)
	return mux
}

// uploadGIF performs a multipart POST/PUT with the given gif bytes.
func uploadGIF(t *testing.T, mux *http.ServeMux, method, path string, gifData []byte, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return uploadGIFAs(t, mux, method, path, gifData, fields, "")
}

// uploadGIFCurl is uploadGIF with a curl User-Agent.
func uploadGIFCurl(t *testing.T, mux *http.ServeMux, method, path string, gifData []byte, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return uploadGIFAs(t, mux, method, path, gifData, fields, "curl/8.0")
}

// uploadGIFAs performs a multipart POST/PUT, optionally setting a User-Agent.
func uploadGIFAs(t *testing.T, mux *http.ServeMux, method, path string, gifData []byte, fields map[string]string, userAgent string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "test.gif")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(gifData); err != nil {
		t.Fatalf("write gif: %v", err)
	}
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("WriteField %s: %v", k, err)
		}
	}
	mw.Close()

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// get performs a GET request with the given User-Agent.
func get(t *testing.T, mux *http.ServeMux, path, userAgent string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// del performs a DELETE request with the given User-Agent.
func del(t *testing.T, mux *http.ServeMux, path, userAgent string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// testGIF returns a minimal valid 1-frame white GIF.
func testGIF(t *testing.T) []byte {
	t.Helper()
	img := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{
		color.RGBA{0, 0, 0, 255},
		color.RGBA{255, 255, 255, 255},
	})
	img.SetColorIndex(0, 0, 1) // white pixel
	g := &gif.GIF{
		Image: []*image.Paletted{img},
		Delay: []int{10},
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatalf("encode test gif: %v", err)
	}
	return buf.Bytes()
}

func TestCreateAndGetAnimation(t *testing.T) {
	mux := newTestServer(t)
	gifData := testGIF(t)

	rec := uploadGIF(t, mux, http.MethodPost, "/anim", gifData, map[string]string{
		"id":   "myanim",
		"name": "My Animation",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateAnimation status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var anim model.Animation
	if err := json.Unmarshal(rec.Body.Bytes(), &anim); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if anim.ID != "myanim" || anim.Name != "My Animation" {
		t.Errorf("created animation = %+v", anim)
	}
	if len(anim.Frames) == 0 {
		t.Error("expected at least one frame")
	}

	// Get metadata as JSON (non-curl UA).
	req := httptest.NewRequest(http.MethodGet, "/anim/myanim", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("GetAnimation status = %d, want %d", rec2.Code, http.StatusOK)
	}
	var meta map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta["id"] != "myanim" {
		t.Errorf("metadata id = %v, want myanim", meta["id"])
	}
}

// failingWriter fails on the failAt-th Write call, then allows subsequent
// writes to succeed. This terminates the infinite RenderLoop while still
// letting the final cursor-show sequence be written.
type failingWriter struct {
	buf    bytes.Buffer
	writes int
	failAt int
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		return 0, errStreamEnd
	}
	return w.buf.Write(p)
}

var errStreamEnd = errors.New("stream end")

// failingResponseWriter is an http.ResponseWriter whose body writes fail once,
// terminating the infinite RenderLoop.
type failingResponseWriter struct {
	header http.Header
	writer *failingWriter
	status int
}

func (w *failingResponseWriter) Header() http.Header         { return w.header }
func (w *failingResponseWriter) WriteHeader(status int)      { w.status = status }
func (w *failingResponseWriter) Write(p []byte) (int, error) { return w.writer.Write(p) }

func TestGetAnimationStreamsForCurl(t *testing.T) {
	mux := newTestServer(t)
	uploadGIF(t, mux, http.MethodPost, "/anim", testGIF(t), map[string]string{"id": "stream"})

	req := httptest.NewRequest(http.MethodGet, "/anim/stream", nil)
	req.Header.Set("User-Agent", "curl/8.0")
	fw := &failingWriter{failAt: 5}
	rec := &failingResponseWriter{header: make(http.Header), writer: fw}
	mux.ServeHTTP(rec, req)

	if rec.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.status, http.StatusOK)
	}
	body := fw.buf.String()
	if !strings.Contains(body, "\033[?25l") {
		t.Error("expected cursor-hide ANSI sequence in stream")
	}
	if !strings.Contains(body, "\033[?25h") {
		t.Error("expected cursor-show ANSI sequence at end of stream")
	}
}

func TestListAnimations(t *testing.T) {
	mux := newTestServer(t)
	gifData := testGIF(t)
	uploadGIF(t, mux, http.MethodPost, "/anim", gifData, map[string]string{"id": "alpha"})
	uploadGIF(t, mux, http.MethodPost, "/anim", gifData, map[string]string{"id": "beta"})

	rec := get(t, mux, "/anim", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list length = %d, want 2", len(list))
	}
	if list[0]["id"] != "alpha" || list[1]["id"] != "beta" {
		t.Errorf("list order = %v, want [alpha beta]", list)
	}
}

func TestUpdateAnimation(t *testing.T) {
	mux := newTestServer(t)
	gifData := testGIF(t)
	uploadGIF(t, mux, http.MethodPost, "/anim", gifData, map[string]string{"id": "upd", "name": "Old"})

	rec := uploadGIF(t, mux, http.MethodPut, "/anim/upd", gifData, map[string]string{
		"name":        "New",
		"description": "A description",
		"width":       "30",
		"height":      "15",
		"max_frames":  "3",
		"min_delay":   "200",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var anim model.Animation
	if err := json.Unmarshal(rec.Body.Bytes(), &anim); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if anim.ID != "upd" || anim.Name != "New" || anim.Description != "A description" {
		t.Errorf("updated animation = %+v", anim)
	}
	if anim.Width != 30 || anim.Height != 15 || anim.Delay != 200 {
		t.Errorf("options not applied: %+v", anim)
	}
}

func TestUpdateAnimationMissing(t *testing.T) {
	mux := newTestServer(t)
	rec := uploadGIF(t, mux, http.MethodPut, "/anim/nope", testGIF(t), nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestDeleteAnimation(t *testing.T) {
	mux := newTestServer(t)
	uploadGIF(t, mux, http.MethodPost, "/anim", testGIF(t), map[string]string{"id": "del"})

	if rec := del(t, mux, "/anim/del", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	// Deleting again should 404.
	if rec := del(t, mux, "/anim/del", ""); rec.Code != http.StatusNotFound {
		t.Errorf("second delete status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestDeleteAnimationInvalidID(t *testing.T) {
	mux := newTestServer(t)
	if rec := del(t, mux, "/anim/!!!", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateAnimationMissingFile(t *testing.T) {
	mux := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/anim", strings.NewReader(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateAnimationInvalidGIF(t *testing.T) {
	mux := newTestServer(t)
	rec := uploadGIF(t, mux, http.MethodPost, "/anim", []byte("not a gif"), nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestCreateAnimationInvalidID(t *testing.T) {
	mux := newTestServer(t)
	// id with no alphanumeric characters is rejected.
	rec := uploadGIF(t, mux, http.MethodPost, "/anim", testGIF(t), map[string]string{"id": "!!!"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateAnimationOptions(t *testing.T) {
	mux := newTestServer(t)
	rec := uploadGIF(t, mux, http.MethodPost, "/anim", testGIF(t), map[string]string{
		"id":         "opt",
		"width":      "40",
		"height":     "20",
		"max_frames": "5",
		"min_delay":  "100",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var anim model.Animation
	if err := json.Unmarshal(rec.Body.Bytes(), &anim); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if anim.Width != 40 || anim.Height != 20 || anim.Delay != 100 {
		t.Errorf("options not applied: %+v", anim)
	}
}

func TestCreateAnimationAutoID(t *testing.T) {
	mux := newTestServer(t)
	// No id provided: derived from the uploaded filename "test.gif".
	rec := uploadGIF(t, mux, http.MethodPost, "/anim", testGIF(t), nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var anim model.Animation
	if err := json.Unmarshal(rec.Body.Bytes(), &anim); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if anim.ID != "test" {
		t.Errorf("auto id = %q, want test", anim.ID)
	}
}

func TestUniqueID(t *testing.T) {
	s, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	h := NewAnimationsHandler(s, "http://localhost:8080")

	// First use returns the base.
	if got := h.uniqueID("base"); got != "base" {
		t.Errorf("uniqueID first = %q, want base", got)
	}
	// After saving base, next returns base-1.
	if err := s.Save(&model.Animation{ID: "base"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := h.uniqueID("base"); got != "base-1" {
		t.Errorf("uniqueID second = %q, want base-1", got)
	}
}

func TestServeHome(t *testing.T) {
	mux := newTestServer(t)
	uploadGIF(t, mux, http.MethodPost, "/anim", testGIF(t), map[string]string{"id": "home", "name": "Home Anim"})

	rec := get(t, mux, "/", "Mozilla/5.0")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "Home Anim") {
		t.Error("expected animation name in home page")
	}
}

func TestServeHomeCurl(t *testing.T) {
	mux := newTestServer(t)
	rec := get(t, mux, "/", "curl/8.0")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "List all animations") {
		t.Error("expected curl help text")
	}
}

func TestGetAnimationInvalidID(t *testing.T) {
	mux := newTestServer(t)
	if rec := get(t, mux, "/anim/!!!", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetAnimationNotFound(t *testing.T) {
	mux := newTestServer(t)
	if rec := get(t, mux, "/anim/missing", ""); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestListAnimationsCurl(t *testing.T) {
	mux := newTestServer(t)
	uploadGIF(t, mux, http.MethodPost, "/anim", testGIF(t), map[string]string{
		"id":          "alpha",
		"name":        "Alpha",
		"description": "The first one",
	})

	rec := get(t, mux, "/anim", "curl/8.0")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"Alpha", "alpha", "The first one", "curl http://localhost:8080/anim/alpha"} {
		if !strings.Contains(body, want) {
			t.Errorf("curl list missing %q; body:\n%s", want, body)
		}
	}
}

func TestListAnimationsCurlEmpty(t *testing.T) {
	mux := newTestServer(t)
	rec := get(t, mux, "/anim", "curl/8.0")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "No animations yet") {
		t.Errorf("expected empty-list hint; body:\n%s", rec.Body.String())
	}
}

func TestCreateAnimationCurl(t *testing.T) {
	mux := newTestServer(t)
	rec := uploadGIFCurl(t, mux, http.MethodPost, "/anim", testGIF(t), map[string]string{
		"id":   "created",
		"name": "Created",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Created") || !strings.Contains(body, "curl http://localhost:8080/anim/created") {
		t.Errorf("unexpected curl create response:\n%s", body)
	}
}

func TestUpdateAnimationCurl(t *testing.T) {
	mux := newTestServer(t)
	uploadGIF(t, mux, http.MethodPost, "/anim", testGIF(t), map[string]string{"id": "updc"})

	rec := uploadGIFCurl(t, mux, http.MethodPut, "/anim/updc", testGIF(t), map[string]string{"name": "Updated"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Updated") {
		t.Errorf("unexpected curl update response:\n%s", rec.Body.String())
	}
}

func TestDeleteAnimationCurl(t *testing.T) {
	mux := newTestServer(t)
	uploadGIF(t, mux, http.MethodPost, "/anim", testGIF(t), map[string]string{"id": "delc"})

	rec := del(t, mux, "/anim/delc", "curl/8.0")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "Deleted") {
		t.Errorf("unexpected curl delete response:\n%s", rec.Body.String())
	}
}

// TestErrorFormatting checks that errors are styled for curl and plain for
// other clients.
func TestErrorFormatting(t *testing.T) {
	mux := newTestServer(t)

	curlRec := get(t, mux, "/anim/missing", "curl/8.0")
	if curlRec.Code != http.StatusNotFound {
		t.Fatalf("curl status = %d, want %d", curlRec.Code, http.StatusNotFound)
	}
	if !strings.Contains(curlRec.Body.String(), "Error:") {
		t.Errorf("expected styled error prefix; body:\n%s", curlRec.Body.String())
	}

	plainRec := get(t, mux, "/anim/missing", "Mozilla/5.0")
	if plainRec.Code != http.StatusNotFound {
		t.Fatalf("plain status = %d, want %d", plainRec.Code, http.StatusNotFound)
	}
	if strings.Contains(plainRec.Body.String(), "\033[") {
		t.Errorf("non-curl error should not contain ANSI escapes; body:\n%s", plainRec.Body.String())
	}
}
