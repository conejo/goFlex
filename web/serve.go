// serve.go — HTTP server bootstrap and template rendering.

package web

import (
	"bytes"
	"context"
	"embed"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goFlex/config"
)

//go:embed templates/*
//go:embed static/*
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

// NewMux returns an http.Handler with all routes wired to the given Hub.
func NewMux(hub *Hub) http.Handler {
	mux := http.NewServeMux()

	// Page
	mux.HandleFunc("GET /{$}", hub.handleIndex)

	// Fragments (SSE targets)
	mux.HandleFunc("GET /discovery", hub.handleDiscovery)
	mux.HandleFunc("GET /log", hub.handleLog)
	mux.HandleFunc("GET /slices", hub.handleSlices)
	mux.HandleFunc("GET /status", hub.handleStatus)
	mux.HandleFunc("GET /subs", hub.handleSubsPanel)

	// Actions
	mux.HandleFunc("POST /connect", hub.handleConnect)
	mux.HandleFunc("POST /disconnect", hub.handleDisconnect)
	mux.HandleFunc("POST /subscribe", hub.handleSubscribe)
	mux.HandleFunc("POST /tune", hub.handleTune)
	mux.HandleFunc("POST /command", hub.handleCommand)

	// SSE
	mux.HandleFunc("GET /events", hub.handleEvents)

	// Static assets
	mux.Handle("/static/", http.FileServer(http.FS(templateFS)))

	return mux
}

// Serve starts the HTTP server with the given config.
// It blocks until the server is shut down via interrupt signal.
func Serve(cfg *config.Config) error {
	hub := NewHub(cfg)

	addr := ":8080"
	log.Printf("goFlex web UI starting on http://localhost%s", addr)
	log.Printf("Radio: %s:%d", cfg.RadioAddress, cfg.RadioPort)

	return serveWithHub(hub, addr)
}

// serveWithHub starts an http.Server on the given address and handles
// graceful shutdown on SIGINT / SIGTERM.
func serveWithHub(hub *Hub, addr string) error {
	server := &http.Server{
		Addr:    addr,
		Handler: NewMux(hub),
	}

	// Graceful shutdown on interrupt.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh

		log.Println("shutting down web server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
		hub.Close()
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// ServeWithListener starts the server on an existing listener (useful for tests).
func ServeWithListener(hub *Hub, ln net.Listener) error {
	server := &http.Server{
		Handler: NewMux(hub),
	}
	return server.Serve(ln)
}
