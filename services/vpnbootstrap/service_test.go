package vpnbootstrap

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	postgresadapter "github.com/steam9steam-prog/ez-vpn-lego/adapters/postgres"
	"github.com/steam9steam-prog/ez-vpn-lego/core/ports"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/secretbox"
)

type fakeRepository struct {
	staged    postgresadapter.StageVPNParams
	finalized postgresadapter.FinalizeVPNParams
	failed    bool
}

func (repository *fakeRepository) Stage(_ context.Context, input postgresadapter.StageVPNParams) error {
	repository.staged = input
	return nil
}
func (repository *fakeRepository) Finalize(_ context.Context, input postgresadapter.FinalizeVPNParams) error {
	repository.finalized = input
	return nil
}
func (repository *fakeRepository) Fail(context.Context, string, string, string) error {
	repository.failed = true
	return nil
}

type fakeHelper struct{ err error }

func (helper fakeHelper) ApplyXray(context.Context, string, string) error { return helper.err }

func TestBootstrapStagesEncryptedStateAndAppliesCandidate(t *testing.T) {
	box, err := secretbox.New(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	repository := &fakeRepository{}
	directory := t.TempDir()
	service := New(repository, fakeHelper{}, box, directory)
	result, err := service.Bootstrap(context.Background(), ports.BootstrapVPNRequest{
		AdminID: "00000000-0000-4000-8000-000000000001", PublicAddress: "203.0.113.1",
		Port: 443, Target: "example.com:443", ServerName: "example.com",
		IdempotencyKey: "bootstrap-vpn-test-0001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.URI == "" || repository.finalized.OperationID == "" {
		t.Fatalf("unexpected bootstrap result: %+v", result)
	}
	if bytes.Contains(repository.staged.InstanceSettings, []byte("private_key")) || bytes.Contains(repository.staged.RevisionContent, []byte("privateKey")) {
		t.Fatal("sensitive state was stored without encryption")
	}
	path := filepath.Join(directory, repository.staged.RevisionID+".json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("candidate permissions: %o", info.Mode().Perm())
	}
}

func TestBootstrapRecordsApplyFailure(t *testing.T) {
	box, _ := secretbox.New(bytes.Repeat([]byte{1}, 32))
	repository := &fakeRepository{}
	service := New(repository, fakeHelper{err: errors.New("apply failed")}, box, t.TempDir())
	_, err := service.Bootstrap(context.Background(), ports.BootstrapVPNRequest{
		AdminID: "00000000-0000-4000-8000-000000000001", PublicAddress: "203.0.113.1",
		Port: 443, Target: "example.com:443", ServerName: "example.com",
		IdempotencyKey: "bootstrap-vpn-test-0002",
	})
	if err == nil || !repository.failed {
		t.Fatalf("expected recorded apply failure, got err=%v failed=%v", err, repository.failed)
	}
}
