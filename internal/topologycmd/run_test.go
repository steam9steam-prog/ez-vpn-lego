package topologycmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	topologyxray "github.com/steam9steam-prog/ez-vpn-lego/drivers/xray/topology"
)

func TestWorkbenchLifecycle(t *testing.T) {
	directory := t.TempDir()
	spec := filepath.Join(directory, "installation.json")
	state := filepath.Join(directory, "state.json")
	content, err := os.ReadFile("../../examples/cascade.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(spec, content, 0o600); err != nil {
		t.Fatal(err)
	}
	invoke := func(args ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v: %s", args, stderr.String())
		}
		return stdout.String()
	}
	invoke("topology", "plan", spec)
	invoke("topology", "init", spec, state)
	invoke("topology", "grant", spec, state, "phone", "My phone", "uk", "us", "ca")
	var before topologyxray.Material
	if err := readJSON(state, &before, true); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(directory, "generation-one")
	result := invoke("topology", "render", spec, state, first)
	if strings.Contains(result, "vless://") || strings.Contains(result, before.Devices[0].SubscriptionToken) {
		t.Fatal("private access printed to command output")
	}
	subscriptionPath := filepath.Join(first, "subscriptions", before.Devices[0].SubscriptionToken)
	subscription, err := os.ReadFile(subscriptionPath)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(subscription)))
	if err != nil || strings.Count(string(decoded), "vless://") != 3 {
		t.Fatalf("invalid subscription: %v", err)
	}
	invoke("topology", "extend", spec, state)
	var after topologyxray.Material
	if err := readJSON(state, &after, true); err != nil {
		t.Fatal(err)
	}
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	if !bytes.Equal(beforeJSON, afterJSON) {
		t.Fatal("extend rotated existing material")
	}
	invoke("topology", "device", state, "phone", "disable")
	second := filepath.Join(directory, "generation-two")
	invoke("topology", "render", spec, state, second)
	if entries, err := os.ReadDir(filepath.Join(second, "subscriptions")); err != nil || len(entries) != 0 {
		t.Fatalf("disabled subscription still present: %v", err)
	}
	for _, key := range before.Devices[0].Keys {
		config, _ := os.ReadFile(filepath.Join(second, "nodes", "entry.json"))
		if bytes.Contains(config, []byte(key)) {
			t.Fatal("disabled key still present")
		}
	}
	invoke("topology", "device", state, "phone", "enable")
	third := filepath.Join(directory, "generation-three")
	invoke("topology", "render", spec, state, third)
	restored, _ := os.ReadFile(filepath.Join(third, "subscriptions", before.Devices[0].SubscriptionToken))
	if !bytes.Equal(subscription, restored) {
		t.Fatal("reenabling changed the subscription")
	}
	for _, path := range []string{state, filepath.Join(first, "nodes", "entry.json"), subscriptionPath} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("insecure file mode on %s: %v", path, err)
		}
	}
}

func TestRejectsUnsafeFilesAndUnknownFields(t *testing.T) {
	directory := t.TempDir()
	private := filepath.Join(directory, "private.json")
	if err := os.WriteFile(private, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var material topologyxray.Material
	if err := readJSON(private, &material, true); err == nil {
		t.Fatal("world-readable state accepted")
	}
	link := filepath.Join(directory, "link.json")
	if err := os.Symlink(private, link); err != nil {
		t.Fatal(err)
	}
	if err := readJSON(link, &material, false); err == nil {
		t.Fatal("symlink accepted")
	}
	spec := filepath.Join(directory, "spec.json")
	content := []byte(`{"schema_version":1,"nodes":[],"routes":[],"hops":["a","b","c"]}`)
	if err := os.WriteFile(spec, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readInstallation(spec); err == nil {
		t.Fatal("unsupported topology fields silently ignored")
	}
}

func TestRefusesToOverwriteStateOrGeneration(t *testing.T) {
	directory := t.TempDir()
	state := filepath.Join(directory, "state.json")
	spec := "../../examples/direct.json"
	var stdout, stderr bytes.Buffer
	if Run([]string{"topology", "init", spec, state}, &stdout, &stderr) != 0 {
		t.Fatal(stderr.String())
	}
	before, _ := os.ReadFile(state)
	if Run([]string{"topology", "init", spec, state}, &stdout, &stderr) == 0 {
		t.Fatal("state overwritten")
	}
	after, _ := os.ReadFile(state)
	if !bytes.Equal(before, after) {
		t.Fatal("state changed after failed init")
	}
	if err := os.Mkdir(filepath.Join(directory, "existing"), 0o700); err != nil {
		t.Fatal(err)
	}
	if Run([]string{"topology", "render", spec, state, filepath.Join(directory, "existing")}, &stdout, &stderr) == 0 {
		t.Fatal("existing generation overwritten")
	}
}

func TestRouteUpdatePreservesLinkAndRevokesRemovedGrant(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state.json")
	spec := "../../examples/cascade.json"
	var stdout, stderr bytes.Buffer
	for _, args := range [][]string{
		{"topology", "init", spec, state},
		{"topology", "grant", spec, state, "phone", "Phone", "uk", "us"},
	} {
		if Run(args, &stdout, &stderr) != 0 {
			t.Fatal(stderr.String())
		}
	}
	var before topologyxray.Material
	if err := readJSON(state, &before, true); err != nil {
		t.Fatal(err)
	}
	if Run([]string{"topology", "routes", spec, state, "phone", "uk", "ca"}, &stdout, &stderr) != 0 {
		t.Fatal(stderr.String())
	}
	var after topologyxray.Material
	if err := readJSON(state, &after, true); err != nil {
		t.Fatal(err)
	}
	old, current := before.Devices[0], after.Devices[0]
	if current.SubscriptionToken != old.SubscriptionToken || current.Keys["uk"] != old.Keys["uk"] || current.Keys["ca"] == "" || current.Keys["us"] != "" {
		t.Fatal("route update did not preserve retained access and revoke the removed grant")
	}
}
