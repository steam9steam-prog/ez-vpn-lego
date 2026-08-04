package vpnbootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	postgresadapter "github.com/steam9steam-prog/ez-vpn-lego/adapters/postgres"
	"github.com/steam9steam-prog/ez-vpn-lego/core/ports"
	"github.com/steam9steam-prog/ez-vpn-lego/drivers/xray/reality"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/candidate"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/id"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/secretbox"
)

type Repository interface {
	Stage(context.Context, postgresadapter.StageVPNParams) error
	Finalize(context.Context, postgresadapter.FinalizeVPNParams) error
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

func (service *Service) Bootstrap(ctx context.Context, request ports.BootstrapVPNRequest) (ports.BootstrapVPNResult, error) {
	if err := validateRequest(request); err != nil {
		return ports.BootstrapVPNResult{}, fmt.Errorf("%w: %v", ports.ErrInvalidArgument, err)
	}
	identifiers, err := generateIDs()
	if err != nil {
		return ports.BootstrapVPNResult{}, err
	}
	keys, err := reality.GenerateKeyPair()
	if err != nil {
		return ports.BootstrapVPNResult{}, err
	}
	shortID, err := reality.GenerateShortID()
	if err != nil {
		return ports.BootstrapVPNResult{}, err
	}
	vlessUUID, err := id.NewUUID()
	if err != nil {
		return ports.BootstrapVPNResult{}, err
	}
	credential := reality.Credential{ID: identifiers.credential, UUID: vlessUUID, ShortID: shortID, Label: "Owner device"}
	configuration, err := reality.Render(reality.State{
		Settings: reality.Settings{
			Listen: "0.0.0.0", Port: request.Port, Target: request.Target,
			ServerNames: []string{request.ServerName}, PrivateKey: keys.Private, LogLevel: "warning",
		},
		Credentials: []reality.Credential{credential},
	})
	if err != nil {
		return ports.BootstrapVPNResult{}, err
	}
	uri, err := reality.ShareURI(request.PublicAddress, request.Port, keys.Public, request.ServerName, credential)
	if err != nil {
		return ports.BootstrapVPNResult{}, err
	}
	instancePlaintext, _ := json.Marshal(instanceSecret{
		PrivateKey: keys.Private, PublicKey: keys.Public, Target: request.Target,
		ServerNames: []string{request.ServerName}, Port: request.Port,
	})
	credentialPlaintext, _ := json.Marshal(credentialSecret{UUID: vlessUUID, ShortID: shortID})
	instanceEncrypted, err := service.secrets.Seal(instancePlaintext, "protocol_instances:"+identifiers.instance)
	if err != nil {
		return ports.BootstrapVPNResult{}, err
	}
	credentialEncrypted, err := service.secrets.Seal(credentialPlaintext, "credentials:"+identifiers.credential)
	if err != nil {
		return ports.BootstrapVPNResult{}, err
	}
	revisionEncrypted, err := service.secrets.Seal(configuration, "config_revisions:"+identifiers.revision)
	if err != nil {
		return ports.BootstrapVPNResult{}, err
	}
	digest := sha256.Sum256(configuration)
	checksum := hex.EncodeToString(digest[:])
	operationInput, _ := json.Marshal(request)
	result := ports.BootstrapVPNResult{
		OperationID: identifiers.operation, UserID: identifiers.user,
		DeviceID: identifiers.device, URI: uri,
	}
	resultJSON, _ := json.Marshal(result)

	if err := service.repository.Stage(ctx, postgresadapter.StageVPNParams{
		AdminID: request.AdminID, IdempotencyKey: request.IdempotencyKey,
		OperationID: identifiers.operation, UserID: identifiers.user, DeviceID: identifiers.device,
		NodeID: identifiers.node, InstanceID: identifiers.instance, CredentialID: identifiers.credential,
		RevisionID: identifiers.revision, PublicAddress: request.PublicAddress, Driver: reality.Name,
		InstanceSettings: instanceEncrypted, CredentialSecret: credentialEncrypted,
		RevisionContent: revisionEncrypted, RevisionHash: checksum, OperationInput: operationInput,
	}); err != nil {
		return ports.BootstrapVPNResult{}, err
	}
	if err := candidate.Write(service.candidateDirectory, identifiers.revision, configuration); err != nil {
		_ = service.repository.Fail(ctx, identifiers.operation, identifiers.revision, "candidate_write_failed")
		return ports.BootstrapVPNResult{}, err
	}
	if err := service.helper.ApplyXray(ctx, identifiers.revision, checksum); err != nil {
		_ = service.repository.Fail(ctx, identifiers.operation, identifiers.revision, "xray_apply_failed")
		return ports.BootstrapVPNResult{}, err
	}
	finalize := postgresadapter.FinalizeVPNParams{
		AdminID: request.AdminID, OperationID: identifiers.operation, UserID: identifiers.user,
		DeviceID: identifiers.device, NodeID: identifiers.node, CredentialID: identifiers.credential,
		RevisionID: identifiers.revision, Result: resultJSON,
	}
	var finalizeErr error
	for attempt := 0; attempt < 3; attempt++ {
		if finalizeErr = service.repository.Finalize(ctx, finalize); finalizeErr == nil {
			return result, nil
		}
		time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
	}
	return ports.BootstrapVPNResult{}, fmt.Errorf("Xray applied but state finalization failed and requires reconciliation: %w", finalizeErr)
}

func validateRequest(request ports.BootstrapVPNRequest) error {
	if request.AdminID == "" || request.IdempotencyKey == "" {
		return errors.New("administrator and idempotency key are required")
	}
	if strings.TrimSpace(request.PublicAddress) == "" || request.Port == 0 {
		return errors.New("public address and port are required")
	}
	if _, _, err := net.SplitHostPort(request.Target); err != nil {
		return errors.New("target must use host:port format")
	}
	if strings.TrimSpace(request.ServerName) == "" {
		return errors.New("Reality server name is required")
	}
	if len(request.IdempotencyKey) < 16 || len(request.IdempotencyKey) > 128 {
		return errors.New("idempotency key must contain between 16 and 128 characters")
	}
	return nil
}

type ids struct{ operation, user, device, node, instance, credential, revision string }

func generateIDs() (ids, error) {
	values := make([]string, 7)
	for index := range values {
		value, err := id.NewUUID()
		if err != nil {
			return ids{}, err
		}
		values[index] = value
	}
	return ids{values[0], values[1], values[2], values[3], values[4], values[5], values[6]}, nil
}
