// handlers.go — HTTP handlers for the HTMX web UI.

package web

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"goFlex/radio"
)

// ─── Page handlers ─────────────────────────────────────────────────────────

func (h *Hub) handleIndex(w http.ResponseWriter, r *http.Request) {
	if h.IsConnected() {
		render(w, "connected.html", h.templateData())
	} else {
		render(w, "discovery.html", h.templateData())
	}
}

// ─── Fragment handlers (HTMX polling targets) ──────────────────────────────

func (h *Hub) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	render(w, "discovery.html", h.templateData())
}

func (h *Hub) handleLog(w http.ResponseWriter, r *http.Request) {
	render(w, "log.html", h.templateData())
}

func (h *Hub) handleSlices(w http.ResponseWriter, r *http.Request) {
	render(w, "slices.html", h.templateData())
}

func (h *Hub) handleStatus(w http.ResponseWriter, r *http.Request) {
	render(w, "status.html", h.templateData())
}

func (h *Hub) handleSubsPanel(w http.ResponseWriter, r *http.Request) {
	render(w, "subs.html", h.templateData())
}

// ─── Action handlers ───────────────────────────────────────────────────────

func (h *Hub) handleConnect(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	addr := r.FormValue("addr")
	subNames := r.Form["subs"]

	// Run connect synchronously so the connection is established before the redirect.
	h.doConnect(addr, subNames)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Hub) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// Run disconnect synchronously so state is updated before the redirect.
	h.doDisconnect()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Hub) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	checked := r.FormValue("checked") == "true"

	h.SendCommand(Command{
		Kind: "subscribe",
		Name: name,
		Val:  fmt.Sprintf("%v", checked),
	})

	render(w, "subs.html", h.templateData())
}

func (h *Hub) handleTune(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	freq := r.FormValue("freq")
	if freq == "" {
		http.Error(w, "frequency required", http.StatusBadRequest)
		return
	}

	h.SendCommand(Command{Kind: "tune", Val: freq})
	render(w, "log.html", h.templateData())
}

func (h *Hub) handleCommand(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	cmd := r.FormValue("cmd")
	if cmd == "" {
		http.Error(w, "command required", http.StatusBadRequest)
		return
	}

	h.SendCommand(Command{Kind: "command", Val: cmd})
	render(w, "log.html", h.templateData())
}

// ─── SSE ───────────────────────────────────────────────────────────────────

func (h *Hub) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.Subscribe()
	defer h.Unsubscribe(ch)

	// Send initial state as pre-rendered HTML fragments.
	h.writeSSEFragment(w, flusher, "status", renderToString("status.html", h.templateData()))
	h.writeSSEFragment(w, flusher, "log", renderToString("log.html", h.templateData()))
	h.writeSSEFragment(w, flusher, "subs", renderToString("subs.html", h.templateData()))
	h.writeSSEFragment(w, flusher, "slices", renderToString("slices.html", h.templateData()))

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			switch evt.Kind {
			case "log":
				h.writeSSEFragment(w, flusher, "log", renderToString("log.html", h.templateData()))
			case "state":
				h.writeSSEFragment(w, flusher, "status", renderToString("status.html", h.templateData()))
				h.writeSSEFragment(w, flusher, "subs", renderToString("subs.html", h.templateData()))
			case "slices":
				h.writeSSEFragment(w, flusher, "slices", renderToString("slices.html", h.templateData()))
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Hub) writeSSEFragment(w io.Writer, flusher http.Flusher, event, html string) {
	fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range strings.Split(html, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
	flusher.Flush()
}

// ─── Template data ─────────────────────────────────────────────────────────

type templateData struct {
	Connected   bool
	Dialing     bool
	Discovering bool
	Status      string
	ErrMsg      string
	Radios      []radio.DiscoveredRadio
	LogEntries  []string
	Subs        []subscription
	Slices      map[string]map[string]string
	Handle      uint32
	Version     string
	Addr        string
}

func (h *Hub) templateData() templateData {
	handle, version := h.ConnInfo()
	return templateData{
		Connected:   h.IsConnected(),
		Dialing:     h.IsDialing(),
		Discovering: h.IsDiscovering(),
		Status:      h.Status(),
		ErrMsg:      h.ErrMsg(),
		Radios:      h.Radios(),
		LogEntries:  h.LogEntries(),
		Subs:        h.Subs(),
		Slices:      h.GetSlices(),
		Handle:      handle,
		Version:     version,
		Addr:        h.addr,
	}
}
