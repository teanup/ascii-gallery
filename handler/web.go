package handler

import (
	_ "embed"
	"html/template"
	"net/http"
	"strings"

	"github.com/teanup/ascii-gallery/store"
)

//go:embed template.html
var templateSource string

// pageData is passed to the HTML template.
type pageData struct {
	BaseURL    string
	Animations []animationSummary
}

type animationSummary struct {
	ID   string
	Name string
}

// WebHandler serves the browser-facing HTML page.
type WebHandler struct {
	Store   *store.Store
	BaseURL string
	tmpl    *template.Template
}

// NewWebHandler creates a WebHandler.
func NewWebHandler(s *store.Store, baseURL string) *WebHandler {
	tmpl := template.Must(template.New("page").Parse(templateSource))
	return &WebHandler{
		Store:   s,
		BaseURL: baseURL,
		tmpl:    tmpl,
	}
}

// ServeHome renders the HTML page for browsers, or curl help text.
func (h *WebHandler) ServeHome(w http.ResponseWriter, r *http.Request) {
	ua := r.UserAgent()
	if strings.HasPrefix(ua, "curl") {
		h.serveCurlHelp(w)
		return
	}

	ids, err := h.Store.List()
	if err != nil {
		ids = nil
	}

	anims := make([]animationSummary, 0, len(ids))
	for _, id := range ids {
		anim, err := h.Store.Load(id)
		if err != nil {
			continue
		}
		anims = append(anims, animationSummary{ID: anim.ID, Name: anim.Name})
	}

	data := pageData{
		BaseURL:    h.BaseURL,
		Animations: anims,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	h.tmpl.Execute(w, data)
}

// serveCurlHelp writes the curl usage help text.
func (h *WebHandler) serveCurlHelp(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	help := `ASCII Gallery - animated ASCII art server

Usage:
  List all animations:
    curl ` + h.BaseURL + `/anim

  View an animation (replace {id} with an animation ID):
    curl ` + h.BaseURL + `/anim/{id}

  Upload a new GIF:
    curl -F "file=@animation.gif" ` + h.BaseURL + `/anim

  Upload with custom options:
    curl -F "file=@animation.gif" -F "id=myid" -F "name=My Animation" -F "width=120" ` + h.BaseURL + `/anim

  Update an existing animation:
    curl -X PUT -F "file=@animation.gif" ` + h.BaseURL + `/anim/{id}

  Delete an animation:
    curl -X DELETE ` + h.BaseURL + `/anim/{id}

`
	w.Write([]byte(help))
}
