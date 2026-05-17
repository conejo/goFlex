// serve.go — HTTP server bootstrap and template rendering.

package web

import (
	"bytes"
	"embed"
	"html/template"
	"log"
	"net/http"

	"goFlex/config"
)

//go:embed templates/*
var templateFS embed.FS

var (
	tmpl      *template.Template
	pageTmpls = make(map[string]*template.Template)
)

func init() {
	funcMap := template.FuncMap{
		"not": func(b bool) bool { return !b },
	}

	// Parse base and fragments into a shared set.
	tmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS,
		"templates/base.html",
		"templates/log.html",
		"templates/slices.html",
		"templates/status.html",
		"templates/subs.html",
	))

	// Each page template defines a {{block "content"}} that would collide
	// if parsed into the same set. Clone the fragment set and parse the page
	// template into its own independent set.
	for _, name := range []string{"connected.html", "discovery.html"} {
		clone, err := tmpl.Clone()
		if err != nil {
			log.Fatalf("template clone error: %v", err)
		}
		_, err = clone.ParseFS(templateFS, "templates/"+name)
		if err != nil {
			log.Fatalf("template parse error (%s): %v", name, err)
		}
		pageTmpls[name] = clone
	}
}

// render executes the named template and writes to w.
func render(w http.ResponseWriter, name string, data templateData) {
	var t *template.Template
	if pt, ok := pageTmpls[name]; ok {
		t = pt
	} else {
		t = tmpl
	}
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template error (%s): %v", name, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// renderToString renders a template to a string (used by SSE).
func renderToString(name string, data templateData) string {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("template error (%s): %v", name, err)
		return ""
	}
	return buf.String()
}

// Serve starts the HTTP server with the given config.
func Serve(cfg *config.Config) error {
	hub := NewHub(cfg)

	mux := http.NewServeMux()

	// Page
	mux.HandleFunc("/", hub.handleIndex)

	// Fragments (HTMX polling)
	mux.HandleFunc("/discovery", hub.handleDiscovery)
	mux.HandleFunc("/log", hub.handleLog)
	mux.HandleFunc("/slices", hub.handleSlices)
	mux.HandleFunc("/status", hub.handleStatus)
	mux.HandleFunc("/subs", hub.handleSubsPanel)

	// Actions
	mux.HandleFunc("/connect", hub.handleConnect)
	mux.HandleFunc("/disconnect", hub.handleDisconnect)
	mux.HandleFunc("/subscribe", hub.handleSubscribe)
	mux.HandleFunc("/tune", hub.handleTune)
	mux.HandleFunc("/command", hub.handleCommand)

	// SSE
	mux.HandleFunc("/events", hub.handleEvents)

	addr := ":8080"
	log.Printf("goFlex web UI starting on http://localhost%s", addr)
	log.Printf("Radio: %s:%d", cfg.RadioAddress, cfg.RadioPort)

	// Clean shutdown on interrupt is handled by the OS; the radio conn
	// will be closed when the process exits.
	return http.ListenAndServe(addr, mux)
}
