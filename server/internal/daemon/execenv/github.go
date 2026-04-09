package execenv

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteGitAskPass writes a GIT_ASKPASS helper script to the workdir that
// echoes the given token as the password for any git credential request.
// Returns the absolute path to the script.
func WriteGitAskPass(workDir, token string) (string, error) {
	script := fmt.Sprintf("#!/bin/sh\necho '%s'\n", token)
	path := filepath.Join(workDir, ".git-askpass.sh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		return "", fmt.Errorf("write git-askpass: %w", err)
	}
	return path, nil
}
