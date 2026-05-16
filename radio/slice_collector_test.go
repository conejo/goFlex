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
