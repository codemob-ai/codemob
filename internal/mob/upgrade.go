package mob

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// globalVersionFile returns the path to ~/.config/codemob/version.
func globalVersionFile() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "codemob", "version")
}

// CheckUpgrade compares the running binary version against the last-seen
// version stored in ~/.config/codemob/version. If they differ, it re-runs
// the idempotent setup steps (gitignore, permissions, slash commands) and
// updates the stored version.
//
// repoRoot may be empty if not inside a git repo.
func CheckUpgrade(version, repoRoot string) {
	if version == "dev" {
		return
	}

	vFile := globalVersionFile()
	data, err := os.ReadFile(vFile)
	if err == nil && strings.TrimSpace(string(data)) == version {
		return
	}

	fmt.Println()
	fmt.Printf("%sUpdated to %s - refreshing setup...%s\n", accent, version, reset)
	fmt.Println()

	// Global setup
	setupGlobalGitignore()

	claude := agentInstalled("claude")
	codex := agentInstalled("codex")

	if claude {
		setupClaudePermissions()
	}
	if codex {
		setupCodexPrompts(claude && codex)
	}

	// Per-repo setup
	if repoRoot != "" && IsInitialized(repoRoot) {
		if claude {
			setupClaudeCommands(repoRoot, claude && codex)
		}
	}

	// Persist new version
	os.MkdirAll(filepath.Dir(vFile), 0755)
	os.WriteFile(vFile, []byte(version), 0644)

	fmt.Println()
}

// WriteVersion persists the given version to the global version file.
// Called by init/reinit so the upgrade check doesn't fire immediately after.
func WriteVersion(version string) {
	if version == "dev" {
		return
	}
	vFile := globalVersionFile()
	os.MkdirAll(filepath.Dir(vFile), 0755)
	os.WriteFile(vFile, []byte(version), 0644)
}

func agentInstalled(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
