package command

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var output bytes.Buffer
	if code := Run("lego-vpnd", []string{"version"}, &output); code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	if !strings.Contains(output.String(), "lego-vpnd dev") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}
