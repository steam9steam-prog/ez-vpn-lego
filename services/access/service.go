package access

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	postgresadapter "github.com/steam9steam-prog/ez-vpn-lego/adapters/postgres"
	"github.com/steam9steam-prog/ez-vpn-lego/core/ports"
	"github.com/steam9steam-prog/ez-vpn-lego/drivers/xray/reality"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/candidate"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/id"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/secretbox"
)

type Repository interface {
	Load(context.Context) (postgresadapter.AccessState, error)
	Stage(context.Context, postgresadapter.StageAccessParams) error
	Finalize(context.Context, postgresadapter.FinalizeAccessParams) error
	Fail(context.Context, string, string, string) error
}

type Helper interface {
	ApplyXray(context.Context, string, string) error
}

type Service struct {
	repository         Repository
	helper             Helper
	secrets            *secretbox.Box
	candidateDirectory string
	mu                 sync.Mutex
}

type instanceSecret struct {
	PrivateKey  string   `json:"private_key"`
	PublicKey   string   `json:"public_key"`
	Target      string   `json:"target"`
	ServerNames []string `json:"server_names"`
	Port        uint16   `json:"port"`
}

type credentialSecret struct {
	UUID    string `json:"uuid"`
	ShortID string `json:"short_id"`
}

func New(repository Repository, helper Helper, secrets *secretbox.Box, candidateDirectory string) *Service {
	return &Service{repository: repository, helper: helper, secrets: secrets, candidateDirectory: candidateDirectory}
}

func (service *Service) Create(ctx context.Context, request ports.CreateAccessRequest) (ports.CreateAccessResult, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	request.Name = strings.TrimSpace(request.Name)
	if request.AdminID == "" || len([]rune(request.Name)) < 1 || len([]rune(request.Name)) > 80 || len(request.IdempotencyKey) < 16 {
		return ports.CreateAccessResult{}, fmt.Errorf("%w: administrator, valid name and idempotency key are required", ports.ErrInvalidArgument)
	}
	state, err := service.repository.Load(ctx)
	if err != nil {
		return ports.CreateAccessResult{}, err
	}
	var instance instanceSecret
	plaintext, err := service.secrets.Open(state.InstanceSettings, "protocol_instances:"+state.InstanceID)
	if err != nil || json.Unmarshal(plaintext, &instance) != nil {
		return ports.CreateAccessResult{}, errorsWrap("decode Reality instance", err)
	}
	credentials := make([]reality.Credential, 0, len(state.Credentials)+1)
	for _, encrypted := range state.Credentials {
		plaintext, err := service.secrets.Open(encrypted.Secret, "credentials:"+encrypted.ID)
		if err != nil {
			return ports.CreateAccessResult{}, err
		}
		var secret credentialSecret
		if err := json.Unmarshal(plaintext, &secret); err != nil {
			return ports.CreateAccessResult{}, err
		}
		credentials = append(credentials, reality.Credential{ID: encrypted.ID, UUID: secret.UUID, ShortID: secret.ShortID, Label: encrypted.Name})
	}
	identifiers, err := generateIDs()
	if err != nil {
		return ports.CreateAccessResult{}, err
	}
	vlessUUID, err := id.NewUUID()
	if err != nil {
		return ports.CreateAccessResult{}, err
	}
	shortID, err := reality.GenerateShortID()
	if err != nil {
		return ports.CreateAccessResult{}, err
	}
	newCredential := reality.Credential{ID: identifiers.credential, UUID: vlessUUID, ShortID: shortID, Label: request.Name}
	credentials = append(credentials, newCredential)
	configuration, err := reality.Render(reality.State{
		Settings:    reality.Settings{Listen: "0.0.0.0", Port: instance.Port, Target: instance.Target, ServerNames: instance.ServerNames, PrivateKey: instance.PrivateKey, LogLevel: "warning"},
		Credentials: credentials,
	})
	if err != nil {
		return ports.CreateAccessResult{}, err
	}
	uri, err := reality.ShareURI(state.PublicAddress, instance.Port, instance.PublicKey, instance.ServerNames[0], newCredential)
	if err != nil {
		return ports.CreateAccessResult{}, err
	}
	secretJSON, _ := json.Marshal(credentialSecret{UUID: vlessUUID, ShortID: shortID})
	encryptedSecret, err := service.secrets.Seal(secretJSON, "credentials:"+identifiers.credential)
	if err != nil {
		return ports.CreateAccessResult{}, err
	}
	encryptedRevision, err := service.secrets.Seal(configuration, "config_revisions:"+identifiers.revision)
	if err != nil {
		return ports.CreateAccessResult{}, err
	}
	digest := sha256.Sum256(configuration)
	checksum := hex.EncodeToString(digest[:])
	result := ports.CreateAccessResult{OperationID: identifiers.operation, UserID: identifiers.user, DeviceID: identifiers.device, URI: uri}
	resultJSON, _ := json.Marshal(result)
	inputJSON, _ := json.Marshal(request)
	if err := service.repository.Stage(ctx, postgresadapter.StageAccessParams{
		AdminID: request.AdminID, IdempotencyKey: request.IdempotencyKey, Name: request.Name,
		OperationID: identifiers.operation, UserID: identifiers.user, DeviceID: identifiers.device,
		CredentialID: identifiers.credential, NodeID: state.NodeID, RevisionID: identifiers.revision,
		Driver: reality.Name, RevisionHash: checksum, CredentialSecret: encryptedSecret,
		RevisionContent: encryptedRevision, OperationInput: inputJSON,
	}); err != nil {
		return ports.CreateAccessResult{}, err
	}
	if err := candidate.Write(service.candidateDirectory, identifiers.revision, configuration); err != nil {
		_ = service.repository.Fail(ctx, identifiers.operation, identifiers.revision, "candidate_write_failed")
		return ports.CreateAccessResult{}, err
	}
	if err := service.helper.ApplyXray(ctx, identifiers.revision, checksum); err != nil {
		_ = service.repository.Fail(ctx, identifiers.operation, identifiers.revision, "xray_apply_failed")
		return ports.CreateAccessResult{}, err
	}
	if err := service.repository.Finalize(ctx, postgresadapter.FinalizeAccessParams{
		AdminID: request.AdminID, OperationID: identifiers.operation, UserID: identifiers.user,
		DeviceID: identifiers.device, CredentialID: identifiers.credential, NodeID: state.NodeID,
		RevisionID: identifiers.revision, Result: resultJSON,
	}); err != nil {
		return ports.CreateAccessResult{}, fmt.Errorf("Xray applied but access finalization failed: %w", err)
	}
	return result, nil
}

type ids struct{ operation, user, device, credential, revision string }

func generateIDs() (ids, error) {
	values := make([]string, 5)
	for index := range values {
		value, err := id.NewUUID()
		if err != nil {
			return ids{}, err
		}
		values[index] = value
	}
	return ids{values[0], values[1], values[2], values[3], values[4]}, nil
}

func errorsWrap(label string, err error) error {
	if err == nil {
		return fmt.Errorf("%s: invalid JSON", label)
	}
	return fmt.Errorf("%s: %w", label, err)
}
