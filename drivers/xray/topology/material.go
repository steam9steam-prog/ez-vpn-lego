package topologyxray

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/steam9steam-prog/ez-vpn-lego/core/topology"
	"github.com/steam9steam-prog/ez-vpn-lego/drivers/xray/reality"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/id"
	"github.com/steam9steam-prog/ez-vpn-lego/services/planner"
)

// Material is private installation state. Reuse it when changing a topology:
// generating it again intentionally changes every server key and access.
type Material struct {
	SchemaVersion int                   `json:"schema_version"`
	Nodes         map[string]NodeSecret `json:"nodes"`
	Relays        map[string]string     `json:"relays"`
	Devices       []Device              `json:"devices"`
}

type NodeSecret struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	ShortID    string `json:"short_id"`
}

type Device struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Enabled           bool              `json:"enabled"`
	Keys              map[string]string `json:"keys"`
	SubscriptionToken string            `json:"subscription_token"`
}

func NewMaterial(installation topology.Installation) (Material, error) {
	material := Material{SchemaVersion: 1, Nodes: map[string]NodeSecret{}, Relays: map[string]string{}, Devices: []Device{}}
	return ExtendMaterial(installation, material)
}

// ExtendMaterial only issues keys for new nodes, routes and existing grants.
// A route removed from the installation is not exported or rendered, even if
// its old material remains available for an explicit operator rollback.
func ExtendMaterial(installation topology.Installation, material Material) (Material, error) {
	if _, err := planner.Build(installation); err != nil {
		return Material{}, err
	}
	if material.SchemaVersion != 1 || material.Nodes == nil || material.Relays == nil {
		return Material{}, errors.New("unsupported or incomplete private material")
	}
	for _, node := range installation.Nodes {
		if !node.HasRole(topology.Entry) && !node.HasRole(topology.Exit) {
			continue
		}
		if _, exists := material.Nodes[node.ID]; exists {
			continue
		}
		keys, err := reality.GenerateKeyPair()
		if err != nil {
			return Material{}, err
		}
		shortID, err := reality.GenerateShortID()
		if err != nil {
			return Material{}, err
		}
		material.Nodes[node.ID] = NodeSecret{PrivateKey: keys.Private, PublicKey: keys.Public, ShortID: shortID}
	}
	for _, route := range installation.Routes {
		if route.ExitID != "" && material.Relays[route.ID] == "" {
			value, err := id.NewUUID()
			if err != nil {
				return Material{}, err
			}
			material.Relays[route.ID] = value
		}
	}
	return material, nil
}

// AddDevice grants access to an explicit route set. Adding another route later
// does not silently grant it to existing devices.
func AddDevice(installation topology.Installation, material *Material, deviceID, name string, routeIDs []string) error {
	if material == nil {
		return errors.New("private material is required")
	}
	if !topology.ValidID(deviceID) || strings.TrimSpace(name) != name || strings.ContainsAny(name, "\r\n\x00") || len([]rune(name)) < 1 || len([]rune(name)) > 80 || len(routeIDs) == 0 {
		return errors.New("a valid device ID, name and at least one route are required")
	}
	for _, existing := range material.Devices {
		if existing.ID == deviceID {
			return errors.New("device already exists; use its existing private material")
		}
	}
	device := Device{ID: deviceID, Name: name, Enabled: true, Keys: map[string]string{}}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return err
	}
	device.SubscriptionToken = base64.RawURLEncoding.EncodeToString(token)
	for _, routeID := range routeIDs {
		if !slices.ContainsFunc(installation.Routes, func(route topology.Route) bool { return route.ID == routeID }) {
			return fmt.Errorf("unknown route %q", routeID)
		}
		if _, exists := device.Keys[routeID]; exists {
			return fmt.Errorf("duplicate route %q", routeID)
		}
		value, err := id.NewUUID()
		if err != nil {
			return err
		}
		device.Keys[routeID] = value
	}
	material.Devices = append(material.Devices, device)
	return nil
}

// SetDeviceRoutes preserves credentials for routes that remain granted and
// issues credentials only for new grants. A removed grant is actually revoked
// in the rendered configuration, without rotating the subscription token.
func SetDeviceRoutes(installation topology.Installation, material *Material, deviceID string, routeIDs []string) error {
	if material == nil || len(routeIDs) == 0 {
		return errors.New("private material and at least one route are required")
	}
	for index := range material.Devices {
		device := &material.Devices[index]
		if device.ID != deviceID {
			continue
		}
		keys := make(map[string]string, len(routeIDs))
		for _, routeID := range routeIDs {
			if !slices.ContainsFunc(installation.Routes, func(route topology.Route) bool { return route.ID == routeID }) {
				return fmt.Errorf("unknown route %q", routeID)
			}
			if _, duplicate := keys[routeID]; duplicate {
				return fmt.Errorf("duplicate route %q", routeID)
			}
			key := device.Keys[routeID]
			if key == "" {
				var err error
				key, err = id.NewUUID()
				if err != nil {
					return err
				}
			}
			keys[routeID] = key
		}
		device.Keys = keys
		return nil
	}
	return errors.New("device not found")
}
