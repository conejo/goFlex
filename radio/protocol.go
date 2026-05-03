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
	MsgUnknown  msgType = iota
	MsgVersion          // V<version>
	MsgHandle           // H<hex-handle>
	MsgResponse         // R<seq>|<code>|<body>
	MsgStatus           // S<handle>|<obj> k=v …
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

// ParseKVs splits "key=val key2=val2" into a map.
func ParseKVs(body string) map[string]string {
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

// ParseCommaKVs splits a comma-separated "key=val,key2=val2" string into a map.
// Values may be quoted with double quotes; quotes are stripped.
func ParseCommaKVs(body string) map[string]string {
	kvs := make(map[string]string)
	for _, token := range strings.Split(body, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		eq := strings.IndexByte(token, '=')
		if eq < 0 {
			kvs[token] = ""
			continue
		}
		key := strings.TrimSpace(token[:eq])
		val := strings.TrimSpace(token[eq+1:])
		// Strip surrounding quotes if present
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		kvs[key] = val
	}
	return kvs
}

// ParseLine parses one newline-terminated line from the radio.
func ParseLine(raw string) ParsedMessage {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParsedMessage{Raw: raw}
	}

	msg := ParsedMessage{Raw: raw}
	body := raw[1:]

	switch raw[0] {
	case 'V':
		msg.Type = MsgVersion
		msg.Object = body

	case 'H':
		msg.Type = MsgHandle
		msg.Handle = atohex(body)

	case 'R':
		msg.Type = MsgResponse
		parts := strings.SplitN(body, "|", 3)
		msg.Sequence = atoui(parts[0])
		if len(parts) > 1 {
			msg.ResultCode = atohexi(parts[1])
		}
		if len(parts) > 2 {
			msg.Object = parts[2]
			msg.KVs = ParseKVs(parts[2])
		}

	case 'S':
		msg.Type = MsgStatus
		parts := strings.SplitN(body, "|", 2)
		if len(parts) < 2 {
			break
		}
		msg.Handle = atohex(parts[0])
		msg.Object, msg.KVs = parseStatusBody(parts[1])
	}

	return msg
}

// ─── Small parsing helpers ──────────────────────────────────────────────────

func atoui(s string) uint32 {
	n, _ := strconv.ParseUint(s, 10, 32)
	return uint32(n)
}

func atohex(s string) uint32 {
	n, _ := strconv.ParseUint(s, 16, 32)
	return uint32(n)
}

func atohexi(s string) int {
	n, _ := strconv.ParseInt(s, 16, 32)
	return int(n)
}

// parseStatusBody extracts the object name and key-value map from the body
// of an S (status) message. Object names can be multi-word, e.g.
// "slice 0" or "display pan 0x40000000".
func parseStatusBody(body string) (object string, kvs map[string]string) {
	eq := strings.IndexByte(body, '=')
	if eq < 0 {
		return strings.TrimSpace(body), nil
	}
	split := strings.LastIndexByte(body[:eq], ' ')
	if split < 0 {
		return "", ParseKVs(body)
	}
	return strings.TrimSpace(body[:split]), ParseKVs(body[split+1:])
}
