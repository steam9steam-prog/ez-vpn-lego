// Package topology describes one independent VPN installation. It does not
// depend on Telegram, a panel, an operating system, or a configuration renderer.
package topology

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Role string

const (
	Control Role = "control"
	Entry   Role = "entry"
	Exit    Role = "exit"
)

type Ownership string

const (
	Native    Ownership = "lego"
	Remnawave Ownership = "remnawave"
	Marzban   Ownership = "marzban"
)

// Node can combine roles. Control is never a traffic hop in its own right.
type Node struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Address           string    `json:"address"`
	Roles             []Role    `json:"roles"`
	Owner             Ownership `json:"owner"`
	Country           string    `json:"country,omitempty"`
	RelayPort         uint16    `json:"relay_port,omitempty"`
	RealityTarget     string    `json:"reality_target,omitempty"`
	RealityServerName string    `json:"reality_server_name,omitempty"`
}

// Route has exactly one traffic node, or an entry followed by an exit.
// Empty ExitID means a direct route. Route depth is deliberately not extensible.
type Route struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	EntryID string `json:"entry_id"`
	ExitID  string `json:"exit_id,omitempty"`
	Profile string `json:"profile"`
	Port    uint16 `json:"port"`
	Domain  string `json:"domain,omitempty"`
}

type Installation struct {
	SchemaVersion int     `json:"schema_version"`
	Nodes         []Node  `json:"nodes"`
	Routes        []Route `json:"routes"`
}

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,79}$`)

func ValidID(value string) bool { return identifier.MatchString(value) }

func (n Node) HasRole(role Role) bool { return slices.Contains(n.Roles, role) }

func (i Installation) Node(id string) (Node, bool) {
	for _, node := range i.Nodes {
		if node.ID == id {
			return node, true
		}
	}
	return Node{}, false
}

func (i Installation) Validate() error {
	if i.SchemaVersion != 1 {
		return errors.New("unsupported installation schema_version; expected 1")
	}
	if len(i.Nodes) == 0 {
		return errors.New("at least one node is required")
	}
	if i.Routes == nil {
		return errors.New("routes must be an array; use an empty array for an installation without routes")
	}
	nodes := make(map[string]Node, len(i.Nodes))
	controls := 0
	for _, node := range i.Nodes {
		if !ValidID(node.ID) || !validName(node.Name) {
			return fmt.Errorf("node %q: ID and name must contain 1 to 80 characters; ID must use letters, numbers, dot, dash or underscore", node.ID)
		}
		if _, exists := nodes[node.ID]; exists {
			return fmt.Errorf("duplicate node ID %q", node.ID)
		}
		if err := ValidateHost(node.Address); err != nil {
			return fmt.Errorf("node %q: %w", node.ID, err)
		}
		if (node.RealityTarget == "") != (node.RealityServerName == "") {
			return fmt.Errorf("node %q: reality_target and reality_server_name must be set together", node.ID)
		}
		if node.RealityTarget != "" {
			host, port, err := net.SplitHostPort(node.RealityTarget)
			if err != nil || ValidateHost(host) != nil || !validPort(port) || ValidateDomain(node.RealityServerName) != nil {
				return fmt.Errorf("node %q: invalid REALITY target or server name", node.ID)
			}
		}
		if node.Owner != Native && node.Owner != Remnawave && node.Owner != Marzban {
			return fmt.Errorf("node %q: owner must be lego, remnawave or marzban", node.ID)
		}
		if len(node.Roles) == 0 {
			return fmt.Errorf("node %q: at least one role is required", node.ID)
		}
		seen := make(map[Role]bool)
		for _, role := range node.Roles {
			if role != Control && role != Entry && role != Exit || seen[role] {
				return fmt.Errorf("node %q: invalid or duplicate role %q", node.ID, role)
			}
			seen[role] = true
		}
		if node.HasRole(Control) {
			controls++
		}
		nodes[node.ID] = node
	}
	if controls != 1 {
		return errors.New("exactly one node must have the control role; it may also carry traffic")
	}
	routes := make(map[string]bool, len(i.Routes))
	listeners := make(map[string]string)
	for _, route := range i.Routes {
		if !ValidID(route.ID) || !validName(route.Name) || !ValidID(route.Profile) || route.Port == 0 {
			return fmt.Errorf("route %q: valid ID, name, profile and nonzero port are required", route.ID)
		}
		if routes[route.ID] {
			return fmt.Errorf("duplicate route ID %q", route.ID)
		}
		routes[route.ID] = true
		entry, exists := nodes[route.EntryID]
		if !exists || !entry.HasRole(Entry) {
			return fmt.Errorf("route %q: entry_id must refer to a node with the entry role", route.ID)
		}
		if route.ExitID != "" {
			exit, exists := nodes[route.ExitID]
			if !exists || !exit.HasRole(Exit) || route.ExitID == route.EntryID {
				return fmt.Errorf("route %q: exit_id must refer to a different node with the exit role", route.ID)
			}
			if exit.Owner != entry.Owner {
				return fmt.Errorf("route %q: one backend must own both ends of a cascade", route.ID)
			}
		}
		if route.Domain != "" {
			if err := ValidateDomain(route.Domain); err != nil {
				return fmt.Errorf("route %q: %w", route.ID, err)
			}
		}
		key := fmt.Sprintf("%s:%d", route.EntryID, route.Port)
		settings := route.Profile + "|" + route.Domain
		if previous, exists := listeners[key]; exists && previous != settings {
			return fmt.Errorf("route %q: port %d on %q already uses another profile or domain", route.ID, route.Port, route.EntryID)
		}
		listeners[key] = settings
	}
	return nil
}

func validPort(port string) bool {
	value, err := strconv.ParseUint(port, 10, 16)
	return err == nil && value > 0
}

func validName(name string) bool {
	return name == strings.TrimSpace(name) && len([]rune(name)) >= 1 && len([]rune(name)) <= 80 && !strings.ContainsAny(name, "\r\n\x00")
}

func ValidateHost(host string) error {
	if _, err := netip.ParseAddr(host); err == nil && !strings.Contains(host, "%") {
		return nil
	}
	return ValidateDomain(host)
}

func ValidateDomain(domain string) error {
	if domain != strings.ToLower(domain) || len(domain) > 253 || net.ParseIP(domain) != nil || !strings.Contains(domain, ".") {
		return errors.New("use a lowercase DNS name such as vpn.example.com, without a scheme, port or path")
	}
	for _, label := range strings.Split(domain, ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New("invalid DNS label")
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return errors.New("DNS names must use ASCII or punycode")
			}
		}
	}
	return nil
}
