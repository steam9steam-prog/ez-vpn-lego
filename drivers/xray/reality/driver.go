package reality

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

const Name = "xray-reality-vision"

type KeyPair struct {
	Private string
	Public  string
}

type Credential struct {
	ID      string
	UUID    string
	ShortID string
	Label   string
}

type Settings struct {
	Listen      string
	Port        uint16
	Target      string
	ServerNames []string
	PrivateKey  string
	LogLevel    string
}

type State struct {
	Settings    Settings
	Credentials []Credential
}

func GenerateKeyPair() (KeyPair, error) {
	private, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return KeyPair{}, fmt.Errorf("generate Reality key pair: %w", err)
	}
	return KeyPair{
		Private: base64.RawURLEncoding.EncodeToString(private.Bytes()),
		Public:  base64.RawURLEncoding.EncodeToString(private.PublicKey().Bytes()),
	}, nil
}

func GenerateShortID() (string, error) {
	value := make([]byte, 8)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate Reality short ID: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func Render(state State) ([]byte, error) {
	if err := validateState(state); err != nil {
		return nil, err
	}
	clients := make([]client, 0, len(state.Credentials))
	shortIDs := make([]string, 0, len(state.Credentials))
	for _, credential := range state.Credentials {
		clients = append(clients, client{ID: credential.UUID, Flow: "xtls-rprx-vision", Email: credential.ID})
		shortIDs = append(shortIDs, credential.ShortID)
	}
	slices.SortFunc(clients, func(left, right client) int { return strings.Compare(left.ID, right.ID) })
	slices.Sort(shortIDs)
	logLevel := state.Settings.LogLevel
	if logLevel == "" {
		logLevel = "warning"
	}

	configuration := config{
		Log: logConfig{LogLevel: logLevel},
		Inbounds: []inbound{{
			Listen: state.Settings.Listen, Port: state.Settings.Port, Protocol: "vless",
			Settings: inboundSettings{Clients: clients, Decryption: "none"},
			StreamSettings: streamSettings{
				Network: "raw", Security: "reality",
				Reality: realitySettings{
					Show: false, Target: state.Settings.Target, Xver: 0,
					ServerNames: state.Settings.ServerNames, PrivateKey: state.Settings.PrivateKey,
					ShortIDs: shortIDs,
				},
			},
			Sniffing: sniffing{Enabled: true, DestOverride: []string{"http", "tls", "quic"}, RouteOnly: true},
		}},
		Outbounds: []outbound{{Protocol: "freedom", Tag: "direct"}, {Protocol: "blackhole", Tag: "block"}},
	}
	result, err := json.MarshalIndent(configuration, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode Xray configuration: %w", err)
	}
	return append(result, '\n'), nil
}

func ValidateWithBinary(ctx context.Context, binaryPath string, configurationPath string) error {
	command := exec.CommandContext(ctx, binaryPath, "run", "-test", "-config", configurationPath)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Xray rejected configuration: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func ShareURI(address string, port uint16, publicKey string, serverName string, credential Credential) (string, error) {
	if net.ParseIP(address) == nil && strings.TrimSpace(address) == "" {
		return "", errors.New("server address is required")
	}
	if port == 0 || publicKey == "" || serverName == "" {
		return "", errors.New("port, public key, and server name are required")
	}
	if err := validateCredential(credential); err != nil {
		return "", err
	}
	query := url.Values{
		"encryption": {"none"}, "flow": {"xtls-rprx-vision"}, "fp": {"chrome"},
		"pbk": {publicKey}, "security": {"reality"}, "sid": {credential.ShortID},
		"sni": {serverName}, "type": {"tcp"},
	}
	return "vless://" + credential.UUID + "@" + net.JoinHostPort(address, strconv.Itoa(int(port))) +
		"?" + query.Encode() + "#" + url.PathEscape(credential.Label), nil
}

func validateState(state State) error {
	settings := state.Settings
	if settings.Listen == "" || settings.Port == 0 {
		return errors.New("Xray listen address and port are required")
	}
	if _, _, err := net.SplitHostPort(settings.Target); err != nil {
		return fmt.Errorf("invalid Reality target: %w", err)
	}
	if len(settings.ServerNames) == 0 || settings.PrivateKey == "" {
		return errors.New("Reality server names and private key are required")
	}
	seenUUID := make(map[string]struct{}, len(state.Credentials))
	seenShortID := make(map[string]struct{}, len(state.Credentials))
	for _, credential := range state.Credentials {
		if err := validateCredential(credential); err != nil {
			return err
		}
		if _, exists := seenUUID[credential.UUID]; exists {
			return fmt.Errorf("duplicate credential UUID %q", credential.UUID)
		}
		if _, exists := seenShortID[credential.ShortID]; exists {
			return fmt.Errorf("duplicate Reality short ID %q", credential.ShortID)
		}
		seenUUID[credential.UUID] = struct{}{}
		seenShortID[credential.ShortID] = struct{}{}
	}
	return nil
}

func validateCredential(credential Credential) error {
	if credential.ID == "" || credential.UUID == "" {
		return errors.New("credential ID and UUID are required")
	}
	decoded, err := hex.DecodeString(credential.ShortID)
	if err != nil || len(decoded) != 8 {
		return fmt.Errorf("invalid Reality short ID %q", credential.ShortID)
	}
	return nil
}

type config struct {
	Log       logConfig  `json:"log"`
	Inbounds  []inbound  `json:"inbounds"`
	Outbounds []outbound `json:"outbounds"`
}

type logConfig struct {
	LogLevel string `json:"loglevel"`
}

type inbound struct {
	Listen         string          `json:"listen"`
	Port           uint16          `json:"port"`
	Protocol       string          `json:"protocol"`
	Settings       inboundSettings `json:"settings"`
	StreamSettings streamSettings  `json:"streamSettings"`
	Sniffing       sniffing        `json:"sniffing"`
}

type inboundSettings struct {
	Clients    []client `json:"clients"`
	Decryption string   `json:"decryption"`
}

type client struct {
	ID    string `json:"id"`
	Flow  string `json:"flow"`
	Email string `json:"email"`
}

type streamSettings struct {
	Network  string          `json:"network"`
	Security string          `json:"security"`
	Reality  realitySettings `json:"realitySettings"`
}

type realitySettings struct {
	Show        bool     `json:"show"`
	Target      string   `json:"target"`
	Xver        int      `json:"xver"`
	ServerNames []string `json:"serverNames"`
	PrivateKey  string   `json:"privateKey"`
	ShortIDs    []string `json:"shortIds"`
}

type sniffing struct {
	Enabled      bool     `json:"enabled"`
	DestOverride []string `json:"destOverride"`
	RouteOnly    bool     `json:"routeOnly"`
}

type outbound struct {
	Protocol string `json:"protocol"`
	Tag      string `json:"tag"`
}
