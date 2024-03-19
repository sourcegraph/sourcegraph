package ci

import (
	"strings"
	"testing"
)

func TestAspectBazelRC(t *testing.T) {
	cmd, path := aspectBazelRC()

	if path != AspectGeneratedBazelRCPath {
		t.Fatalf("expected path to be %s, got %s", AspectGeneratedBazelRCPath, path)
	}

	// cmd should end with a semicolon so that it can be executed on its own and not interfere with other commands
	if !strings.HasSuffix(cmd, ";") {
		t.Fatalf("expected command to end with ';', got %s", cmd)
	}
}
