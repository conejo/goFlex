// radio.go — TCP connection to a FlexRadio (SmartSDR protocol).

package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

const defaultPort = 4992

// RadioConn is a minimal TCP connection to a FlexRadio.
type RadioConn struct {
	conn      net.Conn
	scanner   *bufio.Scanner
	seqCtr    atomic.Uint32
	Handle    uint32
	Version   string
	callbacks map[uint32]func(code int, body string)
}

// Dial opens a TCP connection to the radio and waits for V + H.
// Returns once the radio has issued both (connection is ready for commands).
func Dial(address string) (*RadioConn, error) {
	addr := address
	if !strings.Contains(addr, ":") {
		addr = fmt.Sprintf("%s:%d", addr, defaultPort)
	}

	c, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("radio dial: %w", err)
	}

	rc := &RadioConn{
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
func (rc *RadioConn) readHandshake() error {
	gotVersion := false
	gotHandle := false
	for rc.scanner.Scan() {
		msg := parseLine(rc.scanner.Text())
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

// Send transmits a command and returns the sequence number.
// An optional callback is called when the R-line response arrives.
func (rc *RadioConn) Send(command string, cb func(code int, body string)) (uint32, error) {
	seq := rc.seqCtr.Add(1)
	if cb != nil {
		rc.callbacks[seq] = cb
	}
	wire := fmt.Sprintf("C%d|%s\n", seq, command)
	_, err := fmt.Fprint(rc.conn, wire)
	if err != nil {
		return 0, fmt.Errorf("send: %w", err)
	}
	return seq, nil
}

// ReadLoop reads incoming lines until the connection closes.
// Responses are dispatched to registered callbacks; S-lines call onStatus.
func (rc *RadioConn) ReadLoop(onStatus func(msg ParsedMessage)) error {
	for rc.scanner.Scan() {
		msg := parseLine(rc.scanner.Text())
		switch msg.Type {
		case MsgResponse:
			if cb, ok := rc.callbacks[msg.Sequence]; ok {
				delete(rc.callbacks, msg.Sequence)
				cb(msg.ResultCode, msg.Object)
			}
		case MsgStatus:
			if onStatus != nil {
				onStatus(msg)
			}
		}
	}
	return rc.scanner.Err()
}

// Close shuts down the connection.
func (rc *RadioConn) Close() { rc.conn.Close() }
