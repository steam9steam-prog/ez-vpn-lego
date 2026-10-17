// Package profiles keeps compatibility facts separate from configuration code.
package profiles

import (
	"errors"
	"fmt"
)

type Profile struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Engine      string   `json:"engine"`
	Transport   string   `json:"transport"`
	Security    string   `json:"security"`
	NeedsDomain bool     `json:"needs_domain"`
	Clients     []string `json:"clients"`
	Status      string   `json:"status"`
	Evidence    string   `json:"evidence"`
	Sources     []string `json:"sources"`
}

// Status describes support in Lego. It is not a promise of reachability from
// every operator in a country. Such evidence belongs to dated field reports.
func Catalog() []Profile {
	return []Profile{
		{ID: "vless-reality-vision", Name: "VLESS REALITY Vision", Engine: "xray", Transport: "raw", Security: "reality", Clients: []string{"Happ", "Throne", "Shadowrocket"}, Status: "implemented", Evidence: "Local configuration and traffic tests. Operator reachability must be checked separately.", Sources: []string{"https://xtls.github.io/config/transport.html", "https://github.com/XTLS/REALITY"}},
		{ID: "vless-xhttp-reality", Name: "VLESS XHTTP REALITY", Engine: "xray", Transport: "xhttp", Security: "reality", Clients: []string{"Happ", "Throne"}, Status: "experimental", Evidence: "Requires a client with compatible XHTTP support. No country-wide claim.", Sources: []string{"https://github.com/XTLS/Xray-core/discussions/4113"}},
		{ID: "vless-xhttp-tls", Name: "VLESS XHTTP TLS", Engine: "xray", Transport: "xhttp", Security: "tls", NeedsDomain: true, Clients: []string{"Happ", "Throne", "Shadowrocket"}, Status: "experimental", Evidence: "Requires a domain, valid certificate and compatible XHTTP client. No country-wide claim.", Sources: []string{"https://github.com/XTLS/Xray-core/discussions/4113"}},
	}
}

func Find(id string) (Profile, error) {
	for _, profile := range Catalog() {
		if profile.ID == id {
			return profile, nil
		}
	}
	return Profile{}, fmt.Errorf("unknown profile %q", id)
}

func (p Profile) Validate(domain string) error {
	if p.NeedsDomain && domain == "" {
		return errors.New("this profile requires a domain owned by the operator")
	}
	return nil
}
