package candidate

import (
	"fmt"
	"os"
	"path/filepath"
)

func Write(directory, revisionID string, configuration []byte) error {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create candidate directory: %w", err)
	}
	path := filepath.Join(directory, revisionID+".json")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create Xray candidate: %w", err)
	}
	if _, err := file.Write(configuration); err != nil {
		_ = file.Close()
		return fmt.Errorf("write Xray candidate: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync Xray candidate: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close Xray candidate: %w", err)
	}
	return nil
}
