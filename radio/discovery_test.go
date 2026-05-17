package radio

import (
	"net"
	"reflect"
	"testing"
)

func TestParseDiscoveryPacket_Full(t *testing.T) {
	data := []byte("serial=ABC123 model=6600 version=3.3.28.0 ip=192.168.1.50 port=4992 status=Available name=Flex-6600 nickname=Shack\x7fRadio callsign=K1ABC max_licensed_version=3 inuse=0 gui_client_stations=Station1,Station2 gui_client_handles=0x1,0x2 gui_client_programs=AetherSDR,SmartSDR")
	sender := &net.UDPAddr{IP: net.ParseIP("192.168.1.50"), Port: 4992}

	info, ok := parseDiscoveryPacket(data, sender)
	if !ok {
		t.Fatal("expected packet to parse successfully")
	}

	if info.Serial != "ABC123" {
		t.Errorf("Serial = %q, want ABC123", info.Serial)
	}
	if info.Model != "6600" {
		t.Errorf("Model = %q, want 6600", info.Model)
	}
	if info.Version != "3.3.28.0" {
		t.Errorf("Version = %q, want 3.3.28.0", info.Version)
	}
	if info.Address != "192.168.1.50" {
		t.Errorf("Address = %q, want 192.168.1.50", info.Address)
	}
	if info.Port != 4992 {
		t.Errorf("Port = %d, want 4992", info.Port)
	}
	if info.Status != "Available" {
		t.Errorf("Status = %q, want Available", info.Status)
	}
	if info.Name != "Flex-6600" {
		t.Errorf("Name = %q, want Flex-6600", info.Name)
	}
	if info.Nickname != "Shack Radio" {
		t.Errorf("Nickname = %q, want 'Shack Radio'", info.Nickname)
	}
	if info.Callsign != "K1ABC" {
		t.Errorf("Callsign = %q, want K1ABC", info.Callsign)
	}
	if info.MaxLicensedVersion != 3 {
		t.Errorf("MaxLicensedVersion = %d, want 3", info.MaxLicensedVersion)
	}
	if info.InUse {
		t.Error("expected InUse=false")
	}
	wantStations := []string{"Station1", "Station2"}
	if !reflect.DeepEqual(info.GuiClientStations, wantStations) {
		t.Errorf("GuiClientStations = %v, want %v", info.GuiClientStations, wantStations)
	}
	wantHandles := []string{"0x1", "0x2"}
	if !reflect.DeepEqual(info.GuiClientHandles, wantHandles) {
		t.Errorf("GuiClientHandles = %v, want %v", info.GuiClientHandles, wantHandles)
	}
	wantPrograms := []string{"AetherSDR", "SmartSDR"}
	if !reflect.DeepEqual(info.GuiClientPrograms, wantPrograms) {
		t.Errorf("GuiClientPrograms = %v, want %v", info.GuiClientPrograms, wantPrograms)
	}
}

func TestParseDiscoveryPacket_NoSerial(t *testing.T) {
	data := []byte("model=6600 version=3.3.28.0")
	sender := &net.UDPAddr{IP: net.ParseIP("192.168.1.50")}

	_, ok := parseDiscoveryPacket(data, sender)
	if ok {
		t.Error("expected packet without serial to be rejected")
	}
}

func TestParseDiscoveryPacket_FallbackIP(t *testing.T) {
	data := []byte("serial=ABC123 model=6600")
	sender := &net.UDPAddr{IP: net.ParseIP("10.0.0.5"), Port: 4992}

	info, ok := parseDiscoveryPacket(data, sender)
	if !ok {
		t.Fatal("expected packet to parse")
	}
	if info.Address != "10.0.0.5" {
		t.Errorf("Address = %q, want 10.0.0.5 (fallback to sender)", info.Address)
	}
	if info.Port != uint16(discoveryPort) {
		t.Errorf("Port = %d, want default %d", info.Port, discoveryPort)
	}
}

func TestParseDiscoveryPacket_InUseTrue(t *testing.T) {
	data := []byte("serial=ABC123 inuse=1")
	sender := &net.UDPAddr{IP: net.ParseIP("192.168.1.50")}

	info, ok := parseDiscoveryPacket(data, sender)
	if !ok {
		t.Fatal("expected packet to parse")
	}
	if !info.InUse {
		t.Error("expected InUse=true")
	}
}

func TestParseDiscoveryPacket_EmptyFields(t *testing.T) {
	data := []byte("serial=ABC123")
	sender := &net.UDPAddr{IP: net.ParseIP("192.168.1.50")}

	info, ok := parseDiscoveryPacket(data, sender)
	if !ok {
		t.Fatal("expected packet to parse")
	}
	if info.Name != "" {
		t.Errorf("Name = %q, want empty", info.Name)
	}
	if info.Model != "" {
		t.Errorf("Model = %q, want empty", info.Model)
	}
	if len(info.GuiClientStations) != 0 {
		t.Errorf("GuiClientStations = %v, want empty", info.GuiClientStations)
	}
}

func TestCleanDEL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"hello\x7fworld", "hello world"},
		{"no delimiters", "no delimiters"},
		{"", ""},
		{"\x7f\x7f", "  "},
	}
	for _, tc := range cases {
		got := cleanDEL(tc.in)
		if got != tc.want {
			t.Errorf("cleanDEL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSplitClean(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{"  a  ,  b  ", []string{"a", "b"}},
		{"a\x7fb,c", []string{"a b", "c"}},
		{"", nil},
		{",,", []string{}},
		{"only", []string{"only"}},
	}
	for _, tc := range cases {
		got := splitClean(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitClean(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
