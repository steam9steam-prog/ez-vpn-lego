package helper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakeValidator struct{ err error }

func (validator fakeValidator) Validate(context.Context, string) error { return validator.err }

type fakeService struct {
	errors []error
	calls  int
}

func (service *fakeService) Restart(context.Context) error {
	index := service.calls
	service.calls++
	if index < len(service.errors) {
		return service.errors[index]
	}
	return nil
}

func TestApplyAndRollback(t *testing.T) {
	directory := t.TempDir()
	candidates := filepath.Join(directory, "candidates")
	if err := os.Mkdir(candidates, 0o750); err != nil {
		t.Fatal(err)
	}
	revisionID := "00000000-0000-4000-8000-000000000001"
	candidate := []byte(`{"new":true}`)
	candidatePath := filepath.Join(candidates, revisionID+".json")
	if err := os.WriteFile(candidatePath, candidate, 0o600); err != nil {
		t.Fatal(err)
	}
	configurationPath := filepath.Join(directory, "xray", "config.json")
	if err := os.Mkdir(filepath.Dir(configurationPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configurationPath, []byte(`{"old":true}`), 0o640); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(candidate)
	request := ApplyRequest{RevisionID: revisionID, SHA256: hex.EncodeToString(digest[:])}

	service := &fakeService{}
	engine := NewEngine(candidates, configurationPath, fakeValidator{}, service)
	if err := engine.Apply(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	assertFile(t, configurationPath, candidate)

	if err := os.WriteFile(configurationPath, []byte(`{"old":true}`), 0o640); err != nil {
		t.Fatal(err)
	}
	service = &fakeService{errors: []error{errors.New("failed"), nil}}
	engine = NewEngine(candidates, configurationPath, fakeValidator{}, service)
	if err := engine.Apply(context.Background(), request); err == nil {
		t.Fatal("expected failed apply")
	}
	assertFile(t, configurationPath, []byte(`{"old":true}`))
	if service.calls != 2 {
		t.Fatalf("restart calls: got %d, want 2", service.calls)
	}
}

func assertFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("file content: got %q, want %q", got, want)
	}
}
