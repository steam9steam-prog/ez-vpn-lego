// Package topologyxray renders direct and two-node routes. Every cascade has a
// dedicated authenticated inter-node link; user-selected countries are routed
// by credential identity, never by an untrusted name supplied by a client.
package topologyxray

import (
	"crypto/ecdh"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/steam9steam-prog/ez-vpn-lego/core/topology"
	"github.com/steam9steam-prog/ez-vpn-lego/profiles"
	"github.com/steam9steam-prog/ez-vpn-lego/services/planner"
)

const realityServer = "www.microsoft.com"

type Bundle struct {
	Nodes         map[string]json.RawMessage `json:"nodes"`
	Subscriptions map[string][]string        `json:"subscriptions"`
}

type xconfig struct {
	Log       map[string]string `json:"log"`
	Inbounds  []map[string]any  `json:"inbounds"`
	Outbounds []map[string]any  `json:"outbounds"`
	Routing   xrouting          `json:"routing"`
}

type xrouting struct {
	DomainStrategy string           `json:"domainStrategy"`
	Rules          []map[string]any `json:"rules"`
}

// Render is pure: no filesystem writes, processes, DNS lookups or key issuance.
func Render(installation topology.Installation, material Material) (Bundle, error) {
	plan, err := planner.Build(installation)
	if err != nil {
		return Bundle{}, err
	}
	if err := validateMaterial(installation, material); err != nil {
		return Bundle{}, err
	}
	bundle := Bundle{Nodes: map[string]json.RawMessage{}, Subscriptions: map[string][]string{}}
	for _, item := range plan.Nodes {
		if !item.Node.HasRole(topology.Entry) && !item.Node.HasRole(topology.Exit) {
			continue
		}
		configuration := xconfig{
			Log:      map[string]string{"loglevel": "warning"},
			Inbounds: []map[string]any{},
			// Fail closed unless an explicit rule below selects an egress.
			Outbounds: []map[string]any{{"tag": "block", "protocol": "blackhole"}, {"tag": "direct", "protocol": "freedom"}},
			Routing:   xrouting{DomainStrategy: "AsIs", Rules: []map[string]any{}},
		}
		secret := material.Nodes[item.Node.ID]
		listeners := map[string]map[string]any{}
		for _, route := range item.Routes {
			listenerTag := fmt.Sprintf("client-%d", route.Port)
			listener, exists := listeners[listenerTag]
			if !exists {
				listener = inbound(listenerTag, route.Profile, route.Port, route.Domain, item.Node, secret, []map[string]any{})
				listeners[listenerTag] = listener
			}
			clients := listener["settings"].(map[string]any)["clients"].([]map[string]any)
			outboundTag := "direct"
			if route.ExitID != "" {
				outboundTag = "route-" + route.ID
				exit, _ := installation.Node(route.ExitID)
				configuration.Outbounds = append(configuration.Outbounds, relayOutbound(outboundTag, exit, material.Nodes[exit.ID], material.Relays[route.ID]))
			}
			for _, device := range sortedDevices(material.Devices) {
				credential, granted := device.Keys[route.ID]
				if !device.Enabled || !granted {
					continue
				}
				email := device.ID + "@" + route.ID + ".lego.invalid"
				client := map[string]any{"id": credential, "email": email}
				if route.Profile == "vless-reality-vision" {
					client["flow"] = "xtls-rprx-vision"
				}
				clients = append(clients, client)
				configuration.Routing.Rules = append(configuration.Routing.Rules, map[string]any{"type": "field", "user": []string{email}, "inboundTag": []string{listenerTag}, "outboundTag": outboundTag})
			}
			listener["settings"].(map[string]any)["clients"] = clients
		}
		listenerTags := make([]string, 0, len(listeners))
		for tag := range listeners {
			listenerTags = append(listenerTags, tag)
		}
		slices.Sort(listenerTags)
		for _, tag := range listenerTags {
			configuration.Inbounds = append(configuration.Inbounds, listeners[tag])
		}
		if item.Relay {
			clients := []map[string]any{}
			routes := slices.Clone(installation.Routes)
			slices.SortFunc(routes, func(a, b topology.Route) int { return strings.Compare(a.ID, b.ID) })
			for _, route := range routes {
				if route.ExitID == item.Node.ID {
					clients = append(clients, map[string]any{"id": material.Relays[route.ID], "flow": "xtls-rprx-vision", "email": "relay." + route.ID + "@lego.invalid"})
				}
			}
			configuration.Inbounds = append(configuration.Inbounds, inbound("relay", "vless-reality-vision", planner.NodeRelayPort(item.Node), "", item.Node, secret, clients))
			configuration.Routing.Rules = append(configuration.Routing.Rules, map[string]any{"type": "field", "inboundTag": []string{"relay"}, "outboundTag": "direct"})
		}
		content, err := json.MarshalIndent(configuration, "", "  ")
		if err != nil {
			return Bundle{}, err
		}
		bundle.Nodes[item.Node.ID] = append(content, '\n')
	}
	for _, device := range sortedDevices(material.Devices) {
		if !device.Enabled {
			continue
		}
		uris := []string{}
		for _, route := range installation.Routes {
			if credential, granted := device.Keys[route.ID]; granted {
				node, _ := installation.Node(route.EntryID)
				uri, err := ShareURI(node, route, material.Nodes[node.ID], credential)
				if err != nil {
					return Bundle{}, err
				}
				uris = append(uris, uri)
			}
		}
		bundle.Subscriptions[device.ID] = uris
	}
	return bundle, nil
}

func inbound(tag, profile string, port uint16, domain string, node topology.Node, secret NodeSecret, clients []map[string]any) map[string]any {
	return map[string]any{
		"tag": tag, "listen": "::", "port": port, "protocol": "vless",
		"settings":       map[string]any{"clients": clients, "decryption": "none"},
		"streamSettings": serverStream(profile, domain, node, secret),
	}
}

func serverStream(profile, domain string, node topology.Node, secret NodeSecret) map[string]any {
	stream := map[string]any{"network": "raw", "security": "reality"}
	if profile != "vless-reality-vision" {
		stream["network"] = "xhttp"
		stream["xhttpSettings"] = map[string]any{"path": "/lego", "mode": "packet-up"}
	}
	if profile == "vless-xhttp-tls" {
		stream["security"] = "tls"
		stream["tlsSettings"] = map[string]any{"minVersion": "1.2", "alpn": []string{"h2", "http/1.1"}, "certificates": []map[string]any{{
			"certificateFile": "/etc/ez-vpn-lego/certificates/" + domain + "/fullchain.pem",
			"keyFile":         "/etc/ez-vpn-lego/certificates/" + domain + "/privkey.pem",
		}}}
	} else {
		target, name := realitySettings(node)
		stream["realitySettings"] = map[string]any{"show": false, "target": target, "xver": 0, "serverNames": []string{name}, "privateKey": secret.PrivateKey, "shortIds": []string{secret.ShortID}}
	}
	return stream
}

func relayOutbound(tag string, node topology.Node, secret NodeSecret, credential string) map[string]any {
	_, name := realitySettings(node)
	return map[string]any{
		"tag": tag, "protocol": "vless",
		"settings":       map[string]any{"vnext": []map[string]any{{"address": node.Address, "port": planner.NodeRelayPort(node), "users": []map[string]any{{"id": credential, "encryption": "none", "flow": "xtls-rprx-vision"}}}}},
		"streamSettings": map[string]any{"network": "raw", "security": "reality", "realitySettings": map[string]any{"serverName": name, "fingerprint": "chrome", "publicKey": secret.PublicKey, "shortId": secret.ShortID}},
	}
}

func ShareURI(node topology.Node, route topology.Route, secret NodeSecret, credential string) (string, error) {
	profile, err := profiles.Find(route.Profile)
	if err != nil {
		return "", err
	}
	query := url.Values{"encryption": {"none"}, "security": {profile.Security}, "type": {profile.Transport}, "fp": {"chrome"}}
	address := node.Address
	if profile.Security == "reality" {
		_, name := realitySettings(node)
		query.Set("sni", name)
		query.Set("pbk", secret.PublicKey)
		query.Set("sid", secret.ShortID)
	} else {
		address = route.Domain
		query.Set("sni", route.Domain)
		query.Set("alpn", "h2,http/1.1")
	}
	if profile.Transport == "xhttp" {
		query.Set("path", "/lego")
		query.Set("mode", "packet-up")
	} else {
		query.Set("type", "tcp")
		query.Set("flow", "xtls-rprx-vision")
	}
	return "vless://" + credential + "@" + net.JoinHostPort(address, strconv.Itoa(int(route.Port))) + "?" + query.Encode() + "#" + url.PathEscape(route.Name), nil
}

func realitySettings(node topology.Node) (string, string) {
	if node.RealityTarget != "" {
		return node.RealityTarget, node.RealityServerName
	}
	return realityServer + ":443", realityServer
}

func validateMaterial(installation topology.Installation, material Material) error {
	if material.SchemaVersion != 1 {
		return errors.New("unsupported private material schema_version")
	}
	seen := map[string]bool{}
	publicKeys := map[string]bool{}
	for _, node := range installation.Nodes {
		if !node.HasRole(topology.Entry) && !node.HasRole(topology.Exit) {
			continue
		}
		secret, exists := material.Nodes[node.ID]
		if !exists {
			return fmt.Errorf("missing private material for node %q", node.ID)
		}
		key, err := base64.RawURLEncoding.DecodeString(secret.PrivateKey)
		if err != nil {
			return fmt.Errorf("invalid private key for node %q", node.ID)
		}
		private, err := ecdh.X25519().NewPrivateKey(key)
		if err != nil || base64.RawURLEncoding.EncodeToString(private.PublicKey().Bytes()) != secret.PublicKey {
			return fmt.Errorf("key pair mismatch for node %q", node.ID)
		}
		if publicKeys[secret.PublicKey] {
			return errors.New("each node must have its own REALITY key pair")
		}
		publicKeys[secret.PublicKey] = true
		short, err := hex.DecodeString(secret.ShortID)
		if err != nil || len(short) != 8 {
			return fmt.Errorf("invalid short ID for node %q", node.ID)
		}
	}
	for _, route := range installation.Routes {
		if route.ExitID != "" {
			if err := uniqueUUID(material.Relays[route.ID], seen); err != nil {
				return fmt.Errorf("relay %q: %w", route.ID, err)
			}
		}
	}
	deviceIDs := map[string]bool{}
	tokens := map[string]bool{}
	for _, device := range material.Devices {
		if !topology.ValidID(device.ID) || deviceIDs[device.ID] {
			return errors.New("invalid or duplicate device ID")
		}
		deviceIDs[device.ID] = true
		token, err := base64.RawURLEncoding.DecodeString(device.SubscriptionToken)
		if err != nil || len(token) != 32 || tokens[device.SubscriptionToken] {
			return errors.New("invalid or duplicate subscription token")
		}
		tokens[device.SubscriptionToken] = true
		for _, credential := range device.Keys {
			if err := uniqueUUID(credential, seen); err != nil {
				return fmt.Errorf("device %q: %w", device.ID, err)
			}
		}
	}
	return nil
}

func uniqueUUID(value string, seen map[string]bool) error {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return errors.New("invalid UUID")
	}
	if raw, err := hex.DecodeString(strings.ReplaceAll(value, "-", "")); err != nil || len(raw) != 16 {
		return errors.New("invalid UUID")
	}
	if seen[value] {
		return errors.New("credential UUID must not be shared between devices or routes")
	}
	seen[value] = true
	return nil
}

func sortedDevices(devices []Device) []Device {
	result := slices.Clone(devices)
	slices.SortFunc(result, func(a, b Device) int { return strings.Compare(a.ID, b.ID) })
	return result
}
