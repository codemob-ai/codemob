package mob_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildCore builds the codemob binary and returns its path.
func buildCore(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "codemob")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build codemob: %s\n%s", err, out)
	}
	return bin
}

// repoRoot returns the root of the codemob source repo.
func repoRoot(t *testing.T) string {
	t.Helper()
	// We're in internal/mob/, go up two levels
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "..", "..")
}

// setupTestRepo creates a temp HOME and a git repo inside it, returns (home, repoPath).
func setupTestRepo(t *testing.T) (string, string) {
	t.Helper()

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	repoPath := filepath.Join(tmpHome, "test-repo")
	os.MkdirAll(repoPath, 0755)

	run(t, repoPath, "git", "init")
	run(t, repoPath, "git", "commit", "--allow-empty", "-m", "init")

	return tmpHome, repoPath
}

// initRepo runs codemob init in the given repo, providing "main" as base branch input.
func initRepo(t *testing.T, bin, repoPath string) {
	t.Helper()
	cmd := exec.Command(bin, "init")
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader("main\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init failed: %s\n%s", err, out)
	}
}

// run executes a command in the given directory.
func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %s\n%s", name, args, err, out)
	}
	return string(out)
}

// runCore executes codemob with args in the given directory.
func runCore(t *testing.T, bin, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("codemob %v failed: %s\n%s", args, err, out)
	}
	return string(out)
}

// runCoreExpectError executes codemob expecting failure.
func runCoreExpectError(t *testing.T, bin, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	if cmd.ProcessState.ExitCode() == 0 {
		t.Fatalf("expected codemob %v to fail, but it succeeded: %s", args, out)
	}
	return string(out)
}

// readConfig reads and parses .codemob/config.json from the repo.
func readConfig(t *testing.T, repoPath string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoPath, ".codemob", "config.json"))
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}
	return cfg
}

// parseResult parses CODEMOB_KEY=value lines into a map.
func parseResult(output string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "CODEMOB_") && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			result[parts[0]] = parts[1]
		}
	}
	return result
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestInit(t *testing.T) {
	bin := buildCore(t)
	home, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// given -> config.json should exist
	// when -> we read the config
	cfg := readConfig(t, repoPath)

	// then
	if cfg["default_agent"] != "claude" {
		t.Errorf("expected default_agent=claude, got %v", cfg["default_agent"])
	}
	if cfg["base_branch"] != "main" {
		t.Errorf("expected base_branch=main, got %v", cfg["base_branch"])
	}

	// then -> .codemob/mobs/ dir should exist
	if _, err := os.Stat(filepath.Join(repoPath, ".codemob", "mobs")); err != nil {
		t.Errorf(".codemob/mobs/ not created: %v", err)
	}

	// then -> global gitignore should contain .codemob/
	gitignorePath := filepath.Join(home, ".config", "git", "ignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("global gitignore not created: %v", err)
	}
	if !strings.Contains(string(data), ".codemob/") {
		t.Error("global gitignore does not contain .codemob/")
	}

	// then -> slash commands should be installed in the project's .claude/commands/
	commandsDir := filepath.Join(repoPath, ".claude", "commands")
	for _, name := range []string{
		"mob-list.md", "mob-new.md", "mob-switch.md", "mob-remove.md",
		"codemob-list.md", "codemob-new.md", "codemob-switch.md", "codemob-remove.md",
	} {
		if _, err := os.Stat(filepath.Join(commandsDir, name)); err != nil {
			t.Errorf("slash command %s not installed: %v", name, err)
		}
	}

	// then -> global gitignore should contain codemob command patterns
	if !strings.Contains(string(data), "mob-*.md") {
		t.Error("global gitignore does not contain mob-*.md pattern")
	}
}

func TestInitIdempotent(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)

	// given -> init once
	initRepo(t, bin, repoPath)

	// when -> init again
	initRepo(t, bin, repoPath)

	// then -> should not fail, config should still be valid
	cfg := readConfig(t, repoPath)
	if cfg["base_branch"] != "main" {
		t.Errorf("expected base_branch=main after reinit, got %v", cfg["base_branch"])
	}
}

func TestNewMob(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when
	out := runCore(t, bin, repoPath, "--new", "test-feature", "--no-launch")

	// then -> output should confirm creation
	if !strings.Contains(out, "test-feature") {
		t.Errorf("expected output to mention 'test-feature', got: %s", out)
	}

	// then -> worktree should exist on disk
	worktreePath := filepath.Join(repoPath, ".codemob", "mobs", "test-feature")
	if _, err := os.Stat(worktreePath); err != nil {
		t.Errorf("worktree not created: %v", err)
	}

	// then -> config should have the mob
	cfg := readConfig(t, repoPath)
	mobs := cfg["mobs"].([]interface{})
	if len(mobs) != 1 {
		t.Fatalf("expected 1 mob, got %d", len(mobs))
	}
	mob := mobs[0].(map[string]interface{})
	if mob["name"] != "test-feature" {
		t.Errorf("expected mob name=test-feature, got %v", mob["name"])
	}
	if mob["branch"] != "mob/test-feature" {
		t.Errorf("expected branch=mob/test-feature, got %v", mob["branch"])
	}
	if mob["agent"] != "claude" {
		t.Errorf("expected agent=claude, got %v", mob["agent"])
	}
}

func TestNewMobAutoName(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when -> no name provided
	runCore(t, bin, repoPath, "--new", "--no-launch")

	// then -> config should have exactly one mob with an adjective-fruit name
	cfg := readConfig(t, repoPath)
	mobs := cfg["mobs"].([]interface{})
	if len(mobs) != 1 {
		t.Fatalf("expected 1 mob, got %d", len(mobs))
	}
	name := mobs[0].(map[string]interface{})["name"].(string)
	if !strings.Contains(name, "-") {
		t.Errorf("expected adjective-fruit name, got %q", name)
	}
}

func TestNewMobDuplicateName(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// given -> create a mob
	runCore(t, bin, repoPath, "--new", "dupe-test", "--no-launch")

	// when -> try to create another with the same name
	out := runCoreExpectError(t, bin, repoPath, "--new", "dupe-test", "--no-launch")

	// then
	if !strings.Contains(out, "already exists") {
		t.Errorf("expected 'already exists' error, got: %s", out)
	}
}

func TestListMobs(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// given -> no mobs
	out := runCore(t, bin, repoPath, "--list")

	// then
	if !strings.Contains(out, "No mobs") {
		t.Errorf("expected 'No mobs' message, got: %s", out)
	}

	// given -> create two mobs
	runCore(t, bin, repoPath, "--new", "alpha", "--no-launch")
	runCore(t, bin, repoPath, "--new", "beta", "--no-launch")

	// when
	out = runCore(t, bin, repoPath, "--list")

	// then
	if !strings.Contains(out, "alpha") {
		t.Errorf("expected 'alpha' in list output, got: %s", out)
	}
	if !strings.Contains(out, "beta") {
		t.Errorf("expected 'beta' in list output, got: %s", out)
	}
	if !strings.Contains(out, "mob/alpha") {
		t.Errorf("expected 'mob/alpha' branch in list output, got: %s", out)
	}
}

func TestResumeMob(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "--new", "resume-test", "--no-launch")

	// when
	out := runCore(t, bin, repoPath, "--resume", "resume-test", "--no-launch")

	// then -> should mention the mob name
	if !strings.Contains(out, "resume-test") {
		t.Errorf("expected output to mention 'resume-test', got: %s", out)
	}
}

func TestResumeNotFound(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when
	out := runCoreExpectError(t, bin, repoPath, "--resume", "nonexistent", "--no-launch")

	// then
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' error, got: %s", out)
	}
}

func TestRemoveMob(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "--new", "remove-me", "--no-launch")

	// when
	out := runCore(t, bin, repoPath, "remove", "remove-me")

	// then
	if !strings.Contains(out, "Removed") {
		t.Errorf("expected 'Removed' message, got: %s", out)
	}

	// then -> worktree should be gone
	worktreePath := filepath.Join(repoPath, ".codemob", "mobs", "remove-me")
	if _, err := os.Stat(worktreePath); err == nil {
		t.Error("worktree still exists after remove")
	}

	// then -> config should have no mobs
	cfg := readConfig(t, repoPath)
	mobs, _ := cfg["mobs"].([]interface{})
	if len(mobs) != 0 {
		t.Errorf("expected 0 mobs after remove, got %d", len(mobs))
	}
}

func TestReconciliation(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "--new", "orphan", "--no-launch")

	// given -> manually remove the worktree outside of codemob
	run(t, repoPath, "git", "worktree", "remove", filepath.Join(".codemob", "mobs", "orphan"))

	// when -> list (triggers reconciliation)
	out := runCore(t, bin, repoPath, "--list")

	// then -> orphan should be cleaned from config
	if !strings.Contains(out, "No mobs") {
		t.Errorf("expected 'No mobs' after reconciliation, got: %s", out)
	}

	// then -> config should be empty
	cfg := readConfig(t, repoPath)
	mobs, _ := cfg["mobs"].([]interface{})
	if len(mobs) != 0 {
		t.Errorf("expected 0 mobs after reconciliation, got %d", len(mobs))
	}
}

func TestRepoRootFromInsideWorktree(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "--new", "nested-test", "--no-launch")

	// when -> run list from inside the mob worktree
	worktreePath := filepath.Join(repoPath, ".codemob", "mobs", "nested-test")
	out := runCore(t, bin, worktreePath, "--list")

	// then -> should work and show the mob
	if !strings.Contains(out, "nested-test") {
		t.Errorf("expected 'nested-test' when listing from inside worktree, got: %s", out)
	}
}

func TestUninitializedRepo(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	// do NOT run init

	// when
	out := runCoreExpectError(t, bin, repoPath, "--list")

	// then
	if !strings.Contains(out, "not initialized") {
		t.Errorf("expected 'not initialized' error, got: %s", out)
	}
}

func TestNewMobWithCustomAgent(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when
	runCore(t, bin, repoPath, "--new", "codex-mob", "--agent", "codex", "--no-launch")

	// then -> config should reflect the agent
	cfg := readConfig(t, repoPath)
	mobs := cfg["mobs"].([]interface{})
	mob := mobs[0].(map[string]interface{})
	if mob["agent"] != "codex" {
		t.Errorf("expected agent=codex in config, got %v", mob["agent"])
	}
}
