// hub.go — central event loop bridging radio events to HTTP-visible state.

package web

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"goFlex/config"
	"goFlex/radio"

	"github.com/google/uuid"
)

// Command represents an action requested by an HTTP handler.
type Command struct {
	Kind string // "connect", "disconnect", "subscribe", "tune", "command"
	Addr string
	Subs []string
	Name string
	Val  string // freq MHz or raw command
}

// Event is pushed to SSE subscribers.
type Event struct {
	Kind string // "log", "status", "slices", "state"
	Data string
}

// RadioConn is the subset of radio.Conn used by Hub, extracted for testability.
type RadioConn interface {
	Send(string, func(int, string)) (uint32, error)
	EnableReconnect()
	DisableReconnect()
	Close()
	ReadLoop(func(radio.ParsedMessage)) error
	ReconnectDone() <-chan struct{}
	State() radio.ConnectionState
	GetHandle() uint32
	GetVersion() string
}

// Hub owns all shared state and runs the event loop.
type Hub struct {
	cfg *config.Config

	mu sync.RWMutex
	// Discovery
	radios      []radio.DiscoveredRadio
	discovering bool
	discCancel  context.CancelFunc

	// Connection
	conn      RadioConn
	connected bool
	dialing   bool
	addr      string

	// dialFunc is injected for testing; defaults to radio.Dial.
	dialFunc func(string) (*radio.Conn, error)

	// Subscriptions
	subs []subscription

	// Log
	logBuf []string

	// Slice data
	slices *radio.SliceCollector

	// Status
	status string
	errMsg string

	// SSE subscribers
	subscribers map[chan Event]struct{}

	// Command channel from HTTP handlers
	commands chan Command
}

type subscription struct {
	Name    string
	Label   string
	Checked bool
}

func defaultSubs() []subscription {
	return []subscription{
		{Name: "slice", Label: "Slice", Checked: true},
		{Name: "pan", Label: "Panadapter", Checked: true},
		{Name: "tx", Label: "TX", Checked: true},
		{Name: "meter", Label: "Meter", Checked: true},
		{Name: "audio", Label: "Audio", Checked: true},
		{Name: "radio", Label: "Radio", Checked: true},
		{Name: "client", Label: "Client", Checked: true},
		{Name: "atu", Label: "ATU", Checked: false},
		{Name: "amplifier", Label: "Amplifier", Checked: false},
		{Name: "gps", Label: "GPS", Checked: false},
		{Name: "xvtr", Label: "Transverter", Checked: false},
		{Name: "apd", Label: "APD", Checked: false},
		{Name: "tnf", Label: "TNF", Checked: false},
		{Name: "memories", Label: "Memories", Checked: false},
		{Name: "cwx", Label: "CWX", Checked: false},
		{Name: "dax", Label: "DAX", Checked: false},
		{Name: "daxiq", Label: "DAX IQ", Checked: false},
		{Name: "codec", Label: "Codec", Checked: false},
		{Name: "dvk", Label: "DVK", Checked: false},
		{Name: "usb_cable", Label: "USB Cable", Checked: false},
		{Name: "spot", Label: "Spot", Checked: false},
		{Name: "license", Label: "License", Checked: false},
	}
}

// NewHub creates a Hub and starts its event loop.
func NewHub(cfg *config.Config) *Hub {
	h := &Hub{
		cfg:         cfg,
		subs:        defaultSubs(),
		slices:      radio.NewSliceCollector(),
		subscribers: make(map[chan Event]struct{}),
		commands:    make(chan Command, 16),
		addr:        fmt.Sprintf("%s:%d", cfg.RadioAddress, cfg.RadioPort),
		dialFunc:    radio.Dial,
	}
	go h.startDiscovery()
	go h.loop()
	return h
}

// SendCommand queues a command for the event loop.
func (h *Hub) SendCommand(cmd Command) {
	h.commands <- cmd
}

// Close stops the discovery goroutine and shuts down the event loop.
func (h *Hub) Close() {
	h.stopDiscovery()
	close(h.commands)
}

// ─── Read accessors (thread-safe) ──────────────────────────────────────────

func (h *Hub) Radios() []radio.DiscoveredRadio {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]radio.DiscoveredRadio, len(h.radios))
	copy(out, h.radios)
	return out
}

func (h *Hub) IsDiscovering() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.discovering
}

func (h *Hub) IsConnected() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.connected
}

func (h *Hub) IsDialing() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.dialing
}

func (h *Hub) Status() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.status
}

func (h *Hub) ErrMsg() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.errMsg
}

func (h *Hub) LogEntries() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]string, len(h.logBuf))
	copy(out, h.logBuf)
	return out
}

func (h *Hub) Subs() []subscription {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]subscription, len(h.subs))
	copy(out, h.subs)
	return out
}

func (h *Hub) GetSlices() map[string]map[string]string {
	return h.slices.GetSlices()
}

func (h *Hub) ConnInfo() (handle uint32, version string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.conn != nil {
		return h.conn.GetHandle(), h.conn.GetVersion()
	}
	return 0, ""
}

// ─── SSE ───────────────────────────────────────────────────────────────────

// Subscribe returns a channel that receives events. Call Unsubscribe to clean up.
func (h *Hub) Subscribe() chan Event {
	ch := make(chan Event, 64)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel.
func (h *Hub) Unsubscribe(ch chan Event) {
	h.mu.Lock()
	delete(h.subscribers, ch)
	h.mu.Unlock()
}

func (h *Hub) broadcast(evt Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers {
		select {
		case ch <- evt:
		default:
			// drop if subscriber is slow
		}
	}
}

// ─── Event loop ────────────────────────────────────────────────────────────

func (h *Hub) loop() {
	for cmd := range h.commands {
		h.processCommand(cmd)
	}
}

func (h *Hub) processCommand(cmd Command) {
	switch cmd.Kind {
	case "connect":
		h.doConnect(cmd.Addr, cmd.Subs)
	case "disconnect":
		h.doDisconnect()
	case "subscribe":
		h.doSubscribe(cmd.Name, cmd.Val == "true")
	case "tune":
		h.doTune(cmd.Val)
	case "command":
		h.doRawCommand(cmd.Val)
	}
}

// ─── Discovery ─────────────────────────────────────────────────────────────

func (h *Hub) startDiscovery() {
	ctx, cancel := context.WithCancel(context.Background())

	h.mu.Lock()
	h.discovering = true
	h.discCancel = cancel
	h.mu.Unlock()

	evtCh, err := radio.Listen(ctx)
	if err != nil {
		h.mu.Lock()
		h.discovering = false
		h.mu.Unlock()
		log.Printf("discovery error: %v", err)
		return
	}

	for evt := range evtCh {
		h.mu.Lock()
		if evt.Lost {
			h.radios = removeRadio(h.radios, evt.Radio.Serial)
		} else {
			h.radios = upsertRadio(h.radios, evt.Radio)
		}
		h.mu.Unlock()
		h.broadcast(Event{Kind: "state", Data: "discovery"})
	}
}

func (h *Hub) stopDiscovery() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.discCancel != nil {
		h.discCancel()
		h.discCancel = nil
	}
	h.discovering = false
}

// ─── Connect / Disconnect ──────────────────────────────────────────────────

func (h *Hub) doConnect(addr string, subNames []string) {
	h.mu.Lock()
	if h.dialing || h.connected {
		h.mu.Unlock()
		return
	}
	h.dialing = true
	h.errMsg = ""
	h.status = "Connecting…"
	if addr != "" {
		h.addr = addr
	}
	// Update subscription checked state from the form.
	checked := make(map[string]bool)
	for _, n := range subNames {
		checked[n] = true
	}
	for i := range h.subs {
		h.subs[i].Checked = checked[h.subs[i].Name]
	}
	h.mu.Unlock()

	h.broadcast(Event{Kind: "state", Data: "connecting"})

	conn, err := h.dialFunc(h.addr)
	if err != nil {
		h.mu.Lock()
		h.dialing = false
		h.errMsg = err.Error()
		h.status = "Disconnected"
		h.mu.Unlock()
		h.broadcast(Event{Kind: "state", Data: "error"})
		return
	}

	h.mu.Lock()
	h.conn = conn
	h.connected = true
	h.dialing = false
	h.status = fmt.Sprintf("Connected  handle=0x%X  version=%s", conn.GetHandle(), conn.GetVersion())
	h.logBuf = nil
	h.mu.Unlock()

	// Wire up callbacks BEFORE sending commands so they're captured.
	conn.OnLog = func(direction, line string) {
		var prefix string
		if direction == "tx" {
			prefix = "→ "
		} else {
			prefix = "← "
		}
		h.appendLog(prefix + line)
		h.broadcast(Event{Kind: "log", Data: prefix + line})
	}
	conn.OnStateChange = func(oldState, newState radio.ConnectionState) {
		msg := fmt.Sprintf("[state] %s → %s", oldState, newState)
		h.appendLog(msg)
		h.broadcast(Event{Kind: "log", Data: msg})
		h.broadcast(Event{Kind: "state", Data: "changed"})
	}
	conn.OnPingRtt = func(ms int) {
		msg := fmt.Sprintf("[ping] RTT %d ms", ms)
		h.appendLog(msg)
		h.broadcast(Event{Kind: "log", Data: msg})
	}

	// Log initial connection info.
	h.appendLog(fmt.Sprintf("Connected to %s", h.addr))
	h.appendLog(fmt.Sprintf("Version: %s  Handle: 0x%X", conn.GetVersion(), conn.GetHandle()))

	// Subscribe to checked items.
	for _, s := range h.subs {
		if s.Checked {
			h.appendLog(fmt.Sprintf("subscribing to %s ...", s.Name))
			conn.Send(fmt.Sprintf("sub %s all", s.Name), func(code int, body string) {
				if code != 0 {
					h.appendLog(fmt.Sprintf("[sub] %s rejected (code %d): %s", s.Name, code, body))
				}
			})
		}
	}

	// Register as GUI client.
	guiClientID := uuid.New().String()
	conn.Send(fmt.Sprintf("client gui %s", guiClientID), func(code int, body string) {
		if code == 0 {
			conn.Send("client program AetherSDR", nil)
			conn.Send("client station AetherSDR-Go", nil)
		} else {
			h.appendLog(fmt.Sprintf("[client] gui registration rejected (code %d): %s", code, body))
		}
	})

	conn.EnableReconnect()

	// Start read loop.
	go func() {
		err := conn.ReadLoop(func(msg radio.ParsedMessage) {
			h.slices.HandleStatus(msg)
			line := fmt.Sprintf("%-30s %v", msg.Object, msg.KVs)
			h.appendLog(line)
			h.broadcast(Event{Kind: "log", Data: line})
			h.broadcast(Event{Kind: "slices", Data: "updated"})
		})
		if err != nil {
			h.appendLog(fmt.Sprintf("[conn] connection lost: %v", err))
		} else {
			h.appendLog("[conn] connection lost, attempting reconnect")
		}
		h.broadcast(Event{Kind: "log", Data: "[conn] disconnected"})

		h.mu.Lock()
		h.connected = false
		h.conn = nil
		h.status = "Disconnected — reconnecting…"
		h.mu.Unlock()
		h.broadcast(Event{Kind: "state", Data: "disconnected"})

		// Wait for reconnect.
		done := conn.ReconnectDone()
		if done != nil {
			select {
			case <-done:
				if conn.State() == radio.StateConnected {
					h.mu.Lock()
					h.connected = true
					h.conn = conn
					h.status = fmt.Sprintf("Reconnected  handle=0x%X  version=%s", conn.GetHandle(), conn.GetVersion())
					h.mu.Unlock()
					h.appendLog("[conn] reconnected successfully")
					h.broadcast(Event{Kind: "state", Data: "connected"})
					// Restart read loop.
					go h.readLoopAgain(conn)
					return
				}
			case <-time.After(30 * time.Second):
			}
		}
		h.appendLog("[conn] reconnect failed")
		h.broadcast(Event{Kind: "state", Data: "reconnect_failed"})
	}()

	h.broadcast(Event{Kind: "state", Data: "connected"})
}

func (h *Hub) readLoopAgain(conn *radio.Conn) {
	err := conn.ReadLoop(func(msg radio.ParsedMessage) {
		h.slices.HandleStatus(msg)
		line := fmt.Sprintf("%-30s %v", msg.Object, msg.KVs)
		h.appendLog(line)
		h.broadcast(Event{Kind: "log", Data: line})
		h.broadcast(Event{Kind: "slices", Data: "updated"})
	})
	if err != nil {
		h.appendLog(fmt.Sprintf("[conn] connection lost: %v", err))
	}
	h.broadcast(Event{Kind: "state", Data: "disconnected"})
}

func (h *Hub) doDisconnect() {
	h.mu.Lock()
	conn := h.conn
	h.conn = nil
	h.connected = false
	h.dialing = false
	h.status = "Disconnected"
	h.mu.Unlock()

	if conn != nil {
		conn.DisableReconnect()
		conn.Close()
	}

	h.broadcast(Event{Kind: "state", Data: "disconnected"})
}

// ─── Subscribe / Tune / Raw command ────────────────────────────────────────

func (h *Hub) doSubscribe(name string, checked bool) {
	h.mu.RLock()
	conn := h.conn
	h.mu.RUnlock()

	if conn != nil {
		var cmd string
		if checked {
			cmd = fmt.Sprintf("sub %s all", name)
		} else {
			cmd = fmt.Sprintf("unsub %s all", name)
		}
		if _, err := conn.Send(cmd, nil); err != nil {
			h.appendLog(fmt.Sprintf("[sub] failed to toggle %s: %v", name, err))
		}
	}

	h.mu.Lock()
	for i := range h.subs {
		if h.subs[i].Name == name {
			h.subs[i].Checked = checked
			break
		}
	}
	h.mu.Unlock()

	h.broadcast(Event{Kind: "state", Data: "subs"})
}

func (h *Hub) doTune(freqMHz string) {
	h.mu.RLock()
	conn := h.conn
	h.mu.RUnlock()
	if conn == nil {
		return
	}

	cmd := fmt.Sprintf("slice tune 0 %s autopan=0", freqMHz)
	if _, err := conn.Send(cmd, func(code int, body string) {
		if code != 0 {
			h.appendLog(fmt.Sprintf("[freq] tune rejected (code %d): %s", code, body))
		}
	}); err != nil {
		h.appendLog(fmt.Sprintf("[freq] failed to send tune: %v", err))
	}
	h.appendLog(fmt.Sprintf("[freq] set %s MHz", freqMHz))
}

func (h *Hub) doRawCommand(cmd string) {
	h.mu.RLock()
	conn := h.conn
	h.mu.RUnlock()
	if conn == nil {
		return
	}
	if _, err := conn.Send(cmd, nil); err != nil {
		h.appendLog(fmt.Sprintf("[cmd] failed: %v", err))
	}
}

// ─── Log ───────────────────────────────────────────────────────────────────

func (h *Hub) appendLog(line string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.logBuf = append([]string{line}, h.logBuf...)
	if len(h.logBuf) > h.cfg.MaxLog {
		h.logBuf = h.logBuf[:h.cfg.MaxLog]
	}
}

// ─── Radio list helpers ────────────────────────────────────────────────────

func upsertRadio(radios []radio.DiscoveredRadio, info radio.DiscoveredRadio) []radio.DiscoveredRadio {
	for i, r := range radios {
		if r.Serial == info.Serial {
			radios[i] = info
			return radios
		}
	}
	return append(radios, info)
}

func removeRadio(radios []radio.DiscoveredRadio, serial string) []radio.DiscoveredRadio {
	for i, r := range radios {
		if r.Serial == serial {
			return append(radios[:i], radios[i+1:]...)
		}
	}
	return radios
}
