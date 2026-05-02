// protocol_test.go — unit tests for the SmartSDR wire protocol parser.

package radio

import (
	"reflect"
	"testing"
)

func TestParseLine_Version(t *testing.T) {
	msg := parseLine("V3.3.28.0")
	if msg.Type != msgVersion {
		t.Fatalf("expected type msgVersion, got %v", msg.Type)
	}
	if msg.Object != "3.3.28.0" {
		t.Fatalf("expected Object='3.3.28.0', got %q", msg.Object)
	}
}

func TestParseLine_Handle(t *testing.T) {
	msg := parseLine("H0A1B2C3D")
	if msg.Type != msgHandle {
		t.Fatalf("expected type msgHandle, got %v", msg.Type)
	}
	if msg.Handle != 0x0A1B2C3D {
		t.Fatalf("expected Handle=0x0A1B2C3D, got 0x%X", msg.Handle)
	}
}

func TestParseLine_ResponseOK(t *testing.T) {
	msg := parseLine("R1|0|")
	if msg.Type != msgResponse {
		t.Fatalf("expected type msgResponse, got %v", msg.Type)
	}
	if msg.Sequence != 1 {
		t.Fatalf("expected Sequence=1, got %d", msg.Sequence)
	}
	if msg.ResultCode != 0 {
		t.Fatalf("expected ResultCode=0, got %d", msg.ResultCode)
	}
	if msg.Object != "" {
		t.Fatalf("expected empty Object, got %q", msg.Object)
	}
}

func TestParseLine_ResponseError(t *testing.T) {
	msg := parseLine("R2|50001001|No Such Object")
	if msg.Type != msgResponse {
		t.Fatalf("expected type msgResponse, got %v", msg.Type)
	}
	if msg.Sequence != 2 {
		t.Fatalf("expected Sequence=2, got %d", msg.Sequence)
	}
	if msg.ResultCode != 0x50001001 {
		t.Fatalf("expected ResultCode=0x50001001, got 0x%X", msg.ResultCode)
	}
	if msg.Object != "No Such Object" {
		t.Fatalf("expected Object='No Such Object', got %q", msg.Object)
	}
}

func TestParseLine_ResponseWithKVs(t *testing.T) {
	msg := parseLine("R3|0|freq=14.225000 mode=USB")
	if msg.Type != msgResponse {
		t.Fatalf("expected type msgResponse, got %v", msg.Type)
	}
	if msg.Sequence != 3 {
		t.Fatalf("expected Sequence=3, got %d", msg.Sequence)
	}
	if msg.ResultCode != 0 {
		t.Fatalf("expected ResultCode=0, got %d", msg.ResultCode)
	}
	wantKVs := map[string]string{
		"freq": "14.225000",
		"mode": "USB",
	}
	if !reflect.DeepEqual(msg.KVs, wantKVs) {
		t.Fatalf("expected KVs=%v, got %v", wantKVs, msg.KVs)
	}
}

func TestParseLine_StatusSimpleObject(t *testing.T) {
	msg := parseLine("S0A1B2C3D|slice 0 freq=14.225000 mode=USB")
	if msg.Type != msgStatus {
		t.Fatalf("expected type msgStatus, got %v", msg.Type)
	}
	if msg.Handle != 0x0A1B2C3D {
		t.Fatalf("expected Handle=0x0A1B2C3D, got 0x%X", msg.Handle)
	}
	if msg.Object != "slice 0" {
		t.Fatalf("expected Object='slice 0', got %q", msg.Object)
	}
	wantKVs := map[string]string{
		"freq": "14.225000",
		"mode": "USB",
	}
	if !reflect.DeepEqual(msg.KVs, wantKVs) {
		t.Fatalf("expected KVs=%v, got %v", wantKVs, msg.KVs)
	}
}

func TestParseLine_StatusMultiWordObject(t *testing.T) {
	// This is the critical case that AetherSDR fixed — multi-word object names
	// like "display pan 0x40000000" where the hex handle looks like it could be
	// a KV but is actually part of the object name.
	msg := parseLine("S0A1B2C3D|display pan 0x40000000 x=100 y=200")
	if msg.Type != msgStatus {
		t.Fatalf("expected type msgStatus, got %v", msg.Type)
	}
	if msg.Handle != 0x0A1B2C3D {
		t.Fatalf("expected Handle=0x0A1B2C3D, got 0x%X", msg.Handle)
	}
	if msg.Object != "display pan 0x40000000" {
		t.Fatalf("expected Object='display pan 0x40000000', got %q", msg.Object)
	}
	wantKVs := map[string]string{
		"x": "100",
		"y": "200",
	}
	if !reflect.DeepEqual(msg.KVs, wantKVs) {
		t.Fatalf("expected KVs=%v, got %v", wantKVs, msg.KVs)
	}
}

func TestParseLine_StatusNoKVs(t *testing.T) {
	msg := parseLine("S0A1B2C3D|slice 0 removed")
	if msg.Type != msgStatus {
		t.Fatalf("expected type msgStatus, got %v", msg.Type)
	}
	if msg.Handle != 0x0A1B2C3D {
		t.Fatalf("expected Handle=0x0A1B2C3D, got 0x%X", msg.Handle)
	}
	if msg.Object != "slice 0 removed" {
		t.Fatalf("expected Object='slice 0 removed', got %q", msg.Object)
	}
	if len(msg.KVs) != 0 {
		t.Fatalf("expected no KVs, got %v", msg.KVs)
	}
}

func TestParseLine_StatusOnlyKVs(t *testing.T) {
	// Edge case: status line with no object name, only key=value pairs
	msg := parseLine("S0A1B2C3D|freq=14.225000 mode=USB")
	if msg.Type != msgStatus {
		t.Fatalf("expected type msgStatus, got %v", msg.Type)
	}
	if msg.Handle != 0x0A1B2C3D {
		t.Fatalf("expected Handle=0x0A1B2C3D, got 0x%X", msg.Handle)
	}
	if msg.Object != "" {
		t.Fatalf("expected empty Object, got %q", msg.Object)
	}
	wantKVs := map[string]string{
		"freq": "14.225000",
		"mode": "USB",
	}
	if !reflect.DeepEqual(msg.KVs, wantKVs) {
		t.Fatalf("expected KVs=%v, got %v", wantKVs, msg.KVs)
	}
}

func TestParseLine_StatusEmptyBody(t *testing.T) {
	msg := parseLine("S0A1B2C3D|")
	if msg.Type != msgStatus {
		t.Fatalf("expected type msgStatus, got %v", msg.Type)
	}
	if msg.Handle != 0x0A1B2C3D {
		t.Fatalf("expected Handle=0x0A1B2C3D, got 0x%X", msg.Handle)
	}
	if msg.Object != "" {
		t.Fatalf("expected empty Object, got %q", msg.Object)
	}
	if len(msg.KVs) != 0 {
		t.Fatalf("expected no KVs, got %v", msg.KVs)
	}
}

func TestParseLine_StatusNoPipe(t *testing.T) {
	msg := parseLine("S0A1B2C3D slice 0 freq=14.225000")
	if msg.Type != msgStatus {
		t.Fatalf("expected type msgStatus, got %v", msg.Type)
	}
	// Without pipe, handle should be 0 and body unparsed
	if msg.Handle != 0 {
		t.Fatalf("expected Handle=0 when no pipe, got 0x%X", msg.Handle)
	}
	if msg.Object != "" {
		t.Fatalf("expected empty Object when no pipe, got %q", msg.Object)
	}
}

func TestParseLine_EmptyLine(t *testing.T) {
	msg := parseLine("")
	if msg.Type != msgUnknown {
		t.Fatalf("expected type msgUnknown for empty line, got %v", msg.Type)
	}
	if msg.Raw != "" {
		t.Fatalf("expected Raw='', got %q", msg.Raw)
	}
}

func TestParseLine_WhitespaceOnly(t *testing.T) {
	msg := parseLine("   \t\n  ")
	if msg.Type != msgUnknown {
		t.Fatalf("expected type msgUnknown for whitespace line, got %v", msg.Type)
	}
	if msg.Raw != "" {
		t.Fatalf("expected Raw='', got %q", msg.Raw)
	}
}

func TestParseLine_UnknownTag(t *testing.T) {
	msg := parseLine("Xsome random data")
	if msg.Type != msgUnknown {
		t.Fatalf("expected type msgUnknown for unknown tag, got %v", msg.Type)
	}
	if msg.Raw != "Xsome random data" {
		t.Fatalf("expected Raw='Xsome random data', got %q", msg.Raw)
	}
}

func TestParseLine_VersionWithSpaces(t *testing.T) {
	msg := parseLine("  V3.3.28.0  ")
	if msg.Type != msgVersion {
		t.Fatalf("expected type msgVersion, got %v", msg.Type)
	}
	if msg.Object != "3.3.28.0" {
		t.Fatalf("expected Object='3.3.28.0', got %q", msg.Object)
	}
}

func TestParseLine_HandleLowercase(t *testing.T) {
	msg := parseLine("H0a1b2c3d")
	if msg.Type != msgHandle {
		t.Fatalf("expected type msgHandle, got %v", msg.Type)
	}
	if msg.Handle != 0x0A1B2C3D {
		t.Fatalf("expected Handle=0x0A1B2C3D (lowercase hex), got 0x%X", msg.Handle)
	}
}

func TestParseLine_ResponseNegativeCode(t *testing.T) {
	// Result codes are parsed as hex with bitSize 32.  0xFFFFFFFF overflows
	// int32, so strconv.ParseInt returns ErrRange and the max int32 value.
	msg := parseLine("R1|FFFFFFFF|Error")
	if msg.Type != msgResponse {
		t.Fatalf("expected type msgResponse, got %v", msg.Type)
	}
	if msg.ResultCode != 2147483647 {
		t.Fatalf("expected ResultCode=2147483647 (max int32), got %d", msg.ResultCode)
	}
}

func TestParseLine_StatusWithNegativeValues(t *testing.T) {
	msg := parseLine("S0A1B2C3D|slice 0 filter_lo=-1500 filter_hi=1500")
	if msg.Type != msgStatus {
		t.Fatalf("expected type msgStatus, got %v", msg.Type)
	}
	if msg.Object != "slice 0" {
		t.Fatalf("expected Object='slice 0', got %q", msg.Object)
	}
	wantKVs := map[string]string{
		"filter_lo": "-1500",
		"filter_hi": "1500",
	}
	if !reflect.DeepEqual(msg.KVs, wantKVs) {
		t.Fatalf("expected KVs=%v, got %v", wantKVs, msg.KVs)
	}
}

func TestParseLine_StatusWithEqualsInValue(t *testing.T) {
	// Edge case: value contains '=' character
	msg := parseLine("S0A1B2C3D|slice 0 mysetting=a=b=c")
	if msg.Type != msgStatus {
		t.Fatalf("expected type msgStatus, got %v", msg.Type)
	}
	if msg.Object != "slice 0" {
		t.Fatalf("expected Object='slice 0', got %q", msg.Object)
	}
	wantKVs := map[string]string{
		"mysetting": "a=b=c",
	}
	if !reflect.DeepEqual(msg.KVs, wantKVs) {
		t.Fatalf("expected KVs=%v, got %v", wantKVs, msg.KVs)
	}
}

func TestParseKVs_EmptyString(t *testing.T) {
	kvs := parseKVs("")
	if len(kvs) != 0 {
		t.Fatalf("expected empty map, got %v", kvs)
	}
}

func TestParseKVs_BareWords(t *testing.T) {
	kvs := parseKVs("foo bar baz")
	want := map[string]string{
		"foo": "",
		"bar": "",
		"baz": "",
	}
	if !reflect.DeepEqual(kvs, want) {
		t.Fatalf("expected %v, got %v", want, kvs)
	}
}

func TestParseKVs_Mixed(t *testing.T) {
	kvs := parseKVs("foo bar=1 baz")
	want := map[string]string{
		"foo": "",
		"bar": "1",
		"baz": "",
	}
	if !reflect.DeepEqual(kvs, want) {
		t.Fatalf("expected %v, got %v", want, kvs)
	}
}

func TestParseKVs_MultipleEquals(t *testing.T) {
	// Only split on first '='
	kvs := parseKVs("key=a=b=c")
	want := map[string]string{
		"key": "a=b=c",
	}
	if !reflect.DeepEqual(kvs, want) {
		t.Fatalf("expected %v, got %v", want, kvs)
	}
}

func TestParseLine_RealWorldExamples(t *testing.T) {
	cases := []struct {
		line   string
		want   ParsedMessage
	}{
		{
			line: "V3.4.24.0",
			want: ParsedMessage{Type: msgVersion, Object: "3.4.24.0"},
		},
		{
			line: "H00000001",
			want: ParsedMessage{Type: msgHandle, Handle: 1},
		},
		{
			line: "R42|0|",
			want: ParsedMessage{Type: msgResponse, Sequence: 42, ResultCode: 0, KVs: map[string]string{}},
		},
		{
			line: "S00000001|slice 0 RF_frequency=14.225000",
			want: ParsedMessage{
				Type:   msgStatus,
				Handle: 1,
				Object: "slice 0",
				KVs:    map[string]string{"RF_frequency": "14.225000"},
			},
		},
		{
			line: "S00000001|display pan 0x40000000 min_db=-130 max_db=0",
			want: ParsedMessage{
				Type:   msgStatus,
				Handle: 1,
				Object: "display pan 0x40000000",
				KVs:    map[string]string{"min_db": "-130", "max_db": "0"},
			},
		},
		{
			line: "S00000001|radio ptt=1",
			want: ParsedMessage{
				Type:   msgStatus,
				Handle: 1,
				Object: "radio",
				KVs:    map[string]string{"ptt": "1"},
			},
		},
		{
			line: "S00000001|meter 0 name=SWR value=1.5",
			want: ParsedMessage{
				Type:   msgStatus,
				Handle: 1,
				Object: "meter 0",
				KVs:    map[string]string{"name": "SWR", "value": "1.5"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			got := parseLine(tc.line)
			if got.Type != tc.want.Type {
				t.Fatalf("Type: want %v, got %v", tc.want.Type, got.Type)
			}
			if got.Handle != tc.want.Handle {
				t.Fatalf("Handle: want 0x%X, got 0x%X", tc.want.Handle, got.Handle)
			}
			if got.Sequence != tc.want.Sequence {
				t.Fatalf("Sequence: want %d, got %d", tc.want.Sequence, got.Sequence)
			}
			if got.ResultCode != tc.want.ResultCode {
				t.Fatalf("ResultCode: want %d, got %d", tc.want.ResultCode, got.ResultCode)
			}
			if got.Object != tc.want.Object {
				t.Fatalf("Object: want %q, got %q", tc.want.Object, got.Object)
			}
			if !reflect.DeepEqual(got.KVs, tc.want.KVs) {
				t.Fatalf("KVs: want %v, got %v", tc.want.KVs, got.KVs)
			}
		})
	}
}
