// discovery.go — Passive UDP discovery of FlexRadio SmartSDR devices.
//
// FlexRadio devices broadcast key=value status datagrams to UDP port 4992.
// No outbound request is sent; we simply listen and track what arrives.
// A radio is considered gone when no datagram has been received for 5 s.

package radio

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	discoveryPort    = 4992
	staleTimeout     = 5 * time.Second
	staleCheckPeriod = time.Second
)

// DiscoveredRadio holds the fields broadcast by a FlexRadio discovery datagram.
type DiscoveredRadio struct {
	Name               string
	Model              string
	Serial             string // unique key; datagrams without serial are ignored
	Version            string
	Nickname           string
	Callsign           string
	Address            string // IP address (from ip= field or UDP sender)
	Port               uint16
	Status             string // e.g. "Available", "In_Use"
	MaxLicensedVersion int
	InUse              bool
	GuiClientStations  []string
	GuiClientHandles   []string
	GuiClientPrograms  []string
}

// DiscoveryEvent is emitted on the channel returned by Listen.
type DiscoveryEvent struct {
	Radio DiscoveredRadio
	Lost  bool // true when the radio has gone stale and is being removed
}

// Listen binds to UDP :<discoveryPort> and streams DiscoveryEvents until ctx
// is cancelled or an unrecoverable socket error occurs.  The returned channel
// is closed when listening stops.
func Listen(ctx context.Context) (<-chan DiscoveryEvent, error) {
	pc, err := net.ListenPacket("udp4", ":"+strconv.Itoa(discoveryPort))
	if err != nil {
		return nil, err
	}

	ch := make(chan DiscoveryEvent, 16)
	go runDiscovery(ctx, pc, ch)
	return ch, nil
}

// runDiscovery is the background goroutine; it owns pc and closes ch on exit.
func runDiscovery(ctx context.Context, pc net.PacketConn, ch chan<- DiscoveryEvent) {
	defer pc.Close()
	defer close(ch)

	type entry struct {
		info    DiscoveredRadio
		lastSee time.Time
	}

	var mu sync.Mutex
	seen := make(map[string]*entry)

	// Stale-check ticker — runs in a separate goroutine.
	ticker := time.NewTicker(staleCheckPeriod)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				mu.Lock()
				for serial, e := range seen {
					if now.Sub(e.lastSee) > staleTimeout {
						select {
						case ch <- DiscoveryEvent{Radio: e.info, Lost: true}:
						default:
						}
						delete(seen, serial)
					}
				}
				mu.Unlock()
			}
		}
	}()

	buf := make([]byte, 4096)
	for {
		// Allow ctx cancellation to unblock the read.
		if deadline, ok := ctx.Deadline(); ok {
			pc.SetReadDeadline(deadline)
		} else {
			pc.SetReadDeadline(time.Now().Add(time.Second))
		}

		n, addr, err := pc.ReadFrom(buf)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			// Timeout used to re-check ctx — not a real error.
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}

		info, ok := parseDiscoveryPacket(buf[:n], addr)
		if !ok {
			continue
		}

		mu.Lock()
		e, exists := seen[info.Serial]
		if !exists {
			e = &entry{}
			seen[info.Serial] = e
		}
		e.info = info
		e.lastSee = time.Now()
		mu.Unlock()

		evt := DiscoveryEvent{Radio: info, Lost: false}
		select {
		case ch <- evt:
		case <-ctx.Done():
			return
		}
	}
}

// parseDiscoveryPacket parses a raw UDP datagram into a RadioInfo.
// Returns false when the packet lacks a serial number.
func parseDiscoveryPacket(data []byte, sender net.Addr) (DiscoveredRadio, bool) {
	kvs := make(map[string]string)
	for _, token := range strings.Fields(string(data)) {
		if eq := strings.IndexByte(token, '='); eq >= 0 {
			kvs[strings.ToLower(token[:eq])] = token[eq+1:]
		}
	}

	serial := kvs["serial"]
	if serial == "" {
		return DiscoveredRadio{}, false
	}

	ip := kvs["ip"]
	if ip == "" {
		// Fall back to the UDP sender address.
		if udpAddr, ok := sender.(*net.UDPAddr); ok {
			ip = udpAddr.IP.String()
		}
	}

	port := uint16(discoveryPort)
	if p, err := strconv.ParseUint(kvs["port"], 10, 16); err == nil {
		port = uint16(p)
	}

	maxLic, _ := strconv.Atoi(kvs["max_licensed_version"])
	inUse := kvs["inuse"] == "1"

	return DiscoveredRadio{
		Name:               kvs["name"],
		Model:              kvs["model"],
		Serial:             serial,
		Version:            kvs["version"],
		Nickname:           cleanDEL(kvs["nickname"]),
		Callsign:           kvs["callsign"],
		Address:            ip,
		Port:               port,
		Status:             kvs["status"],
		MaxLicensedVersion: maxLic,
		InUse:              inUse,
		GuiClientStations:  splitClean(kvs["gui_client_stations"]),
		GuiClientHandles:   splitClean(kvs["gui_client_handles"]),
		GuiClientPrograms:  splitClean(kvs["gui_client_programs"]),
	}, true
}

// cleanDEL replaces the 0x7F (DEL) characters used as delimiters in some
// SmartSDR fields with a space, matching AetherSDR's sanitisation.
func cleanDEL(s string) string {
	return strings.ReplaceAll(s, "\x7f", " ")
}

// splitClean splits a comma-separated field and sanitises each element.
func splitClean(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(cleanDEL(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
