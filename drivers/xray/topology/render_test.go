package topologyxray

import (
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/steam9steam-prog/ez-vpn-lego/core/topology"
)

func installation() topology.Installation {
	return topology.Installation{SchemaVersion: 1, Nodes: []topology.Node{
		{ID: "entry", Name: "Entry", Address: "192.0.2.1", Roles: []topology.Role{topology.Control, topology.Entry}, Owner: topology.Native},
		{ID: "exit", Name: "Exit", Address: "2001:db8::2", Roles: []topology.Role{topology.Exit}, Owner: topology.Native},
	}, Routes: []topology.Route{
		{ID: "direct", Name: "Direct", EntryID: "entry", Profile: "vless-reality-vision", Port: 443},
		{ID: "cascade", Name: "Via exit", EntryID: "entry", ExitID: "exit", Profile: "vless-reality-vision", Port: 443},
	}}
}

func materialFor(t *testing.T, i topology.Installation) Material {
	t.Helper()
	m, err := NewMaterial(i)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddDevice(i, &m, "phone", "Phone", []string{"direct", "cascade"}); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestRenderRoutesByCredentialAndFailsClosed(t *testing.T) {
	i := installation()
	m := materialFor(t, i)
	bundle, err := Render(i, m)
	if err != nil {
		t.Fatal(err)
	}
	var entry xconfig
	if err := json.Unmarshal(bundle.Nodes["entry"], &entry); err != nil {
		t.Fatal(err)
	}
	if len(entry.Inbounds) != 1 || entry.Outbounds[0]["protocol"] != "blackhole" || len(entry.Routing.Rules) != 2 {
		t.Fatalf("unexpected entry: %s", bundle.Nodes["entry"])
	}
	if entry.Routing.Rules[0]["outboundTag"] != "route-cascade" || entry.Routing.Rules[1]["outboundTag"] != "direct" {
		t.Fatalf("wrong selected exits: %+v", entry.Routing.Rules)
	}
	var exit xconfig
	if err := json.Unmarshal(bundle.Nodes["exit"], &exit); err != nil {
		t.Fatal(err)
	}
	if len(exit.Inbounds) != 1 || len(exit.Routing.Rules) != 1 || exit.Routing.Rules[0]["outboundTag"] != "direct" {
		t.Fatalf("unexpected exit: %s", bundle.Nodes["exit"])
	}
	for _, uri := range bundle.Subscriptions["phone"] {
		parsed, err := url.Parse(uri)
		if err != nil || parsed.Hostname() != "192.0.2.1" {
			t.Fatalf("client must connect only to entry: %s %v", uri, err)
		}
	}
}

func TestCredentialsSurviveAddingNodeAndRoute(t *testing.T) {
	i := installation()
	m := materialFor(t, i)
	before, err := Render(i, m)
	if err != nil {
		t.Fatal(err)
	}
	state, _ := json.Marshal(m)
	i.Nodes = append(i.Nodes, topology.Node{ID: "exit-two", Name: "Second exit", Address: "192.0.2.3", Roles: []topology.Role{topology.Exit}, Owner: topology.Native})
	i.Routes = append(i.Routes, topology.Route{ID: "new", Name: "New route", EntryID: "entry", ExitID: "exit-two", Profile: "vless-reality-vision", Port: 443})
	var copy Material
	_ = json.Unmarshal(state, &copy)
	afterMaterial, err := ExtendMaterial(i, copy)
	if err != nil {
		t.Fatal(err)
	}
	after, err := Render(i, afterMaterial)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Subscriptions, after.Subscriptions) || !reflect.DeepEqual(m.Devices, afterMaterial.Devices) || m.Nodes["entry"] != afterMaterial.Nodes["entry"] {
		t.Fatal("adding a route changed existing credentials or grants")
	}
	if err := SetDeviceRoutes(i, &afterMaterial, "phone", []string{"direct", "cascade", "new"}); err != nil {
		t.Fatal(err)
	}
	granted, err := Render(i, afterMaterial)
	if err != nil || len(granted.Subscriptions["phone"]) != 3 {
		t.Fatalf("new route grant failed: %v", err)
	}
	if !reflect.DeepEqual(before.Subscriptions["phone"], granted.Subscriptions["phone"][:2]) || m.Devices[0].SubscriptionToken != afterMaterial.Devices[0].SubscriptionToken {
		t.Fatal("adding a grant rotated an old credential or subscription token")
	}
}

func TestDeviceAndRouteIdentityCannotCollide(t *testing.T) {
	i := installation()
	i.Routes[0].ID = "b.c"
	i.Routes[1].ID = "c"
	m, err := NewMaterial(i)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddDevice(i, &m, "a.b", "First", []string{"c"}); err != nil {
		t.Fatal(err)
	}
	if err := AddDevice(i, &m, "a", "Second", []string{"b.c"}); err != nil {
		t.Fatal(err)
	}
	bundle, err := Render(i, m)
	if err != nil {
		t.Fatal(err)
	}
	var config xconfig
	if err := json.Unmarshal(bundle.Nodes["entry"], &config); err != nil {
		t.Fatal(err)
	}
	first, _ := json.Marshal(config.Routing.Rules[0]["user"])
	second, _ := json.Marshal(config.Routing.Rules[1]["user"])
	if string(first) == string(second) {
		t.Fatal("different device/route pairs have the same routing identity")
	}
}

func TestDisableRevokesImportedKeysAndSubscription(t *testing.T) {
	i := installation()
	m := materialFor(t, i)
	m.Devices[0].Enabled = false
	bundle, err := Render(i, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Subscriptions) != 0 {
		t.Fatal("disabled subscription exported")
	}
	for _, key := range m.Devices[0].Keys {
		if strings.Contains(string(bundle.Nodes["entry"]), key) {
			t.Fatal("disabled key is still accepted")
		}
	}
	m.Devices[0].Enabled = true
	restored, err := Render(i, m)
	if err != nil || len(restored.Subscriptions["phone"]) != 2 {
		t.Fatalf("enable failed: %v", err)
	}
}

func TestDuplicateAndCorruptMaterialRejected(t *testing.T) {
	i := installation()
	m := materialFor(t, i)
	m.Devices[0].Keys["direct"] = m.Devices[0].Keys["cascade"]
	if _, err := Render(i, m); err == nil {
		t.Fatal("shared key accepted")
	}
	m = materialFor(t, i)
	secret := m.Nodes["entry"]
	secret.PublicKey = "incorrect"
	m.Nodes["entry"] = secret
	if _, err := Render(i, m); err == nil {
		t.Fatal("mismatched key pair accepted")
	}
}

func TestRenderDeterministic(t *testing.T) {
	i := installation()
	m := materialFor(t, i)
	first, err := Render(i, m)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(i, m)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal("render changed without a state change")
	}
}
