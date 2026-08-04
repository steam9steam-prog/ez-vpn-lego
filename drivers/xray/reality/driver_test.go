package reality

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderIsDeterministicAndDoesNotLeakPublicMetadata(t *testing.T) {
	keys, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	state := State{Settings: Settings{
		Listen: "0.0.0.0", Port: 443, Target: "example.com:443",
		ServerNames: []string{"example.com"}, PrivateKey: keys.Private, LogLevel: "warning",
	}, Credentials: []Credential{
		{ID: "device-b", UUID: "00000000-0000-4000-8000-000000000002", ShortID: "2222222222222222", Label: "B"},
		{ID: "device-a", UUID: "00000000-0000-4000-8000-000000000001", ShortID: "1111111111111111", Label: "A"},
	}}
	first, err := Render(state)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(state)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("rendered configuration is not deterministic")
	}
	if !json.Valid(first) || !strings.Contains(string(first), `"xtls-rprx-vision"`) {
		t.Fatalf("unexpected configuration: %s", first)
	}
}

func TestShareURI(t *testing.T) {
	uri, err := ShareURI("203.0.113.1", 443, "public-key", "example.com", Credential{
		ID: "device", UUID: "00000000-0000-4000-8000-000000000001",
		ShortID: "0123456789abcdef", Label: "Alice phone",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"vless://", "security=reality", "flow=xtls-rprx-vision", "Alice%20phone"} {
		if !strings.Contains(uri, part) {
			t.Fatalf("URI %q does not contain %q", uri, part)
		}
	}
}
