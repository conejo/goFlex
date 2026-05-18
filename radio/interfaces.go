// interfaces.go — shared abstractions for radio connection consumers.

package radio

// RadioConn is the subset of *Conn used by UI packages (app and web).
// Extracted for testability so mock implementations can be injected.
type RadioConn interface {
	Send(string, func(int, string)) (uint32, error)
	EnableReconnect()
	DisableReconnect()
	Close()
	ReadLoop(func(ParsedMessage)) error
	ReconnectDone() <-chan struct{}
	State() ConnectionState
	GetHandle() uint32
	GetVersion() string
	SetOnLog(func(string, string))
	SetOnStateChange(func(ConnectionState, ConnectionState))
	SetOnPingRtt(func(int))
	OnDisconnected()
}
