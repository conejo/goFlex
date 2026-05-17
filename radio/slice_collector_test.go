package radio

import (
	"testing"
)

func TestSliceCollector_HandleStatus_AddsSlice(t *testing.T) {
	sc := NewSliceCollector()

	msg := ParsedMessage{
		Type:   MsgStatus,
		Object: "slice 0",
		KVs:    map[string]string{"RF_frequency": "14.300", "mode": "USB"},
	}
	sc.HandleStatus(msg)

	slices := sc.GetSlices()
	if len(slices) != 1 {
		t.Fatalf("expected 1 slice, got %d", len(slices))
	}
	props := slices["0"]
	if props["RF_frequency"] != "14.300" {
		t.Errorf("RF_frequency: want 14.300, got %s", props["RF_frequency"])
	}
	if props["mode"] != "USB" {
		t.Errorf("mode: want USB, got %s", props["mode"])
	}
}

func TestSliceCollector_HandleStatus_UpdatesSlice(t *testing.T) {
	sc := NewSliceCollector()

	sc.HandleStatus(ParsedMessage{
		Type:   MsgStatus,
		Object: "slice 0",
		KVs:    map[string]string{"RF_frequency": "14.300"},
	})
	sc.HandleStatus(ParsedMessage{
		Type:   MsgStatus,
		Object: "slice 0",
		KVs:    map[string]string{"mode": "USB"},
	})

	slices := sc.GetSlices()
	props := slices["0"]
	if props["RF_frequency"] != "14.300" {
		t.Errorf("RF_frequency: want 14.300, got %s", props["RF_frequency"])
	}
	if props["mode"] != "USB" {
		t.Errorf("mode: want USB, got %s", props["mode"])
	}
}

func TestSliceCollector_HandleStatus_IgnoresNonStatus(t *testing.T) {
	sc := NewSliceCollector()

	sc.HandleStatus(ParsedMessage{
		Type:   MsgResponse,
		Object: "slice 0",
		KVs:    map[string]string{"RF_frequency": "14.300"},
	})

	slices := sc.GetSlices()
	if len(slices) != 0 {
		t.Fatalf("expected 0 slices for non-status message, got %d", len(slices))
	}
}

func TestSliceCollector_HandleStatus_IgnoresNonSlice(t *testing.T) {
	sc := NewSliceCollector()

	sc.HandleStatus(ParsedMessage{
		Type:   MsgStatus,
		Object: "radio",
		KVs:    map[string]string{"callsign": "K1ABC"},
	})

	slices := sc.GetSlices()
	if len(slices) != 0 {
		t.Fatalf("expected 0 slices for non-slice object, got %d", len(slices))
	}
}

func TestSliceCollector_HandleStatus_MultipleSlices(t *testing.T) {
	sc := NewSliceCollector()

	sc.HandleStatus(ParsedMessage{
		Type:   MsgStatus,
		Object: "slice 0",
		KVs:    map[string]string{"RF_frequency": "14.300"},
	})
	sc.HandleStatus(ParsedMessage{
		Type:   MsgStatus,
		Object: "slice 1",
		KVs:    map[string]string{"RF_frequency": "7.200"},
	})

	slices := sc.GetSlices()
	if len(slices) != 2 {
		t.Fatalf("expected 2 slices, got %d", len(slices))
	}
}

func TestSliceCollector_GetSlices_ReturnsDeepCopy(t *testing.T) {
	sc := NewSliceCollector()

	sc.HandleStatus(ParsedMessage{
		Type:   MsgStatus,
		Object: "slice 0",
		KVs:    map[string]string{"RF_frequency": "14.300"},
	})

	slices := sc.GetSlices()
	// Mutate the returned copy.
	slices["0"]["RF_frequency"] = "modified"

	// Original should be unchanged.
	original := sc.GetSlices()
	if original["0"]["RF_frequency"] != "14.300" {
		t.Errorf("original should be unchanged, got %s", original["0"]["RF_frequency"])
	}
}

func TestSliceCollector_Empty(t *testing.T) {
	sc := NewSliceCollector()
	slices := sc.GetSlices()
	if len(slices) != 0 {
		t.Fatalf("expected 0 slices for new collector, got %d", len(slices))
	}
}

func TestSliceCollector_HandleStatus_MalformedObject(t *testing.T) {
	sc := NewSliceCollector()
	// "slice" with no space and no ID — should be ignored.
	sc.HandleStatus(ParsedMessage{Type: MsgStatus, Object: "slice", KVs: map[string]string{"freq": "14.225"}})
	if len(sc.GetSlices()) != 0 {
		t.Error("expected no slices for malformed 'slice' object")
	}
}

func TestSliceCollector_HandleStatus_EmptyKVs(t *testing.T) {
	sc := NewSliceCollector()
	sc.HandleStatus(ParsedMessage{Type: MsgStatus, Object: "slice 0"})

	got := sc.GetSlices()
	if len(got) != 1 {
		t.Fatalf("expected 1 slice, got %d", len(got))
	}
	if len(got["0"]) != 0 {
		t.Errorf("expected empty props, got %v", got["0"])
	}
}

func TestSliceCollector_GetSlices_DeepCopy_OuterMap(t *testing.T) {
	sc := NewSliceCollector()
	sc.HandleStatus(ParsedMessage{Type: MsgStatus, Object: "slice 0", KVs: map[string]string{"freq": "14.225"}})

	got := sc.GetSlices()
	// Mutate the outer map — should not affect collector.
	got["new"] = map[string]string{"freq": "1.0"}

	got2 := sc.GetSlices()
	if _, ok := got2["new"]; ok {
		t.Error("deep copy failed: new key leaked into collector")
	}
}

func TestSliceCollector_ConcurrentAccess(t *testing.T) {
	sc := NewSliceCollector()
	// Run with -race to detect data races.
	go func() {
		for i := 0; i < 100; i++ {
			sc.HandleStatus(ParsedMessage{Type: MsgStatus, Object: "slice 0", KVs: map[string]string{"n": string(rune(i))}})
		}
	}()
	go func() {
		for i := 0; i < 100; i++ {
			_ = sc.GetSlices()
		}
	}()
	for i := 0; i < 100; i++ {
		sc.HandleStatus(ParsedMessage{Type: MsgStatus, Object: "slice 1", KVs: map[string]string{"n": string(rune(i))}})
		_ = sc.GetSlices()
	}
}
