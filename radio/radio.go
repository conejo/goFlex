// radio.go — TCP connection to a FlexRadio (SmartSDR protocol).

package radio

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

const defaultPort = 4992

// Conn is a live TCP connection to a FlexRadio.
type Conn struct {
	conn      net.Conn
	scanner   *bufio.Scanner
	seqCtr    atomic.Uint32
	Handle    uint32
	Version   string
	callbacks map[uint32]func(code int, body string)
	// OnLog is called (if set) for every sent command and received line.
	OnLog func(direction, line string)
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

	rc := &Conn{
		conn:      c,
		scanner:   bufio.NewScanner(c),
		callbacks: make(map[uint32]func(int, string)),
	}

	if err := rc.readHandshake(); err != nil {
		c.Close()
		return nil, err
	}

	return rc, nil
}

// readHandshake reads lines until both V and H are received.
func (rc *Conn) readHandshake() error {
	gotVersion := false
	gotHandle := false
	for rc.scanner.Scan() {
		msg := parseLine(rc.scanner.Text())
		switch msg.Type {
		case msgVersion:
			rc.Version = msg.Object
			gotVersion = true
		case msgHandle:
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
		rc.callbacks[seq] = cb
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
		msg := parseLine(raw)
		switch msg.Type {
		case msgResponse:
			if rc.OnLog != nil {
				rc.OnLog("rx", raw)
			}
			if cb, ok := rc.callbacks[msg.Sequence]; ok {
				delete(rc.callbacks, msg.Sequence)
				cb(msg.ResultCode, msg.Object)
			}
		case msgStatus:
			if onStatus != nil {
				onStatus(msg)
			}
		}
	}
	return rc.scanner.Err()
}

// Close shuts down the connection.
func (rc *Conn) Close() { rc.conn.Close() }
