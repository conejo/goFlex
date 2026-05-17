// radio.go — TCP connection to a FlexRadio (SmartSDR protocol).
//
// Connection resilience features (matching AetherSDR):
//   • TCP keepalive with OS-level probes
//   • Heartbeat ping every 30 s to detect dead connections
//   • Graceful disconnect with stream cleanup
//   • Connection state tracking (Disconnected / Connecting / Connected / Error)
//   • Auto-reconnect with exponential back-off
//   • RTT measurement via ping replies

package radio

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultPort           = 4992
	heartbeatInterval     = 30 * time.Second
	pingTimeout           = 5 * time.Second
	reconnectInitialDelay = 1 * time.Second
	reconnectMaxDelay     = 30 * time.Second
)

// ConnectionState mirrors AetherSDR's state machine.
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateError
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "Disconnected"
	case StateConnecting:
		return "Connecting"
	case StateConnected:
		return "Connected"
	case StateError:
		return "Error"
	default:
		return "Unknown"
	}
}

// Conn is a resilient TCP connection to a FlexRadio.
type Conn struct {
	conn      net.Conn
	scanner   *bufio.Scanner
	seqCtr    atomic.Uint32
	Handle    uint32
	Version   string
	callbacks map[uint32]func(code int, body string)
	cbMu      sync.Mutex

	// OnLog is called (if set) for every sent command and received line.
	OnLog func(direction, line string)

	// OnStateChange is called whenever the connection state changes.
	OnStateChange func(oldState, newState ConnectionState)

	// OnPingRtt is called with the measured RTT in milliseconds after each
	// successful heartbeat ping.
	OnPingRtt func(ms int)

	state atomic.Int32 // stores ConnectionState
	addr  string       // last dial address, used for reconnect

	// heartbeat
	heartbeatTimer *time.Timer
	pingSeq        uint32
	pingSent       time.Time

	// reconnect
	reconnectTimer    *time.Timer
	reconnectDelay    time.Duration
	reconnecting      bool
	reconnectStopCh   chan struct{}
	reconnectStopOnce sync.Once
	reconnectDoneCh   chan struct{} // closed when reconnect succeeds
	reconnectDoneOnce sync.Once

	// graceful disconnect
	gracefulMu    sync.Mutex
	gracefulClose bool
}

// Dial opens a TCP connection to the radio and waits for the V + H handshake.
func Dial(address string) (*Conn, error) {
	addr := address
	if !strings.Contains(addr, ":") {
		addr = fmt.Sprintf("%s:%d", addr, defaultPort)
	}

	c, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("radio dial: %w", err)
	}

	// Enable TCP keepalive — detects dead peers at the kernel level.
	if tcpConn, ok := c.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(3 * time.Second)
	}

	rc := &Conn{
		conn:            c,
		scanner:         bufio.NewScanner(c),
		callbacks:       make(map[uint32]func(int, string)),
		addr:            addr,
		reconnectStopCh: make(chan struct{}),
		reconnectDoneCh: make(chan struct{}),
	}
	rc.setState(StateConnecting)

	if err := rc.readHandshake(); err != nil {
		c.Close()
		rc.setState(StateError)
		return nil, err
	}

	rc.setState(StateConnected)
	rc.startHeartbeat()
	return rc, nil
}

// State returns the current connection state.
func (rc *Conn) State() ConnectionState {
	return ConnectionState(rc.state.Load())
}

// GetHandle returns the radio handle assigned during handshake.
func (rc *Conn) GetHandle() uint32 { return rc.Handle }

// GetVersion returns the radio firmware version from the handshake.
func (rc *Conn) GetVersion() string { return rc.Version }

func (rc *Conn) setState(s ConnectionState) {
	old := ConnectionState(rc.state.Swap(int32(s)))
	if old != s && rc.OnStateChange != nil {
		rc.OnStateChange(old, s)
	}
}

// readHandshake reads lines until both V and H are received.
func (rc *Conn) readHandshake() error {
	gotVersion := false
	gotHandle := false
	for rc.scanner.Scan() {
		msg := ParseLine(rc.scanner.Text())
		switch msg.Type {
		case MsgVersion:
			rc.Version = msg.Object
			gotVersion = true
		case MsgHandle:
			rc.Handle = msg.Handle
			gotHandle = true
		}
		if gotVersion && gotHandle {
			return nil
		}
	}
	if err := rc.scanner.Err(); err != nil {
		return fmt.Errorf("handshake read: %w", err)
	}
	return fmt.Errorf("connection closed before handshake complete")
}

// Send transmits a command and optionally registers a response callback.
func (rc *Conn) Send(command string, cb func(code int, body string)) (uint32, error) {
	seq := rc.seqCtr.Add(1)
	if cb != nil {
		rc.cbMu.Lock()
		rc.callbacks[seq] = cb
		rc.cbMu.Unlock()
	}
	wire := fmt.Sprintf("C%d|%s", seq, command)
	_, err := fmt.Fprintf(rc.conn, "%s\n", wire)
	if err != nil {
		return 0, fmt.Errorf("send: %w", err)
	}
	if rc.OnLog != nil {
		rc.OnLog("tx", wire)
	}
	return seq, nil
}

// ReadLoop reads incoming lines until the connection closes, dispatching
// responses to registered callbacks and status lines to onStatus.
func (rc *Conn) ReadLoop(onStatus func(ParsedMessage)) error {
	for rc.scanner.Scan() {
		raw := rc.scanner.Text()
		msg := ParseLine(raw)
		switch msg.Type {
		case MsgResponse:
			// Check for ping reply first.
			if msg.Sequence == rc.pingSeq && rc.pingSeq != 0 {
				rtt := int(time.Since(rc.pingSent).Milliseconds())
				rc.pingSeq = 0
				if rc.OnPingRtt != nil {
					rc.OnPingRtt(rtt)
				}
				continue // don't log pings
			}
			if rc.OnLog != nil {
				rc.OnLog("rx", raw)
			}
			rc.cbMu.Lock()
			if cb, ok := rc.callbacks[msg.Sequence]; ok {
				delete(rc.callbacks, msg.Sequence)
				rc.cbMu.Unlock()
				cb(msg.ResultCode, msg.Object)
			} else {
				rc.cbMu.Unlock()
			}
		case MsgStatus:
			if onStatus != nil {
				onStatus(msg)
			}
		}
	}
	return rc.scanner.Err()
}

// RegisterClient registers this connection as a GUI client.
func (rc *Conn) RegisterClient(ctx context.Context, clientID string) error {
	done := make(chan struct{})
	_, err := rc.Send(fmt.Sprintf("client gui %s", clientID), func(code int, body string) {
		close(done)
	})
	if err != nil {
		return err
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SubscribeSlices subscribes to all slice status updates.
func (rc *Conn) SubscribeSlices(ctx context.Context) error {
	done := make(chan struct{})
	_, err := rc.Send("sub slice all", func(code int, body string) {
		close(done)
	})
	if err != nil {
		return err
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// UnsubscribeSlices unsubscribes from all slice status updates.
func (rc *Conn) UnsubscribeSlices(ctx context.Context) error {
	done := make(chan struct{})
	_, err := rc.Send("unsub slice all", func(code int, body string) {
		close(done)
	})
	if err != nil {
		return err
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ─── Heartbeat ──────────────────────────────────────────────────────────────

func (rc *Conn) startHeartbeat() {
	rc.heartbeatTimer = time.AfterFunc(heartbeatInterval, rc.heartbeatTick)
}

func (rc *Conn) stopHeartbeat() {
	if rc.heartbeatTimer != nil {
		rc.heartbeatTimer.Stop()
		rc.heartbeatTimer = nil
	}
}

func (rc *Conn) heartbeatTick() {
	if rc.State() != StateConnected {
		return
	}
	rc.pingSent = time.Now()
	seq, err := rc.Send("ping", nil)
	if err != nil {
		rc.conn.Close()
		return
	}
	rc.pingSeq = seq

	// If no reply within pingTimeout, treat as dead connection.
	time.AfterFunc(pingTimeout, func() {
		if rc.pingSeq == seq {
			// Ping reply never arrived — force disconnect.
			rc.conn.Close()
		}
	})

	rc.heartbeatTimer = time.AfterFunc(heartbeatInterval, rc.heartbeatTick)
}

// ─── Graceful disconnect ────────────────────────────────────────────────────

// GracefulDisconnect sends stream remove commands and waits for radio
// responses before closing the TCP socket.  This prevents stale sessions
// in the radio's client table (AetherSDR #2218).
func (rc *Conn) GracefulDisconnect(streamID string, streamRemoveSeq uint32) {
	rc.gracefulMu.Lock()
	rc.gracefulClose = true
	rc.gracefulMu.Unlock()

	rc.stopHeartbeat()

	if streamID != "" && streamRemoveSeq != 0 {
		// Wait up to 2 s for the radio to ack the stream remove.
		done := make(chan struct{})
		if _, err := rc.Send(fmt.Sprintf("stream remove 0x%s", streamID), func(code int, body string) {
			close(done)
		}); err != nil {
			// Send failed — skip waiting for response.
		} else {
			select {
			case <-done:
			case <-time.After(2 * time.Second):
			}
		}
	}

	rc.Close()
}

// Close shuts down the connection immediately.
func (rc *Conn) Close() {
	rc.stopHeartbeat()
	rc.stopReconnect()
	if rc.conn != nil {
		rc.conn.Close()
	}
	rc.setState(StateDisconnected)
}

// ─── Auto-reconnect ─────────────────────────────────────────────────────────

// EnableReconnect starts watching for disconnects and automatically re-dials
// with exponential back-off.  Call before ReadLoop.
func (rc *Conn) EnableReconnect() {
	rc.reconnecting = true
	rc.reconnectDelay = reconnectInitialDelay
}

// DisableReconnect stops any in-progress reconnect attempts.
func (rc *Conn) DisableReconnect() {
	rc.reconnecting = false
	rc.stopReconnect()
}

func (rc *Conn) stopReconnect() {
	if rc.reconnectTimer != nil {
		rc.reconnectTimer.Stop()
		rc.reconnectTimer = nil
	}
	rc.reconnectStopOnce.Do(func() {
		if rc.reconnectStopCh != nil {
			close(rc.reconnectStopCh)
		}
	})
	// Signal any waiter on ReconnectDone that reconnect won't happen.
	if rc.reconnectDoneCh != nil {
		rc.reconnectDoneOnce.Do(func() {
			close(rc.reconnectDoneCh)
		})
	}
}

// OnDisconnected should be called when ReadLoop returns (connection lost).
// If reconnect is enabled, it schedules a re-dial attempt.
func (rc *Conn) OnDisconnected() {
	rc.setState(StateDisconnected)
	if !rc.reconnecting {
		return
	}

	// Check if this was a graceful disconnect initiated by us.
	rc.gracefulMu.Lock()
	wasGraceful := rc.gracefulClose
	rc.gracefulClose = false
	rc.gracefulMu.Unlock()
	if wasGraceful {
		return
	}

	// Create a fresh channel for this reconnect cycle.
	rc.reconnectDoneCh = make(chan struct{})
	rc.reconnectDoneOnce = sync.Once{}

	rc.reconnectTimer = time.AfterFunc(rc.reconnectDelay, func() {
		select {
		case <-rc.reconnectStopCh:
			return
		default:
		}

		newConn, err := Dial(rc.addr)
		if err != nil {
			// Back off and retry.
			rc.reconnectDelay *= 2
			if rc.reconnectDelay > reconnectMaxDelay {
				rc.reconnectDelay = reconnectMaxDelay
			}
			rc.OnDisconnected()
			return
		}

		// Success — copy new connection state to this Conn.
		rc.conn = newConn.conn
		rc.scanner = newConn.scanner
		rc.Handle = newConn.Handle
		rc.Version = newConn.Version
		rc.state.Store(int32(StateConnected))
		rc.reconnectDelay = reconnectInitialDelay
		rc.startHeartbeat()
		rc.reconnectDoneOnce.Do(func() {
			close(rc.reconnectDoneCh)
		})
	})
}

// Addr returns the dial address used for this connection.
func (rc *Conn) Addr() string { return rc.addr }

// ReconnectDone returns a channel that is closed when an auto-reconnect
// attempt succeeds. Returns nil if reconnect is not enabled.
func (rc *Conn) ReconnectDone() <-chan struct{} {
	if !rc.reconnecting {
		return nil
	}
	return rc.reconnectDoneCh
}
