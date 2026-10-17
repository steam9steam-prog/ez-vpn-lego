package ports

import (
	"context"
	"encoding/json"

	"github.com/steam9steam-prog/ez-vpn-lego/core/topology"
)

// Backend defines configuration ownership, not just a web interface. A panel
// adapter must report its actual capabilities before an installation uses it.
type BackendCapabilities struct {
	Profiles      []string `json:"profiles"`
	Cascade       bool     `json:"cascade"`
	Subscriptions bool     `json:"subscriptions"`
	Revoke        bool     `json:"revoke"`
	LiveAccess    bool     `json:"live_access"`
}

type Backend interface {
	Name() topology.Ownership
	Capabilities(context.Context) (BackendCapabilities, error)
	Plan(context.Context, topology.Installation) (json.RawMessage, error)
}
