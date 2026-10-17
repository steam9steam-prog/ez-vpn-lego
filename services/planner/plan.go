// Package planner turns an installation into a validated node plan without
// changing a server, issuing a certificate, or creating credentials.
package planner

import (
	"fmt"
	"slices"
	"strings"

	"github.com/steam9steam-prog/ez-vpn-lego/core/topology"
	"github.com/steam9steam-prog/ez-vpn-lego/profiles"
)

type DomainRequirement struct {
	Name    string `json:"name"`
	NodeID  string `json:"node_id"`
	Address string `json:"address"`
	Purpose string `json:"purpose"`
}

type Plan struct {
	SchemaVersion int                 `json:"schema_version"`
	ControlNodeID string              `json:"control_node_id"`
	Nodes         []NodePlan          `json:"nodes"`
	Domains       []DomainRequirement `json:"domains"`
	Warnings      []string            `json:"warnings"`
}

type NodePlan struct {
	Node   topology.Node    `json:"node"`
	Routes []topology.Route `json:"routes"`
	Relay  bool             `json:"relay"`
}

func Build(installation topology.Installation) (Plan, error) {
	if err := installation.Validate(); err != nil {
		return Plan{}, err
	}
	plan := Plan{SchemaVersion: 1, Nodes: []NodePlan{}, Domains: []DomainRequirement{}, Warnings: []string{}}
	nodes := slices.Clone(installation.Nodes)
	slices.SortFunc(nodes, func(a, b topology.Node) int { return strings.Compare(a.ID, b.ID) })
	indices := make(map[string]int, len(nodes))
	for _, node := range nodes {
		if node.HasRole(topology.Control) {
			plan.ControlNodeID = node.ID
		}
		if node.Owner != topology.Native {
			return Plan{}, fmt.Errorf("node %q: %s adapter is not installed; Lego will not take ownership of a panel-managed node", node.ID, node.Owner)
		}
		indices[node.ID] = len(plan.Nodes)
		plan.Nodes = append(plan.Nodes, NodePlan{Node: node, Routes: []topology.Route{}})
	}
	domains := make(map[string]string)
	routes := slices.Clone(installation.Routes)
	slices.SortFunc(routes, func(a, b topology.Route) int { return strings.Compare(a.ID, b.ID) })
	for _, route := range routes {
		profile, err := profiles.Find(route.Profile)
		if err != nil {
			return Plan{}, err
		}
		if err := profile.Validate(route.Domain); err != nil {
			return Plan{}, fmt.Errorf("route %q: %w", route.ID, err)
		}
		item := &plan.Nodes[indices[route.EntryID]]
		item.Routes = append(item.Routes, route)
		if route.ExitID != "" {
			plan.Nodes[indices[route.ExitID]].Relay = true
		}
		if profile.NeedsDomain {
			if owner, exists := domains[route.Domain]; exists && owner != item.Node.ID {
				return Plan{}, fmt.Errorf("domain %q points to more than one entry; use a separate hostname for each entry", route.Domain)
			}
			if _, exists := domains[route.Domain]; !exists {
				plan.Domains = append(plan.Domains, DomainRequirement{Name: route.Domain, NodeID: item.Node.ID, Address: item.Node.Address, Purpose: "vpn-tls"})
			}
			domains[route.Domain] = item.Node.ID
		}
		if profile.Status == "experimental" {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("route %s uses experimental profile %s; verify the installed client version", route.ID, route.Profile))
		}
	}
	for _, item := range plan.Nodes {
		if !item.Relay {
			continue
		}
		for _, route := range item.Routes {
			if route.Port == NodeRelayPort(item.Node) {
				return Plan{}, fmt.Errorf("node %q: port %d is reserved for incoming cascade links", item.Node.ID, NodeRelayPort(item.Node))
			}
		}
	}
	return plan, nil
}

// RelayPort is distinct from client listeners so one machine can be an entry
// for one route and an exit for another without turning either into three hops.
const RelayPort uint16 = 10443

func NodeRelayPort(node topology.Node) uint16 {
	if node.RelayPort != 0 {
		return node.RelayPort
	}
	return RelayPort
}
