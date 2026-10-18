package main

import (
	"bytes"
	"errors"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMagicPacket(t *testing.T) {
	packet, err := magicPacket("aa-bb-cc-dd-ee-ff")
	if err != nil {
		t.Fatal(err)
	}
	if len(packet) != 102 || !bytes.Equal(packet[:6], bytes.Repeat([]byte{255}, 6)) {
		t.Fatalf("invalid packet: %x", packet)
	}
	for i := 6; i < len(packet); i += 6 {
		if !bytes.Equal(packet[i:i+6], []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}) {
			t.Fatalf("invalid repetition at %d", i)
		}
	}
	for _, mac := range []string{"", "invalid", "aa:bb:cc:dd:ee:ff:00:11"} {
		if _, err := magicPacket(mac); err == nil {
			t.Errorf("accepted %q", mac)
		}
	}
}

func TestDestination(t *testing.T) {
	for _, tc := range []struct{ ip, mask, want string }{
		{"", "", "255.255.255.255"},
		{"192.168.1.100", "", "192.168.1.100"},
		{"192.168.1.100", "255.255.255.0", "192.168.1.255"},
		{"10.20.3.5", "255.255.252.0", "10.20.3.255"},
		{"10.0.0.1", "255.255.255.252", "10.0.0.3"},
		{"", "255.255.255.0", ""},
		{"999.1.1.1", "", ""},
		{"::1", "", ""},
		{"10.0.0.1", "255.0.255.0", ""},
		{"10.0.0.1", "255.255.255.255", ""},
	} {
		got, err := destination(tc.ip, tc.mask)
		if tc.want == "" {
			if err == nil {
				t.Errorf("accepted %+v", tc)
			}
			continue
		}
		if err != nil || got.String() != tc.want {
			t.Errorf("%+v: got %v, %v", tc, got, err)
		}
	}
}

func TestWakeHandler(t *testing.T) {
	for _, tc := range []struct {
		name, method, body string
		sendErr            bool
		status, calls      int
	}{
		{"valid", "POST", `{"mac":"AA:BB:CC:DD:EE:FF","ip":"192.168.1.100","mask":"255.255.255.0"}`, false, 200, 1},
		{"default broadcast", "POST", `{"mac":"AA:BB:CC:DD:EE:FF"}`, false, 200, 1},
		{"bad mac", "POST", `{"mac":"bad"}`, false, 400, 0},
		{"bad ip", "POST", `{"mac":"AA:BB:CC:DD:EE:FF","ip":"bad"}`, false, 400, 0},
		{"missing ip", "POST", `{"mac":"AA:BB:CC:DD:EE:FF","mask":"255.255.255.0"}`, false, 400, 0},
		{"trailing JSON", "POST", `{"mac":"AA:BB:CC:DD:EE:FF"}{}`, false, 400, 0},
		{"unknown field", "POST", `{"mac":"AA:BB:CC:DD:EE:FF","other":1}`, false, 400, 0},
		{"oversized", "POST", strings.Repeat("x", 1025), false, 400, 0},
		{"method", "GET", "", false, 405, 0},
		{"send error", "POST", `{"mac":"AA:BB:CC:DD:EE:FF"}`, true, 500, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			handler := wakeHandler(func(packet []byte, ip net.IP) error {
				calls++
				if len(packet) != 102 {
					t.Errorf("packet length %d", len(packet))
				}
				if tc.name == "valid" && ip.String() != "192.168.1.255" {
					t.Errorf("destination %s", ip)
				}
				if tc.sendErr {
					return errors.New("network unavailable")
				}
				return nil
			})
			w := httptest.NewRecorder()
			handler(w, httptest.NewRequest(tc.method, "/api/wake", strings.NewReader(tc.body)))
			if w.Code != tc.status || calls != tc.calls {
				t.Fatalf("status %d, sends %d; want %d, %d", w.Code, calls, tc.status, tc.calls)
			}
		})
	}
}
