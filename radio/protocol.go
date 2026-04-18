// protocol.go — SmartSDR wire protocol parser.
//
// Protocol wire format:
//   Inbound lines, newline-terminated:
//     V<version>               — firmware version string
//     H<hex-handle>            — client handle assigned by radio
//     R<seq>|<hex-code>|<body> — response to a command
//     S<handle>|<obj> k=v …   — status update
//
//   Outbound commands:
//     C<seq>|<command>\n

package radio

import (
	"strconv"
	"strings"
)

// msgType identifies the kind of line received from the radio.
type msgType int

const (
	msgUnknown  msgType = iota
	msgVersion          // V<version>
	msgHandle           // H<hex-handle>
	msgResponse         // R<seq>|<code>|<body>
	msgStatus           // S<handle>|<obj> k=v …
)

// ParsedMessage is the result of parsing one line received from the radio.
type ParsedMessage struct {
	Type       msgType
	Raw        string
	Object     string            // version string, status object name, or response body
	Handle     uint32            // hex handle from H or S lines
	Sequence   uint32            // response sequence number
	ResultCode int               // hex result code from R lines (0 = OK)
	KVs        map[string]string // key=value pairs from status/response body
}

// parseKVs splits "key=val key2=val2" into a map.
func parseKVs(body string) map[string]string {
	kvs := make(map[string]string)
	for _, token := range strings.Fields(body) {
		eq := strings.IndexByte(token, '=')
		if eq < 0 {
			kvs[token] = ""
		} else {
			kvs[token[:eq]] = token[eq+1:]
		}
	}
	return kvs
}

// parseLine parses one newline-terminated line from the radio.
func parseLine(raw string) ParsedMessage {
	raw = strings.TrimSpace(raw)
	msg := ParsedMessage{Raw: raw}
	if len(raw) == 0 {
		return msg
	}

	tag := raw[0]
	body := raw[1:]

	switch tag {
	case 'V':
		msg.Type = msgVersion
		msg.Object = body

	case 'H':
		msg.Type = msgHandle
		h, _ := strconv.ParseUint(body, 16, 32)
		msg.Handle = uint32(h)

	case 'R':
		msg.Type = msgResponse
		parts := strings.SplitN(body, "|", 3)
		if len(parts) >= 1 {
			seq, _ := strconv.ParseUint(parts[0], 10, 32)
			msg.Sequence = uint32(seq)
		}
		if len(parts) >= 2 {
			code, _ := strconv.ParseInt(parts[1], 16, 32)
			msg.ResultCode = int(code)
		}
		if len(parts) >= 3 {
			msg.Object = parts[2]
			msg.KVs = parseKVs(parts[2])
		}

	case 'S':
		msg.Type = msgStatus
		pipe := strings.IndexByte(body, '|')
		if pipe < 0 {
			break
		}
		h, _ := strconv.ParseUint(body[:pipe], 16, 32)
		msg.Handle = uint32(h)
		statusBody := body[pipe+1:]

		tokens := strings.Fields(statusBody)
		objTokens := []string{}
		kvStart := len(tokens)
		for i, t := range tokens {
			if strings.ContainsRune(t, '=') {
				kvStart = i
				break
			}
			objTokens = append(objTokens, t)
		}
		msg.Object = strings.Join(objTokens, " ")
		msg.KVs = parseKVs(strings.Join(tokens[kvStart:], " "))
	}

	return msg
}
