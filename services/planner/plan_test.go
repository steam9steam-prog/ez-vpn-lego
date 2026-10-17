package planner

import (
	"context"
	"fmt"
	"net/netip"
	"reflect"
	"slices"
	"testing"

	"github.com/steam9steam-prog/ez-vpn-lego/core/topology"
)

func sample() topology.Installation {
	return topology.Installation{SchemaVersion: 1, Nodes: []topology.Node{
		{ID: "control", Name: "Control", Address: "192.0.2.1", Roles: []topology.Role{topology.Control}, Owner: topology.Native},
		{ID: "entry", Name: "Entry", Address: "192.0.2.2", Roles: []topology.Role{topology.Entry}, Owner: topology.Native},
		{ID: "exit", Name: "Exit", Address: "2001:db8::3", Roles: []topology.Role{topology.Exit}, Owner: topology.Native},
	}, Routes: []topology.Route{
		{ID: "direct", Name: "Direct", EntryID: "entry", Profile: "vless-reality-vision", Port: 443},
		{ID: "cascade", Name: "Cascade", EntryID: "entry", ExitID: "exit", Profile: "vless-reality-vision", Port: 443},
	}}
}

func TestSupportedTopologiesAndControlPlacement(t *testing.T) {
	installation := sample()
	plan, err := Build(installation)
	if err != nil || len(plan.Domains) != 0 || len(plan.Nodes) != 3 || plan.ControlNodeID != "control" {
		t.Fatalf("separate control: %+v %v", plan, err)
	}
	installation.Nodes = installation.Nodes[1:]
	installation.Nodes[0].Roles = append(installation.Nodes[0].Roles, topology.Control)
	plan, err = Build(installation)
	if err != nil || plan.ControlNodeID != "entry" {
		t.Fatalf("combined control: %+v %v", plan, err)
	}
	installation.Nodes[0].Roles = []topology.Role{topology.Entry}
	installation.Nodes[1].Roles = append(installation.Nodes[1].Roles, topology.Control)
	if _, err := Build(installation); err != nil {
		t.Fatalf("control on exit: %v", err)
	}
}

func TestNoFixedNodeLimit(t *testing.T) {
	installation := sample()
	for index := range 1000 {
		id := fmt.Sprintf("exit-%d", index)
		installation.Nodes = append(installation.Nodes, topology.Node{ID: id, Name: id, Address: "192.0.2.3", Roles: []topology.Role{topology.Exit}, Owner: topology.Native})
		installation.Routes = append(installation.Routes, topology.Route{ID: id, Name: id, EntryID: "entry", ExitID: id, Profile: "vless-reality-vision", Port: 443})
	}
	if _, err := Build(installation); err != nil {
		t.Fatal(err)
	}
}

func TestPlanIsIndependentOfInputOrder(t *testing.T) {
	i := sample()
	first, err := Build(i)
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(i.Nodes)
	slices.Reverse(i.Routes)
	second, err := Build(i)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("input ordering changed the plan: %v", err)
	}
}

func TestOneTLSDomainCanServeMultipleExits(t *testing.T) {
	i := sample()
	for index := range i.Routes {
		i.Routes[index].Profile = "vless-xhttp-tls"
		i.Routes[index].Domain = "vpn.example.com"
	}
	plan, err := Build(i)
	if err != nil || len(plan.Domains) != 1 {
		t.Fatalf("shared TLS domain: %+v %v", plan.Domains, err)
	}
	i.Nodes[2].Roles = append(i.Nodes[2].Roles, topology.Entry)
	i.Routes[0].EntryID = "exit"
	if _, err := Build(i); err == nil {
		t.Fatal("one TLS name assigned to two different entries")
	}
}

func TestDNSAcceptsExpectedDualStackRecords(t *testing.T) {
	resolver := fakeResolver{
		"entry.example.com": {netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("2001:db8::2")},
		"vpn.example.com":   {netip.MustParseAddr("2001:db8::2"), netip.MustParseAddr("192.0.2.2")},
	}
	result := CheckDNS(context.Background(), resolver, DomainRequirement{Name: "vpn.example.com", Address: "entry.example.com"})
	if !result.Ready {
		t.Fatalf("valid dual-stack DNS rejected: %+v", result)
	}
}

func TestInvalidPlans(t *testing.T) {
	tests := []struct {
		name string
		edit func(*topology.Installation)
	}{
		{"missing control", func(i *topology.Installation) { i.Nodes[0].Roles = []topology.Role{topology.Exit} }},
		{"two controls", func(i *topology.Installation) { i.Nodes[1].Roles = append(i.Nodes[1].Roles, topology.Control) }},
		{"duplicate node", func(i *topology.Installation) { i.Nodes = append(i.Nodes, i.Nodes[0]) }},
		{"self loop", func(i *topology.Installation) { i.Routes[1].ExitID = "entry" }},
		{"missing exit", func(i *topology.Installation) { i.Routes[1].ExitID = "missing" }},
		{"wrong role", func(i *topology.Installation) { i.Routes[1].ExitID = "control" }},
		{"panel ownership", func(i *topology.Installation) { i.Nodes[0].Owner = topology.Remnawave }},
		{"unknown profile", func(i *topology.Installation) { i.Routes[0].Profile = "made-up" }},
		{"missing domain", func(i *topology.Installation) { i.Routes[0].Profile = "vless-xhttp-tls" }},
		{"port collision", func(i *topology.Installation) { i.Routes[0].Profile = "vless-xhttp-reality" }},
		{"duplicate route", func(i *topology.Installation) { i.Routes = append(i.Routes, i.Routes[0]) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installation := sample()
			test.edit(&installation)
			if _, err := Build(installation); err == nil {
				t.Fatal("invalid installation accepted")
			}
		})
	}
}

type fakeResolver map[string][]netip.Addr

func (r fakeResolver) LookupNetIP(_ context.Context, _, name string) ([]netip.Addr, error) {
	addresses, exists := r[name]
	if !exists {
		return nil, fmt.Errorf("not found")
	}
	return addresses, nil
}

func TestDNSRejectsStaleAAAAAndProxy(t *testing.T) {
	requirement := DomainRequirement{Name: "vpn.example.com", Address: "192.0.2.2"}
	for _, test := range []struct {
		name  string
		ips   []string
		ready bool
	}{
		{"correct", []string{"192.0.2.2"}, true},
		{"stale AAAA", []string{"192.0.2.2", "2001:db8::99"}, false},
		{"proxy", []string{"192.0.2.99"}, false},
		{"missing", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolver := fakeResolver{}
			for _, value := range test.ips {
				resolver[requirement.Name] = append(resolver[requirement.Name], netip.MustParseAddr(value))
			}
			if result := CheckDNS(context.Background(), resolver, requirement); result.Ready != test.ready {
				t.Fatalf("unexpected DNS check: %+v", result)
			}
		})
	}
}
