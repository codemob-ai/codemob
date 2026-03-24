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
// It also places a fake "claude" stub on PATH so that codemob init succeeds
// in CI environments where the real agent binaries aren't installed.
func setupTestRepo(t *testing.T) (string, string) {
	t.Helper()

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create a fake claude binary that satisfies checkDependencies:
	//   --version → prints a version string
	//   auth status --json → prints {"loggedIn":true}
	stubDir := filepath.Join(tmpHome, "bin")
	os.MkdirAll(stubDir, 0755)
	stubPath := filepath.Join(stubDir, "claude")
	stubScript := `#!/bin/sh
case "$1" in
  --version) echo "claude-stub 0.0.0" ;;
  auth) echo '{"loggedIn":true}' ;;
  *) exit 0 ;;
esac
`
	os.WriteFile(stubPath, []byte(stubScript), 0755)
	t.Setenv("PATH", stubDir+":"+os.Getenv("PATH"))

	repoPath := filepath.Join(tmpHome, "test-repo")
	os.MkdirAll(repoPath, 0755)

	run(t, repoPath, "git", "init", "-b", "main")
	run(t, repoPath, "git", "config", "user.email", "test@codemob.ai")
	run(t, repoPath, "git", "config", "user.name", "codemob-test")
	run(t, repoPath, "git", "commit", "--allow-empty", "-m", "init")

	return tmpHome, repoPath
}

// initRepo runs codemob init in the given repo, providing defaults for base branch and agent.
func initRepo(t *testing.T, bin, repoPath string) {
	t.Helper()
	cmd := exec.Command(bin, "init")
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader("main\nclaude\n")
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
	out, err := cmd.CombinedOutput()
	if err == nil {
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
		"mob-list.md", "mob-new.md", "mob-switch.md", "mob-remove.md", "mob-drop.md",
		"codemob-list.md", "codemob-new.md", "codemob-switch.md", "codemob-remove.md", "codemob-drop.md",
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

func TestSlashCommandsReferenceValidCommands(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	commandsDir := filepath.Join(repoPath, ".claude", "commands")

	// given -> expected codemob commands that each slash command should contain
	expected := map[string][]string{
		"mob-list.md":   {"codemob list"},
		"mob-new.md":    {"codemob queue new"},
		"mob-switch.md": {"codemob list-others", "codemob queue switch"},
		"mob-remove.md": {"codemob list", "codemob remove", "codemob queue remove"},
		"mob-drop.md":   {"codemob queue remove"},
	}

	for file, commands := range expected {
		// when -> read slash command content
		content, err := os.ReadFile(filepath.Join(commandsDir, file))
		if err != nil {
			t.Fatalf("could not read %s: %v", file, err)
		}

		// then -> each expected command should appear in the content
		for _, cmd := range commands {
			if !strings.Contains(string(content), cmd) {
				t.Errorf("%s: expected to find %q in content", file, cmd)
			}
		}
	}

	// then -> codemob-* variants should have identical content to mob-* variants
	for mobFile := range expected {
		codemobFile := strings.Replace(mobFile, "mob-", "codemob-", 1)
		mobContent, _ := os.ReadFile(filepath.Join(commandsDir, mobFile))
		codemobContent, err := os.ReadFile(filepath.Join(commandsDir, codemobFile))
		if err != nil {
			t.Fatalf("could not read %s: %v", codemobFile, err)
		}
		if string(mobContent) != string(codemobContent) {
			t.Errorf("%s and %s have different content", mobFile, codemobFile)
		}
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
	out := runCore(t, bin, repoPath, "new", "test-feature", "--no-launch")

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
	runCore(t, bin, repoPath, "new", "--no-launch")

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
	runCore(t, bin, repoPath, "new", "dupe-test", "--no-launch")

	// when -> try to create another with the same name
	out := runCoreExpectError(t, bin, repoPath, "new", "dupe-test", "--no-launch")

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
	out := runCore(t, bin, repoPath, "list")

	// then
	if !strings.Contains(out, "No mobs") {
		t.Errorf("expected 'No mobs' message, got: %s", out)
	}

	// given -> create two mobs
	runCore(t, bin, repoPath, "new", "alpha", "--no-launch")
	runCore(t, bin, repoPath, "new", "beta", "--no-launch")

	// when
	out = runCore(t, bin, repoPath, "list")

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
	runCore(t, bin, repoPath, "new", "resume-test", "--no-launch")

	// when
	out := runCore(t, bin, repoPath, "resume", "resume-test", "--no-launch")

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
	out := runCoreExpectError(t, bin, repoPath, "resume", "nonexistent", "--no-launch")

	// then
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' error, got: %s", out)
	}
}

func TestRemoveMob(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "remove-me", "--no-launch")

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
	runCore(t, bin, repoPath, "new", "orphan", "--no-launch")

	// given -> manually remove the worktree outside of codemob
	run(t, repoPath, "git", "worktree", "remove", filepath.Join(".codemob", "mobs", "orphan"))

	// when -> list (triggers reconciliation)
	out := runCore(t, bin, repoPath, "list")

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
	runCore(t, bin, repoPath, "new", "nested-test", "--no-launch")

	// when -> run list from inside the mob worktree
	worktreePath := filepath.Join(repoPath, ".codemob", "mobs", "nested-test")
	out := runCore(t, bin, worktreePath, "list")

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
	out := runCoreExpectError(t, bin, repoPath, "list")

	// then
	if !strings.Contains(out, "not initialized") {
		t.Errorf("expected 'not initialized' error, got: %s", out)
	}
}

// ─── Name Validation ──────────────────────────────────────────────────────────

func TestNameValidation_Uppercase(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when -> uppercase name
	runCore(t, bin, repoPath, "new", "MyFeature", "--no-launch")

	// then -> should work
	cfg := readConfig(t, repoPath)
	mobs := cfg["mobs"].([]interface{})
	if len(mobs) != 1 {
		t.Fatalf("expected 1 mob, got %d", len(mobs))
	}
	if mobs[0].(map[string]interface{})["name"] != "MyFeature" {
		t.Errorf("expected name=MyFeature, got %v", mobs[0].(map[string]interface{})["name"])
	}
}

func TestNameValidation_AllNumericRejected(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when -> all-numeric name
	out := runCoreExpectError(t, bin, repoPath, "new", "123", "--no-launch")

	// then
	if !strings.Contains(out, "numeric") {
		t.Errorf("expected numeric rejection error, got: %s", out)
	}
}

func TestNameValidation_TooLong(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when -> 61 char name
	longName := strings.Repeat("a", 61)
	out := runCoreExpectError(t, bin, repoPath, "new", longName, "--no-launch")

	// then
	if !strings.Contains(out, "too long") {
		t.Errorf("expected too long error, got: %s", out)
	}
}

func TestNameValidation_LeadingHyphen(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when
	out := runCoreExpectError(t, bin, repoPath, "new", "-bad", "--no-launch")

	// then
	if !strings.Contains(out, "hyphen") {
		t.Errorf("expected hyphen error, got: %s", out)
	}
}

func TestNameValidation_SpecialChars(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when
	out := runCoreExpectError(t, bin, repoPath, "new", "foo/bar", "--no-launch")

	// then
	if !strings.Contains(out, "letters") {
		t.Errorf("expected invalid char error, got: %s", out)
	}
}

// ─── Index-Based Resolution ──────────────────────────────────────────────────

func TestResumeByIndex(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "alpha", "--no-launch")
	runCore(t, bin, repoPath, "new", "beta", "--no-launch")

	// when -> resume by index
	out := runCore(t, bin, repoPath, "resume", "2", "--no-launch")

	// then -> should mention beta
	if !strings.Contains(out, "beta") {
		t.Errorf("expected 'beta' in resume output, got: %s", out)
	}
}

func TestRemoveByIndex(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "first", "--no-launch")
	runCore(t, bin, repoPath, "new", "second", "--no-launch")

	// when -> remove by index
	runCore(t, bin, repoPath, "remove", "1")

	// then -> only second remains
	cfg := readConfig(t, repoPath)
	mobs := cfg["mobs"].([]interface{})
	if len(mobs) != 1 {
		t.Fatalf("expected 1 mob after remove, got %d", len(mobs))
	}
	if mobs[0].(map[string]interface{})["name"] != "second" {
		t.Errorf("expected 'second' to remain, got %v", mobs[0].(map[string]interface{})["name"])
	}
}

// ─── List Others ─────────────────────────────────────────────────────────────

func TestListOthersExcludesCurrent(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "mob-a", "--no-launch")
	runCore(t, bin, repoPath, "new", "mob-b", "--no-launch")

	// when -> list-others from inside mob-a
	worktreeA := filepath.Join(repoPath, ".codemob", "mobs", "mob-a")
	out := runCore(t, bin, worktreeA, "list-others")

	// then -> should show mob-b but not mob-a
	if strings.Contains(out, "mob-a") {
		t.Errorf("--list-others should exclude current mob, got: %s", out)
	}
	if !strings.Contains(out, "mob-b") {
		t.Errorf("--list-others should show mob-b, got: %s", out)
	}
}

// ─── Purge ───────────────────────────────────────────────────────────────────

func TestPurge(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "one", "--no-launch")
	runCore(t, bin, repoPath, "new", "two", "--no-launch")

	// when -> purge with "y" confirmation
	cmd := exec.Command(bin, "purge")
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader("y\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("clear failed: %s\n%s", err, out)
	}

	// then -> no mobs
	cfg := readConfig(t, repoPath)
	mobs, _ := cfg["mobs"].([]interface{})
	if len(mobs) != 0 {
		t.Errorf("expected 0 mobs after purge, got %d", len(mobs))
	}

	// then -> worktrees should be gone
	if _, err := os.Stat(filepath.Join(repoPath, ".codemob", "mobs", "one")); err == nil {
		t.Error("worktree 'one' still exists after purge")
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".codemob", "mobs", "two")); err == nil {
		t.Error("worktree 'two' still exists after purge")
	}
}

// ─── Session Cleanup ─────────────────────────────────────────────────────────

func TestRemoveCleansSessionFiles(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "target", "--no-launch")
	runCore(t, bin, repoPath, "new", "other", "--no-launch")

	// given -> two sessions pointing to target, one to other
	writeSessionFile(t, repoPath, "sess-a", "target")
	writeSessionFile(t, repoPath, "sess-b", "target")
	writeSessionFile(t, repoPath, "sess-c", "other")

	// when -> remove target
	runCore(t, bin, repoPath, "remove", "target")

	// then -> session files for target should be gone
	sessDir := filepath.Join(repoPath, ".codemob", "sessions")
	if _, err := os.Stat(filepath.Join(sessDir, "sess-a")); err == nil {
		t.Error("session file sess-a still exists after removing target")
	}
	if _, err := os.Stat(filepath.Join(sessDir, "sess-b")); err == nil {
		t.Error("session file sess-b still exists after removing target")
	}

	// then -> session file for other should remain
	if _, err := os.Stat(filepath.Join(sessDir, "sess-c")); err != nil {
		t.Error("session file sess-c was incorrectly removed")
	}
}

func TestPurgeCleansSessionFiles(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "one", "--no-launch")
	runCore(t, bin, repoPath, "new", "two", "--no-launch")

	// given -> session files exist
	writeSessionFile(t, repoPath, "sess-x", "one")
	writeSessionFile(t, repoPath, "sess-y", "two")

	// when -> purge
	cmd := exec.Command(bin, "purge")
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader("y\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("purge failed: %s\n%s", err, out)
	}

	// then -> sessions directory should be gone
	sessDir := filepath.Join(repoPath, ".codemob", "sessions")
	if _, err := os.Stat(sessDir); err == nil {
		t.Error("sessions directory still exists after purge")
	}
}

func TestReconcileCleansSessionFiles(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "orphan", "--no-launch")
	runCore(t, bin, repoPath, "new", "alive", "--no-launch")

	// given -> session files for both mobs
	writeSessionFile(t, repoPath, "sess-orphan", "orphan")
	writeSessionFile(t, repoPath, "sess-alive", "alive")

	// given -> manually remove orphan's worktree (simulates external deletion)
	run(t, repoPath, "git", "worktree", "remove", filepath.Join(".codemob", "mobs", "orphan"))

	// when -> list triggers reconciliation
	runCore(t, bin, repoPath, "list")

	// then -> session file for orphan should be cleaned up
	sessDir := filepath.Join(repoPath, ".codemob", "sessions")
	if _, err := os.Stat(filepath.Join(sessDir, "sess-orphan")); err == nil {
		t.Error("session file for orphan still exists after reconciliation")
	}

	// then -> session file for alive should remain
	if _, err := os.Stat(filepath.Join(sessDir, "sess-alive")); err != nil {
		t.Error("session file for alive was incorrectly removed")
	}
}

// ─── Queue Validation ────────────────────────────────────────────────────────

func TestQueueUnknownAction(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when
	out := runCoreExpectError(t, bin, repoPath, "queue", "bogus", "target")

	// then
	if !strings.Contains(out, "unknown queue action") {
		t.Errorf("expected unknown action error, got: %s", out)
	}
}

func TestQueueSwitchRequiresTarget(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when -> switch with no target
	out := runCoreExpectError(t, bin, repoPath, "queue", "switch")

	// then
	if !strings.Contains(out, "requires a target") {
		t.Errorf("expected target required error, got: %s", out)
	}
}

// ─── Agent Flag ──────────────────────────────────────────────────────────────

func TestAgentMissingValue(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when -> --agent with no value
	out := runCoreExpectError(t, bin, repoPath, "new", "--agent")

	// then
	if !strings.Contains(out, "--agent requires") {
		t.Errorf("expected --agent requires value error, got: %s", out)
	}
}

func TestResumeRejectsUnknownFlags(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "test-mob", "--no-launch")

	// when -> --agent passed to --resume
	out := runCoreExpectError(t, bin, repoPath, "resume", "--agent", "codex")

	// then
	if !strings.Contains(out, "unknown flag") {
		t.Errorf("expected unknown flag error, got: %s", out)
	}
}

func TestNewRejectsUnknownFlags(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when
	out := runCoreExpectError(t, bin, repoPath, "new", "--typo", "--no-launch")

	// then
	if !strings.Contains(out, "unknown flag") {
		t.Errorf("expected unknown flag error, got: %s", out)
	}
}

func TestRemoveRejectsUnknownFlags(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "test-mob", "--no-launch")

	// when
	out := runCoreExpectError(t, bin, repoPath, "remove", "--typo")

	// then
	if !strings.Contains(out, "unknown flag") {
		t.Errorf("expected unknown flag error, got: %s", out)
	}
}

// ─── Session Tracking ─────────────────────────────────────────────────────────

// writeSessionFile creates a session file mapping a session ID to a mob name.
func writeSessionFile(t *testing.T, repoPath, sessionID, mobName string) {
	t.Helper()
	sessDir := filepath.Join(repoPath, ".codemob", "sessions")
	os.MkdirAll(sessDir, 0755)
	if err := os.WriteFile(filepath.Join(sessDir, sessionID), []byte(mobName), 0644); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}
}

// runCoreWithSession runs codemob with a CODEMOB_SESSION env var and optional stdin.
func runCoreWithSession(t *testing.T, bin, dir, sessionID, stdin string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Env = append(os.Environ(), "CODEMOB_SESSION="+sessionID)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestResumeDefaultsToSessionLastMob(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "alpha", "--no-launch")
	runCore(t, bin, repoPath, "new", "beta", "--no-launch")

	// given -> session file points to beta
	writeSessionFile(t, repoPath, "sess-1", "beta")

	// when -> resume with empty input (should use session default)
	out, err := runCoreWithSession(t, bin, repoPath, "sess-1", "\n", "resume", "--no-launch")

	// then -> should resume beta
	if err != nil {
		t.Fatalf("resume failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Resuming mob 'beta'") {
		t.Errorf("expected to resume 'beta' via session default, got: %s", out)
	}
}

func TestResumeShowsLastMobMarker(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "alpha", "--no-launch")
	runCore(t, bin, repoPath, "new", "beta", "--no-launch")

	// given -> session file points to alpha
	writeSessionFile(t, repoPath, "sess-2", "alpha")

	// when -> resume by explicit name (we still see the picker output)
	out, err := runCoreWithSession(t, bin, repoPath, "sess-2", "beta\n", "resume", "--no-launch")

	// then -> output should show ◀ marker next to alpha
	if err != nil {
		t.Fatalf("resume failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "alpha ◀") {
		t.Errorf("expected ◀ marker next to 'alpha', got: %s", out)
	}
	if !strings.Contains(out, "[alpha]") {
		t.Errorf("expected [alpha] default in prompt, got: %s", out)
	}
}

func TestResumeIgnoresRemovedMobInSession(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "temp", "--no-launch")
	runCore(t, bin, repoPath, "new", "keeper", "--no-launch")

	// given -> session points to temp, but we remove it
	writeSessionFile(t, repoPath, "sess-3", "temp")
	runCore(t, bin, repoPath, "remove", "temp")

	// when -> resume with empty input (session mob is gone, only one mob left)
	out, err := runCoreWithSession(t, bin, repoPath, "sess-3", "", "resume", "--no-launch")

	// then -> should auto-select the only remaining mob (keeper)
	if err != nil {
		t.Fatalf("resume failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Resuming mob 'keeper'") {
		t.Errorf("expected to resume 'keeper' (only remaining mob), got: %s", out)
	}
}

func TestSessionIsolation(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "mob-a", "--no-launch")
	runCore(t, bin, repoPath, "new", "mob-b", "--no-launch")

	// given -> two different sessions point to different mobs
	writeSessionFile(t, repoPath, "terminal-1", "mob-a")
	writeSessionFile(t, repoPath, "terminal-2", "mob-b")

	// when -> resume from terminal-1
	out1, err := runCoreWithSession(t, bin, repoPath, "terminal-1", "\n", "resume", "--no-launch")
	if err != nil {
		t.Fatalf("resume (terminal-1) failed: %v\n%s", err, out1)
	}

	// then -> should default to mob-a
	if !strings.Contains(out1, "Resuming mob 'mob-a'") {
		t.Errorf("terminal-1 should resume mob-a, got: %s", out1)
	}

	// when -> resume from terminal-2
	out2, err := runCoreWithSession(t, bin, repoPath, "terminal-2", "\n", "resume", "--no-launch")
	if err != nil {
		t.Fatalf("resume (terminal-2) failed: %v\n%s", err, out2)
	}

	// then -> should default to mob-b
	if !strings.Contains(out2, "Resuming mob 'mob-b'") {
		t.Errorf("terminal-2 should resume mob-b, got: %s", out2)
	}
}

func TestResumeWithoutSessionWorks(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "no-session", "--no-launch")

	// when -> resume with explicit name, no CODEMOB_SESSION set
	out := runCore(t, bin, repoPath, "resume", "no-session", "--no-launch")

	// then -> should work fine
	if !strings.Contains(out, "Resuming mob 'no-session'") {
		t.Errorf("expected to resume 'no-session', got: %s", out)
	}
}

// ─── Original Tests ──────────────────────────────────────────────────────────

func TestNewMobWithCustomAgent(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when
	runCore(t, bin, repoPath, "new", "codex-mob", "--agent", "codex", "--no-launch")

	// then -> config should reflect the agent
	cfg := readConfig(t, repoPath)
	mobs := cfg["mobs"].([]interface{})
	mob := mobs[0].(map[string]interface{})
	if mob["agent"] != "codex" {
		t.Errorf("expected agent=codex in config, got %v", mob["agent"])
	}
}

// ─── Open ────────────────────────────────────────────────────────────────────

func TestOpenRejectsUnknownFlags(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "open-test", "--no-launch")

	// when
	out := runCoreExpectError(t, bin, repoPath, "open", "--typo")

	// then
	if !strings.Contains(out, "unknown flag") {
		t.Errorf("expected 'unknown flag' error, got: %s", out)
	}
}

func TestOpenNotFound(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "exists", "--no-launch")

	// when
	out := runCoreExpectError(t, bin, repoPath, "open", "nonexistent")

	// then
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' error, got: %s", out)
	}
}

func TestOpenNoMobs(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when
	out := runCoreExpectError(t, bin, repoPath, "open", "anything")

	// then
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' error, got: %s", out)
	}
}

// ─── Path ────────────────────────────────────────────────────────────────────

func TestPathByName(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "path-test", "--no-launch")

	// when
	out := runCore(t, bin, repoPath, "path", "path-test")

	// then
	if !strings.HasSuffix(strings.TrimSpace(out), filepath.Join(".codemob", "mobs", "path-test")) {
		t.Errorf("expected path ending in .codemob/mobs/path-test, got %q", strings.TrimSpace(out))
	}
}

func TestPathByIndex(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "first", "--no-launch")
	runCore(t, bin, repoPath, "new", "second", "--no-launch")

	// when
	out := runCore(t, bin, repoPath, "path", "2")

	// then
	if !strings.HasSuffix(strings.TrimSpace(out), filepath.Join(".codemob", "mobs", "second")) {
		t.Errorf("expected path ending in .codemob/mobs/second, got %q", strings.TrimSpace(out))
	}
}

func TestPathRoot(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when
	out := runCore(t, bin, repoPath, "path", "root")

	// then
	if !strings.HasSuffix(strings.TrimSpace(out), "test-repo") {
		t.Errorf("expected path ending in test-repo, got %q", strings.TrimSpace(out))
	}
}

func TestPathRootWorksWithNoMobs(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// given -> no mobs exist

	// when
	out := runCore(t, bin, repoPath, "path", "root")

	// then
	if !strings.HasSuffix(strings.TrimSpace(out), "test-repo") {
		t.Errorf("expected path ending in test-repo, got %q", strings.TrimSpace(out))
	}
}

func TestPathNotFound(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)
	runCore(t, bin, repoPath, "new", "exists", "--no-launch")

	// when
	out := runCoreExpectError(t, bin, repoPath, "path", "nonexistent")

	// then
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' error, got: %s", out)
	}
}

func TestPathNoMobs(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// given -> no mobs exist

	// when
	out := runCoreExpectError(t, bin, repoPath, "path")

	// then
	if !strings.Contains(out, "no mobs") {
		t.Errorf("expected 'no mobs' error, got: %s", out)
	}
}

func TestPathReservedName(t *testing.T) {
	bin := buildCore(t)
	_, repoPath := setupTestRepo(t)
	initRepo(t, bin, repoPath)

	// when -> try to create a mob named "root"
	out := runCoreExpectError(t, bin, repoPath, "new", "root", "--no-launch")

	// then
	if !strings.Contains(out, "reserved") {
		t.Errorf("expected 'reserved' error, got: %s", out)
	}
}
