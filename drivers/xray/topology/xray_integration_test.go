package topologyxray

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/steam9steam-prog/ez-vpn-lego/drivers/xray/reality"
)

// This test uses only loopback. Distinct loopback source addresses distinguish
// a true exit from an accidental direct fallback on the entry node.
func TestXrayDirectAndCascadeTraffic(t *testing.T) {
	binary := os.Getenv("XRAY_BINARY")
	if binary == "" {
		t.Skip("XRAY_BINARY is not configured")
	}
	if _, err := os.Stat(binary); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	destination := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("site")) }))
	destination.EnableHTTP2 = true
	destination.Config.ErrorLog = log.New(io.Discard, "", 0)
	// Emulate a conventional TLS 1.3 target. Go's evolving default hybrid key
	// shares are not the behavior this pinned Xray interoperability test checks.
	destination.TLS = &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, CurvePreferences: []tls.CurveID{tls.X25519}}
	destination.StartTLS()
	defer destination.Close()
	site, _ := url.Parse(destination.URL)
	i := installation()
	i.Nodes[0].Address = "127.0.0.1"
	i.Nodes[1].Address = "127.0.0.1"
	i.Nodes[1].RelayPort = freePort(t)
	for index := range i.Nodes {
		i.Nodes[index].RealityTarget = site.Host
		i.Nodes[index].RealityServerName = "example.com"
	}
	entryPort := freePort(t)
	for index := range i.Routes {
		i.Routes[index].Port = entryPort
	}
	m := materialFor(t, i)
	bundle, err := Render(i, m)
	if err != nil {
		t.Fatal(err)
	}
	var exitCommand *exec.Cmd
	for _, nodeID := range []string{"exit", "entry"} {
		var configuration map[string]any
		if err := json.Unmarshal(bundle.Nodes[nodeID], &configuration); err != nil {
			t.Fatal(err)
		}
		for _, item := range configuration["outbounds"].([]any) {
			outbound := item.(map[string]any)
			if outbound["tag"] == "direct" {
				if nodeID == "exit" {
					outbound["sendThrough"] = "127.0.0.2"
				} else {
					outbound["sendThrough"] = "127.0.0.3"
				}
			}
		}
		encoded, _ := json.Marshal(configuration)
		process := startXray(t, ctx, binary, encoded)
		if nodeID == "exit" {
			exitCommand = process
		}
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		_, _ = w.Write([]byte(host))
	}))
	defer origin.Close()
	udpOrigin, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer udpOrigin.Close()
	go func() {
		buffer := make([]byte, 512)
		for {
			_, address, err := udpOrigin.ReadFrom(buffer)
			if err != nil {
				return
			}
			host, _, _ := net.SplitHostPort(address.String())
			_, _ = udpOrigin.WriteTo([]byte(host), address)
		}
	}()
	for _, route := range i.Routes {
		t.Run(route.ID, func(t *testing.T) {
			proxyPort := freePort(t)
			socksPort := freePort(t)
			outbound := relayOutbound("vpn", i.Nodes[0], m.Nodes["entry"], m.Devices[0].Keys[route.ID])
			outbound["settings"].(map[string]any)["vnext"].([]map[string]any)[0]["port"] = entryPort
			clientConfig, _ := json.Marshal(map[string]any{
				"log":       map[string]string{"loglevel": "warning"},
				"inbounds":  []map[string]any{{"listen": "127.0.0.1", "port": proxyPort, "protocol": "http"}, {"listen": "127.0.0.1", "port": socksPort, "protocol": "socks", "settings": map[string]any{"auth": "noauth", "udp": true}}},
				"outbounds": []map[string]any{outbound},
			})
			startXray(t, ctx, binary, clientConfig)
			proxyURL, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(int(proxyPort)))
			transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
			response, err := client.Get(origin.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, _ := io.ReadAll(response.Body)
			expected := "127.0.0.3"
			if route.ExitID != "" {
				expected = "127.0.0.2"
			}
			if string(body) != expected {
				t.Fatalf("egress = %q, expected %q; HTTP %d", body, expected, response.StatusCode)
			}
			if udp := socksUDP(t, socksPort, udpOrigin.LocalAddr().(*net.UDPAddr).Port); udp != expected {
				t.Fatalf("UDP egress = %q, expected %q", udp, expected)
			}
			if route.ExitID != "" {
				if err := exitCommand.Process.Kill(); err != nil {
					t.Fatal(err)
				}
				failed, err := client.Get(origin.URL)
				if err == nil {
					defer failed.Body.Close()
					if failed.StatusCode < 400 {
						t.Fatal("failed exit unexpectedly carried traffic")
					}
				}
			}
		})
	}
}

func startXray(t *testing.T, ctx context.Context, binary string, configuration []byte) *exec.Cmd {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, configuration, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := reality.ValidateWithBinary(ctx, binary, path); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, binary, "run", "-config", path)
	// A file avoids racing a bytes.Buffer with the running process on failures.
	logPath := filepath.Join(t.TempDir(), "xray.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	command.Stdout, command.Stderr = logFile, logFile
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
		_ = logFile.Close()
		if t.Failed() {
			data, _ := os.ReadFile(logPath)
			// Logs contain synthetic credentials only, never an operator state.
			t.Log(string(data))
		}
	})
	var decoded struct {
		Inbounds []struct {
			Port uint16 `json:"port"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(configuration, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, inbound := range decoded.Inbounds {
		address := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(inbound.Port)))
		deadline := time.Now().Add(5 * time.Second)
		ready := false
		for time.Now().Before(deadline) {
			connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
			if err == nil {
				_ = connection.Close()
				ready = true
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if !ready {
			t.Fatalf("Xray did not listen on %s", address)
		}
	}
	return command
}

func socksUDP(t *testing.T, proxyPort uint16, originPort int) string {
	t.Helper()
	connection, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(proxyPort))), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := connection.Write([]byte{5, 1, 0}); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 2)
	if _, err := io.ReadFull(connection, response); err != nil || !bytes.Equal(response, []byte{5, 0}) {
		t.Fatalf("SOCKS auth: %v %v", response, err)
	}
	if _, err := connection.Write([]byte{5, 3, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 4)
	if _, err := io.ReadFull(connection, header); err != nil || header[1] != 0 {
		t.Fatalf("SOCKS associate: %v %v", header, err)
	}
	addressLength := 4
	if header[3] == 4 {
		addressLength = 16
	} else if header[3] != 1 {
		t.Fatal("unexpected SOCKS address type")
	}
	address := make([]byte, addressLength+2)
	if _, err := io.ReadFull(connection, address); err != nil {
		t.Fatal(err)
	}
	port := binary.BigEndian.Uint16(address[addressLength:])
	udp, err := net.DialTimeout("udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	_ = udp.SetDeadline(time.Now().Add(5 * time.Second))
	packet := []byte{0, 0, 0, 1, 127, 0, 0, 1, byte(originPort >> 8), byte(originPort), 'p', 'r', 'o', 'b', 'e'}
	if _, err := udp.Write(packet); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 512)
	count, err := udp.Read(buffer)
	if err != nil {
		t.Fatal(err)
	}
	if count < 10 || buffer[3] != 1 {
		t.Fatal("invalid UDP reply")
	}
	return string(buffer[10:count])
}

func freePort(t *testing.T) uint16 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return uint16(port)
}

func TestXrayAcceptsRevokedGeneration(t *testing.T) {
	binary := os.Getenv("XRAY_BINARY")
	if binary == "" {
		t.Skip("XRAY_BINARY is not configured")
	}
	i := installation()
	m := materialFor(t, i)
	m.Devices[0].Enabled = false
	bundle, err := Render(i, m)
	if err != nil {
		t.Fatal(err)
	}
	for id, config := range bundle.Nodes {
		path := filepath.Join(t.TempDir(), id+".json")
		if err := os.WriteFile(path, config, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := reality.ValidateWithBinary(context.Background(), binary, path); err != nil {
			t.Fatal(err)
		}
		for _, uuid := range m.Devices[0].Keys {
			if bytes.Contains(config, []byte(uuid)) {
				t.Fatal("revoked credential is present in Xray configuration")
			}
		}
	}
}

func TestXrayXHTTPTLSConfiguration(t *testing.T) {
	binary := os.Getenv("XRAY_BINARY")
	if binary == "" {
		t.Skip("XRAY_BINARY is not configured")
	}
	i := installation()
	for index := range i.Routes {
		i.Routes[index].Profile = "vless-xhttp-tls"
		i.Routes[index].Domain = "vpn.example.com"
	}
	m := materialFor(t, i)
	bundle, err := Render(i, m)
	if err != nil {
		t.Fatal(err)
	}
	site := httptest.NewTLSServer(http.NotFoundHandler())
	defer site.Close()
	certificates := t.TempDir()
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: site.Certificate().Raw})
	private, err := x509.MarshalPKCS8PrivateKey(site.TLS.Certificates[0].PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(certificates, "fullchain.pem"), certificate, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(certificates, "privkey.pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), 0o600); err != nil {
		t.Fatal(err)
	}
	for id, config := range bundle.Nodes {
		// Test files replace the deployment certificate directory, never OS files.
		config = bytes.ReplaceAll(config, []byte("/etc/ez-vpn-lego/certificates/vpn.example.com"), []byte(certificates))
		path := filepath.Join(t.TempDir(), id+".json")
		if err := os.WriteFile(path, config, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := reality.ValidateWithBinary(context.Background(), binary, path); err != nil {
			t.Fatal(err)
		}
	}
	for _, uri := range bundle.Subscriptions["phone"] {
		parsed, err := url.Parse(uri)
		if err != nil || parsed.Hostname() != "vpn.example.com" || parsed.Query().Get("security") != "tls" || parsed.Query().Get("pbk") != "" {
			t.Fatal("invalid TLS export")
		}
	}
}

func TestXrayXHTTPRealityConfiguration(t *testing.T) {
	binary := os.Getenv("XRAY_BINARY")
	if binary == "" {
		t.Skip("XRAY_BINARY is not configured")
	}
	i := installation()
	for index := range i.Routes {
		i.Routes[index].Profile = "vless-xhttp-reality"
	}
	m := materialFor(t, i)
	bundle, err := Render(i, m)
	if err != nil {
		t.Fatal(err)
	}
	for id, config := range bundle.Nodes {
		path := filepath.Join(t.TempDir(), id+".json")
		if err := os.WriteFile(path, config, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := reality.ValidateWithBinary(context.Background(), binary, path); err != nil {
			t.Fatal(err)
		}
	}
	for _, uri := range bundle.Subscriptions["phone"] {
		parsed, err := url.Parse(uri)
		if err != nil || parsed.Query().Get("flow") != "" || parsed.Query().Get("type") != "xhttp" || !strings.Contains(parsed.Query().Get("path"), "/lego") {
			t.Fatal(fmt.Sprintf("invalid XHTTP export %s: %v", uri, err))
		}
	}
}
