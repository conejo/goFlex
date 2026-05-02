// protocol_test.go — unit tests for the SmartSDR wire protocol parser.

package radio

import (
	"reflect"
	"testing"
)

func TestParseLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		line string
		want ParsedMessage
	}{
		{"version", "V3.3.28.0", ParsedMessage{Type: msgVersion, Object: "3.3.28.0"}},
		{"handle", "H0A1B2C3D", ParsedMessage{Type: msgHandle, Handle: 0x0A1B2C3D}},
		{"response OK", "R1|0|", ParsedMessage{Type: msgResponse, Sequence: 1, ResultCode: 0, Object: "", KVs: map[string]string{}}},
		{"response error", "R2|50001001|No Such Object", ParsedMessage{Type: msgResponse, Sequence: 2, ResultCode: 0x50001001, Object: "No Such Object", KVs: map[string]string{"No": "", "Such": "", "Object": ""}}},
		{"response with KVs", "R3|0|freq=14.225000 mode=USB", ParsedMessage{Type: msgResponse, Sequence: 3, ResultCode: 0, Object: "freq=14.225000 mode=USB", KVs: map[string]string{"freq": "14.225000", "mode": "USB"}}},
		{"status simple", "S0A1B2C3D|slice 0 freq=14.225000 mode=USB", ParsedMessage{Type: msgStatus, Handle: 0x0A1B2C3D, Object: "slice 0", KVs: map[string]string{"freq": "14.225000", "mode": "USB"}}},
		{"status multi-word object", "S0A1B2C3D|display pan 0x40000000 x=100 y=200", ParsedMessage{Type: msgStatus, Handle: 0x0A1B2C3D, Object: "display pan 0x40000000", KVs: map[string]string{"x": "100", "y": "200"}}},
		{"status no KVs", "S0A1B2C3D|slice 0 removed", ParsedMessage{Type: msgStatus, Handle: 0x0A1B2C3D, Object: "slice 0 removed"}},
		{"status only KVs", "S0A1B2C3D|freq=14.225000 mode=USB", ParsedMessage{Type: msgStatus, Handle: 0x0A1B2C3D, Object: "", KVs: map[string]string{"freq": "14.225000", "mode": "USB"}}},
		{"status empty body", "S0A1B2C3D|", ParsedMessage{Type: msgStatus, Handle: 0x0A1B2C3D}},
		{"status no pipe", "S0A1B2C3D slice 0 freq=14.225000", ParsedMessage{Type: msgStatus}},
		{"empty line", "", ParsedMessage{Type: msgUnknown, Raw: ""}},
		{"whitespace only", "   \t\n  ", ParsedMessage{Type: msgUnknown, Raw: ""}},
		{"unknown tag", "Xsome random data", ParsedMessage{Type: msgUnknown, Raw: "Xsome random data"}},
		{"version with spaces", "  V3.3.28.0  ", ParsedMessage{Type: msgVersion, Object: "3.3.28.0"}},
		{"handle lowercase", "H0a1b2c3d", ParsedMessage{Type: msgHandle, Handle: 0x0A1B2C3D}},
		{"response negative code", "R1|FFFFFFFF|Error", ParsedMessage{Type: msgResponse, Sequence: 1, ResultCode: 2147483647, Object: "Error", KVs: map[string]string{"Error": ""}}},
		{"status negative values", "S0A1B2C3D|slice 0 filter_lo=-1500 filter_hi=1500", ParsedMessage{Type: msgStatus, Handle: 0x0A1B2C3D, Object: "slice 0", KVs: map[string]string{"filter_lo": "-1500", "filter_hi": "1500"}}},
		{"status equals in value", "S0A1B2C3D|slice 0 mysetting=a=b=c", ParsedMessage{Type: msgStatus, Handle: 0x0A1B2C3D, Object: "slice 0", KVs: map[string]string{"mysetting": "a=b=c"}}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
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

func TestParseKVs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want map[string]string
	}{
		{"empty", "", map[string]string{}},
		{"bare words", "foo bar baz", map[string]string{"foo": "", "bar": "", "baz": ""}},
		{"mixed", "foo bar=1 baz", map[string]string{"foo": "", "bar": "1", "baz": ""}},
		{"multiple equals", "key=a=b=c", map[string]string{"key": "a=b=c"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseKVs(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, got)
			}
		})
	}
}

func TestParseLine_RealWorldExamples(t *testing.T) {
	t.Parallel()
	cases := []struct {
		line string
		want ParsedMessage
	}{
		{"V3.4.24.0", ParsedMessage{Type: msgVersion, Object: "3.4.24.0"}},
		{"H00000001", ParsedMessage{Type: msgHandle, Handle: 1}},
		{"R42|0|", ParsedMessage{Type: msgResponse, Sequence: 42, ResultCode: 0, KVs: map[string]string{}}},
		{"S00000001|slice 0 RF_frequency=14.225000", ParsedMessage{Type: msgStatus, Handle: 1, Object: "slice 0", KVs: map[string]string{"RF_frequency": "14.225000"}}},
		{"S00000001|display pan 0x40000000 min_db=-130 max_db=0", ParsedMessage{Type: msgStatus, Handle: 1, Object: "display pan 0x40000000", KVs: map[string]string{"min_db": "-130", "max_db": "0"}}},
		{"S00000001|radio ptt=1", ParsedMessage{Type: msgStatus, Handle: 1, Object: "radio", KVs: map[string]string{"ptt": "1"}}},
		{"S00000001|meter 0 name=SWR value=1.5", ParsedMessage{Type: msgStatus, Handle: 1, Object: "meter 0", KVs: map[string]string{"name": "SWR", "value": "1.5"}}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.line, func(t *testing.T) {
			t.Parallel()
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
