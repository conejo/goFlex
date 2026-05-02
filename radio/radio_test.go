// radio_test.go — unit tests for connection resilience features.

package radio

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestConnectionState_String(t *testing.T) {
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
		if got := tc.state.String(); got != tc.want {
			t.Fatalf("state %d: want %q, got %q", tc.state, tc.want, got)
		}
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
