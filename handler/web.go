package handler

import (
	_ "embed" // Embed HTML template
	"html/template"
	"net/http"

	"github.com/teanup/ascii-gallery/model"
	"github.com/teanup/ascii-gallery/store"
)

//go:embed template.html
var templateSource string

// pageData is passed to the HTML template.
type pageData struct {
	BaseURL    string
	Animations []*model.Animation
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
	if isCurl(r.UserAgent()) {
		h.serveCurlHelp(w)
		return
	}

	anims, err := h.Store.LoadAll()
	if err != nil {
		anims = nil
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
	help := title("ASCII Gallery") + subtitle(" - animated ASCII art server") +
		"\n\nList all animations:\n  " +
		command("curl "+h.BaseURL+"/anim") +
		"\n\nView an animation (replace {id} with an ID):\n  " +
		command("curl "+h.BaseURL+"/anim/{id}") +
		"\n\nUpload a new GIF:\n  " +
		command("curl "+h.BaseURL+"/anim -F file=@animation.gif") +
		"\n\nUpload with custom options:\n  " +
		command("curl "+h.BaseURL+"/anim -F file=@animation.gif -F id=myid -F \"name=My Animation\" -F width=120") +
		"\n\nUpdate an existing animation:\n  " +
		command("curl "+h.BaseURL+"/anim/{id} -X PUT -F file=@animation.gif") +
		"\n\nDelete an animation:\n  " +
		command("curl "+h.BaseURL+"/anim/{id} -X DELETE") + "\n"
	writeText(w, http.StatusOK, help)
}
