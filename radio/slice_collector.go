package radio

import (
	"strings"
	"sync"
)

// SliceCollector gathers status updates for slices.
type SliceCollector struct {
	mu     sync.Mutex
	slices map[string]map[string]string // sliceID -> properties
}

// NewSliceCollector creates a new SliceCollector.
func NewSliceCollector() *SliceCollector {
	return &SliceCollector{slices: make(map[string]map[string]string)}
}

// HandleStatus processes a status message and updates slice state if applicable.
func (sc *SliceCollector) HandleStatus(msg ParsedMessage) {
	if msg.Type != MsgStatus {
		return
	}
	if !strings.HasPrefix(msg.Object, "slice ") {
		return
	}
	parts := strings.SplitN(msg.Object, " ", 2)
	if len(parts) != 2 {
		return
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sliceID := parts[1]
	if sc.slices[sliceID] == nil {
		sc.slices[sliceID] = make(map[string]string)
	}
	for k, v := range msg.KVs {
		sc.slices[sliceID][k] = v
	}
}

// GetSlices returns a deep copy of the collected slices.
func (sc *SliceCollector) GetSlices() map[string]map[string]string {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	result := make(map[string]map[string]string, len(sc.slices))
	for id, props := range sc.slices {
		cp := make(map[string]string, len(props))
		for k, v := range props {
			cp[k] = v
		}
		result[id] = cp
	}
	return result
}
