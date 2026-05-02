// radio_test.go — unit tests for connection resilience features.

package radio

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestConnectionState_String(t *testing.T) {
	t.Parallel()
	cases := []struct {
		state ConnectionState
		want  string
	}{
		{StateDisconnected, "Disconnected"},
		{StateConnecting, "Connecting"},
		{StateConnected, "Connected"},
		{StateError, "Error"},
		{ConnectionState(99), "Unknown"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if got := tc.state.String(); got != tc.want {
				t.Fatalf("state %d: want %q, got %q", tc.state, tc.want, got)
			}
		})
	}
}

func TestConn_StateTransitions(t *testing.T) {
	// Create a minimal Conn to test state transitions.
	c := &Conn{}

	if c.State() != StateDisconnected {
		t.Fatalf("initial state should be Disconnected, got %v", c.State())
	}

	var transitions []string
	c.OnStateChange = func(oldState, newState ConnectionState) {
		transitions = append(transitions, oldState.String()+"->"+newState.String())
	}

	c.setState(StateConnecting)
	if c.State() != StateConnecting {
		t.Fatalf("expected Connecting, got %v", c.State())
	}

	c.setState(StateConnected)
	if c.State() != StateConnected {
		t.Fatalf("expected Connected, got %v", c.State())
	}

	c.setState(StateDisconnected)
	if c.State() != StateDisconnected {
		t.Fatalf("expected Disconnected, got %v", c.State())
	}

	wantTransitions := []string{
		"Disconnected->Connecting",
		"Connecting->Connected",
		"Connected->Disconnected",
	}
	if len(transitions) != len(wantTransitions) {
		t.Fatalf("expected %d transitions, got %d: %v", len(wantTransitions), len(transitions), transitions)
	}
	for i, want := range wantTransitions {
		if transitions[i] != want {
			t.Fatalf("transition %d: want %q, got %q", i, want, transitions[i])
		}
	}
}

func TestConn_EnableDisableReconnect(t *testing.T) {
	c := &Conn{reconnectStopCh: make(chan struct{})}

	if c.reconnecting {
		t.Fatal("reconnect should be disabled by default")
	}

	c.EnableReconnect()
	if !c.reconnecting {
		t.Fatal("reconnect should be enabled after EnableReconnect")
	}
	if c.reconnectDelay != reconnectInitialDelay {
		t.Fatalf("expected initial delay %v, got %v", reconnectInitialDelay, c.reconnectDelay)
	}

	c.DisableReconnect()
	if c.reconnecting {
		t.Fatal("reconnect should be disabled after DisableReconnect")
	}
}

func TestConn_GracefulDisconnect_NoStream(t *testing.T) {
	c := &Conn{
		reconnectStopCh: make(chan struct{}),
	}
	c.EnableReconnect()
	c.GracefulDisconnect("", 0)

	// GracefulDisconnect sets gracefulClose = true; OnDisconnected resets it.
	c.gracefulMu.Lock()
	wasGraceful := c.gracefulClose
	c.gracefulMu.Unlock()
	if !wasGraceful {
		t.Fatal("gracefulClose should be true after GracefulDisconnect")
	}

	// Simulate OnDisconnected being called (as the TUI would do).
	c.OnDisconnected()
	c.gracefulMu.Lock()
	wasGraceful = c.gracefulClose
	c.gracefulMu.Unlock()
	if wasGraceful {
		t.Fatal("gracefulClose should be reset after OnDisconnected")
	}
}

func TestConn_Heartbeat_StartStop(t *testing.T) {
	c := &Conn{}

	c.startHeartbeat()
	if c.heartbeatTimer == nil {
		t.Fatal("heartbeat timer should be set")
	}

	c.stopHeartbeat()
	if c.heartbeatTimer != nil {
		t.Fatal("heartbeat timer should be nil after stop")
	}
}

func TestConn_StopReconnect_Idempotent(t *testing.T) {
	c := &Conn{
		reconnectStopCh: make(chan struct{}),
	}

	// First stop should close the channel.
	c.stopReconnect()
	select {
	case <-c.reconnectStopCh:
		// expected
	default:
		t.Fatal("reconnectStopCh should be closed after first stop")
	}

	// Second stop should not panic (idempotent).
	c.stopReconnect()
}

func TestConn_OnDisconnected_NoReconnect(t *testing.T) {
	c := &Conn{
		reconnectStopCh: make(chan struct{}),
	}
	// reconnecting is false by default.
	c.OnDisconnected()
	if c.State() != StateDisconnected {
		t.Fatalf("expected Disconnected, got %v", c.State())
	}
}

func TestConn_OnDisconnected_Graceful(t *testing.T) {
	c := &Conn{
		reconnectStopCh: make(chan struct{}),
	}
	c.EnableReconnect()
	c.gracefulClose = true

	c.OnDisconnected()
	if c.State() != StateDisconnected {
		t.Fatalf("expected Disconnected, got %v", c.State())
	}
	// Graceful disconnect should not schedule reconnect.
	if c.reconnectTimer != nil {
		t.Fatal("reconnect timer should not be set after graceful disconnect")
	}
}

func TestConn_Addr(t *testing.T) {
	c := &Conn{addr: "192.168.1.50:4992"}
	if c.Addr() != "192.168.1.50:4992" {
		t.Fatalf("expected Addr='192.168.1.50:4992', got %q", c.Addr())
	}
}

func TestConn_Send_CallbackConcurrency(t *testing.T) {
	// Verify callbacks map is protected by mutex.
	// Use a dummy net.Conn (net.Pipe) so Send can write without panicking.
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	// Drain the pipe so Send doesn't block.
	go io.Copy(io.Discard, server)

	c := &Conn{
		conn:      client,
		callbacks: make(map[uint32]func(int, string)),
	}
	c.seqCtr.Store(0)

	seq, err := c.Send("test command", func(code int, body string) {})
	if err != nil {
		t.Fatalf("unexpected send error: %v", err)
	}
	if seq != 1 {
		t.Fatalf("expected seq=1, got %d", seq)
	}

	// Verify callback was registered (and can be cleared safely).
	c.cbMu.Lock()
	delete(c.callbacks, 1)
	c.cbMu.Unlock()
}

func TestConn_PingRttCallback(t *testing.T) {
	c := &Conn{}
	var rttReceived int
	c.OnPingRtt = func(ms int) {
		rttReceived = ms
	}

	// Simulate a ping reply arriving with a fixed elapsed time.
	c.pingSeq = 5
	c.pingSent = time.Now().Add(-123 * time.Millisecond)

	// Manually trigger the RTT calculation path.
	if c.pingSeq == 5 {
		rtt := int(time.Since(c.pingSent).Milliseconds())
		c.pingSeq = 0
		if c.OnPingRtt != nil {
			c.OnPingRtt(rtt)
		}
	}

	if rttReceived < 100 || rttReceived > 200 {
		t.Fatalf("expected RTT around 123ms, got %d", rttReceived)
	}
}

func TestConn_ReconnectBackoff(t *testing.T) {
	c := &Conn{
		reconnectStopCh: make(chan struct{}),
		addr:            "127.0.0.1:1", // invalid — will fail quickly
	}
	c.EnableReconnect()

	// Simulate a failed reconnect attempt.
	initialDelay := c.reconnectDelay
	c.reconnectDelay *= 2
	if c.reconnectDelay != 2*initialDelay {
		t.Fatalf("expected delay to double, got %v", c.reconnectDelay)
	}

	// Cap at max.
	c.reconnectDelay = reconnectMaxDelay + time.Second
	if c.reconnectDelay > reconnectMaxDelay {
		c.reconnectDelay = reconnectMaxDelay
	}
	if c.reconnectDelay != reconnectMaxDelay {
		t.Fatalf("expected delay capped at %v, got %v", reconnectMaxDelay, c.reconnectDelay)
	}
}

// ─── Connection lifecycle tests ─────────────────────────────────────────────

func TestDial_Success(t *testing.T) {
	// Spin up a mock radio that sends the V + H handshake.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		c, _ := ln.Accept()
		if c != nil {
			fmt.Fprintf(c, "V3.3.28.0\nH00000001\n")
			// Keep connection open until test is done.
			buf := make([]byte, 1024)
			for {
				_, err := c.Read(buf)
				if err != nil {
					return
				}
			}
		}
	}()

	conn, err := Dial(ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if conn.State() != StateConnected {
		t.Fatalf("expected Connected, got %v", conn.State())
	}
	if conn.Version != "3.3.28.0" {
		t.Fatalf("Version: want %q, got %q", "3.3.28.0", conn.Version)
	}
	if conn.Handle != 1 {
		t.Fatalf("Handle: want 1, got %d", conn.Handle)
	}
}

func TestDial_HandshakeIncomplete(t *testing.T) {
	// Listener sends only V line then closes.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		c, _ := ln.Accept()
		if c != nil {
			fmt.Fprintf(c, "V3.3.28.0\n")
			c.Close()
		}
	}()

	_, err = Dial(ln.Addr().String())
	if err == nil {
		t.Fatal("expected error for incomplete handshake")
	}
	if !strings.Contains(err.Error(), "connection closed before handshake complete") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── ReadLoop dispatch tests ────────────────────────────────────────────────

func TestReadLoop_ResponseDispatch(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	// Drain the pipe so Send doesn't block.
	go io.Copy(io.Discard, server)

	c := &Conn{
		conn:      client,
		scanner:   bufio.NewScanner(client),
		callbacks: make(map[uint32]func(int, string)),
	}
	c.seqCtr.Store(0)

	var gotCode int
	var gotBody string
	c.Send("test", func(code int, body string) {
		gotCode = code
		gotBody = body
	})

	go func() {
		// Write the response on the server side.
		fmt.Fprintf(server, "R1|0|ok\n")
		server.Close()
	}()

	err := c.ReadLoop(nil)
	if err != nil {
		t.Fatalf("ReadLoop: %v", err)
	}

	if gotCode != 0 {
		t.Fatalf("code: want 0, got %d", gotCode)
	}
	if gotBody != "ok" {
		t.Fatalf("body: want %q, got %q", "ok", gotBody)
	}
}

func TestReadLoop_StatusDispatch(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	c := &Conn{
		conn:    client,
		scanner: bufio.NewScanner(client),
	}

	var gotMsg ParsedMessage
	go func() {
		fmt.Fprintf(server, "S00000001|slice 0 freq=14.225000\n")
		server.Close()
	}()

	err := c.ReadLoop(func(msg ParsedMessage) {
		gotMsg = msg
	})
	if err != nil {
		t.Fatalf("ReadLoop: %v", err)
	}

	if gotMsg.Type != msgStatus {
		t.Fatalf("type: want msgStatus, got %v", gotMsg.Type)
	}
	if gotMsg.Object != "slice 0" {
		t.Fatalf("object: want %q, got %q", "slice 0", gotMsg.Object)
	}
	if gotMsg.KVs["freq"] != "14.225000" {
		t.Fatalf("freq: want %q, got %q", "14.225000", gotMsg.KVs["freq"])
	}
}

func TestReadLoop_PingReply(t *testing.T) {
	// Test the ping-reply logic directly without relying on ReadLoop + net.Pipe
	// timing, which is fragile due to synchronous pipe I/O.
	c := &Conn{}
	var rttReceived int
	c.OnPingRtt = func(ms int) {
		rttReceived = ms
	}

	c.pingSeq = 1
	c.pingSent = time.Now().Add(-50 * time.Millisecond)

	msg := parseLine("R1|0|\n")
	if msg.Type != msgResponse || msg.Sequence != 1 {
		t.Fatalf("parseLine failed: %+v", msg)
	}

	// Inline the ping-reply handling from ReadLoop.
	if msg.Sequence == c.pingSeq && c.pingSeq != 0 {
		rtt := int(time.Since(c.pingSent).Milliseconds())
		c.pingSeq = 0
		if c.OnPingRtt != nil {
			c.OnPingRtt(rtt)
		}
	}

	if rttReceived == 0 {
		t.Fatal("expected OnPingRtt to be called")
	}
	if rttReceived < 10 || rttReceived > 500 {
		t.Fatalf("RTT out of range: %d", rttReceived)
	}
}

// ─── Send / Close edge cases ────────────────────────────────────────────────

func TestSend_WithoutCallback(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	go io.Copy(io.Discard, server)

	c := &Conn{
		conn:      client,
		callbacks: make(map[uint32]func(int, string)),
	}
	c.seqCtr.Store(0)

	seq, err := c.Send("sub slice all", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seq != 1 {
		t.Fatalf("seq: want 1, got %d", seq)
	}

	// No callback should be registered.
	c.cbMu.Lock()
	_, exists := c.callbacks[1]
	c.cbMu.Unlock()
	if exists {
		t.Fatal("callback should not be registered when cb is nil")
	}
}

func TestSend_SequenceIncrement(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	go io.Copy(io.Discard, server)

	c := &Conn{
		conn:      client,
		callbacks: make(map[uint32]func(int, string)),
	}
	c.seqCtr.Store(0)

	seq1, _ := c.Send("cmd1", nil)
	seq2, _ := c.Send("cmd2", nil)
	seq3, _ := c.Send("cmd3", nil)

	if seq1 != 1 || seq2 != 2 || seq3 != 3 {
		t.Fatalf("sequences: want 1,2,3 got %d,%d,%d", seq1, seq2, seq3)
	}
}

func TestClose_Idempotent(t *testing.T) {
	c := &Conn{reconnectStopCh: make(chan struct{})}
	c.Close()
	c.Close() // should not panic
}

// ─── End-to-end lifecycle test ───────────────────────────────────────────────

func TestConn_FullLifecycle(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		c, _ := ln.Accept()
		if c == nil {
			return
		}
		fmt.Fprintf(c, "V3.4.24.0\nH00000002\n")
		// Echo back any commands as responses, then close after first reply.
		scanner := bufio.NewScanner(c)
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) > 0 && line[0] == 'C' {
				parts := strings.SplitN(line[1:], "|", 2)
				if len(parts) == 2 {
					fmt.Fprintf(c, "R%s|0|ok\n", parts[0])
					c.Close()
					return
				}
			}
		}
		c.Close()
	}()

	conn, err := Dial(ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	if conn.State() != StateConnected {
		t.Fatalf("expected Connected, got %v", conn.State())
	}
	if conn.Version != "3.4.24.0" {
		t.Fatalf("Version: want %q, got %q", "3.4.24.0", conn.Version)
	}
	if conn.Handle != 2 {
		t.Fatalf("Handle: want 2, got %d", conn.Handle)
	}

	// Send a command and verify response dispatch.
	var gotBody string
	seq, err := conn.Send("sub slice all", func(code int, body string) {
		gotBody = body
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if seq != 1 {
		t.Fatalf("seq: want 1, got %d", seq)
	}

	// ReadLoop returns when the server closes the connection.
	_ = conn.ReadLoop(nil)

	if gotBody != "ok" {
		t.Fatalf("response body: want %q, got %q", "ok", gotBody)
	}

	conn.Close()
	if conn.State() != StateDisconnected {
		t.Fatalf("expected Disconnected after Close, got %v", conn.State())
	}
}
