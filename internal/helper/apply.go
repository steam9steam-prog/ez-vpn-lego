package helper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

var revisionPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Validator interface {
	Validate(context.Context, string) error
}

type Service interface {
	Restart(context.Context) error
}

type ApplyRequest struct {
	RevisionID string `json:"revision_id"`
	SHA256     string `json:"sha256"`
}

type Engine struct {
	candidateDirectory string
	configurationPath  string
	validator          Validator
	service            Service
	mu                 sync.Mutex
}

func NewEngine(candidateDirectory, configurationPath string, validator Validator, service Service) *Engine {
	return &Engine{
		candidateDirectory: candidateDirectory,
		configurationPath:  configurationPath,
		validator:          validator,
		service:            service,
	}
}

func (engine *Engine) Apply(ctx context.Context, request ApplyRequest) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	if !revisionPattern.MatchString(request.RevisionID) {
		return errors.New("invalid revision ID")
	}
	if len(request.SHA256) != sha256.Size*2 {
		return errors.New("invalid revision checksum")
	}
	candidatePath := filepath.Join(engine.candidateDirectory, request.RevisionID+".json")
	candidate, err := readCandidate(candidatePath)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(candidate)
	if !equalHex(request.SHA256, digest[:]) {
		return errors.New("candidate checksum mismatch")
	}
	if err := engine.validator.Validate(ctx, candidatePath); err != nil {
		return err
	}

	previous, previousMode, existed, err := readCurrent(engine.configurationPath)
	if err != nil {
		return err
	}
	if err := atomicWrite(engine.configurationPath, candidate, 0o640); err != nil {
		return fmt.Errorf("install Xray configuration: %w", err)
	}
	if err := engine.service.Restart(ctx); err == nil {
		return nil
	} else if rollbackErr := rollback(engine.configurationPath, previous, previousMode, existed); rollbackErr != nil {
		return fmt.Errorf("restart Xray: %v; rollback configuration: %w", err, rollbackErr)
	} else if recoveryErr := engine.service.Restart(ctx); recoveryErr != nil {
		return fmt.Errorf("restart Xray: %v; recovery restart: %w", err, recoveryErr)
	} else {
		return fmt.Errorf("new Xray configuration failed; previous configuration restored: %w", err)
	}
}

func readCandidate(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect candidate: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o022 != 0 {
		return nil, errors.New("candidate must be a regular file and not group/world writable")
	}
	if info.Size() > 4<<20 {
		return nil, errors.New("candidate exceeds 4 MiB")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open candidate: %w", err)
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, 4<<20+1))
	if err != nil {
		return nil, fmt.Errorf("read candidate: %w", err)
	}
	return content, nil
}

func readCurrent(path string) ([]byte, os.FileMode, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, fmt.Errorf("inspect current configuration: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, 0, false, errors.New("current Xray configuration is not a regular file")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, false, fmt.Errorf("read current configuration: %w", err)
	}
	return content, info.Mode().Perm(), true, nil
}

func atomicWrite(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".ez-vpn-lego-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func rollback(path string, previous []byte, mode os.FileMode, existed bool) error {
	if existed {
		return atomicWrite(path, previous, mode)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func equalHex(encoded string, expected []byte) bool {
	decoded, err := hex.DecodeString(encoded)
	return err == nil && len(decoded) == len(expected) && string(decoded) == string(expected)
}
