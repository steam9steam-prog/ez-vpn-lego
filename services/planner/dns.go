package planner

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strings"
)

type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type DNSResult struct {
	Domain   string   `json:"domain"`
	Expected []string `json:"expected"`
	Actual   []string `json:"actual"`
	Ready    bool     `json:"ready"`
	Message  string   `json:"message"`
}

// CheckDNS compares every A and AAAA answer. A correct A record does not make a
// stale AAAA record safe. A reverse proxy/CDN is not silently treated as origin.
func CheckDNS(ctx context.Context, resolver Resolver, requirement DomainRequirement) DNSResult {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	result := DNSResult{Domain: requirement.Name, Expected: []string{}, Actual: []string{}}
	expected, err := resolveAddress(ctx, resolver, requirement.Address)
	if err != nil {
		result.Message = "cannot resolve the node address"
		return result
	}
	for _, address := range expected {
		result.Expected = append(result.Expected, address.Unmap().String())
	}
	slices.Sort(result.Expected)
	actual, err := resolver.LookupNetIP(ctx, "ip", requirement.Name)
	if err != nil || len(actual) == 0 {
		result.Message = "create an A or AAAA record pointing to the entry node, then check again"
		return result
	}
	result.Ready = true
	for _, address := range actual {
		value := address.Unmap().String()
		result.Actual = append(result.Actual, value)
		if !slices.Contains(result.Expected, value) {
			result.Ready = false
		}
	}
	slices.Sort(result.Actual)
	result.Actual = slices.Compact(result.Actual)
	if result.Ready {
		result.Message = "DNS points to the entry; certificate issuance and operator reachability still need a live check"
	} else {
		result.Message = fmt.Sprintf("all A/AAAA records must point to %s; remove stale records and disable CDN proxying for this profile", strings.Join(result.Expected, ", "))
	}
	return result
}

func resolveAddress(ctx context.Context, resolver Resolver, address string) ([]netip.Addr, error) {
	if parsed, err := netip.ParseAddr(address); err == nil {
		return []netip.Addr{parsed}, nil
	}
	return resolver.LookupNetIP(ctx, "ip", address)
}
