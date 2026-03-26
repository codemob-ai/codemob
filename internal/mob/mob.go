package mob

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	gitutil "github.com/codemob-ai/codemob/internal/git"
)

const (
	CodemobDir  = ".codemob"
	MobsDir     = ".codemob/mobs"
	ConfigFile  = ".codemob/config.json"
)

type Mob struct {
	Name      string `json:"name"`
	Branch    string `json:"branch"`
	CreatedAt string `json:"created_at"`
	Agent     string `json:"agent"`
}

type Config struct {
	DefaultAgent    string `json:"default_agent"`
	BaseBranch      string `json:"base_branch"`
	RepoRoot        string `json:"repo_root,omitempty"`
	MobsDirPath     string `json:"mobs_dir,omitempty"`
	PostCreateScript string `json:"post_create_script"`
	Mobs            []Mob  `json:"mobs"`
}

// MobsPath returns the absolute path to the mobs directory.
func MobsPath(repoRoot string, cfg *Config) string {
	if cfg != nil && cfg.MobsDirPath != "" {
		return cfg.MobsDirPath
	}
	return filepath.Join(repoRoot, MobsDir)
}

// MobPath returns the absolute path to a specific mob's worktree.
func MobPath(repoRoot string, cfg *Config, name string) string {
	return filepath.Join(MobsPath(repoRoot, cfg), name)
}

// FindRepoRoot finds the main repo root, accounting for being inside a mob worktree.
func FindRepoRoot() (string, error) {
	mainRoot, toplevel := insideWorktreeEx()
	if mainRoot != "" {
		return mainRoot, nil
	}
	// Not in a worktree. toplevel is already computed from the slow path
	// (empty if the fast path matched or git failed).
	if toplevel != "" {
		return toplevel, nil
	}
	return gitutil.RepoRoot()
}

// InsideWorktree returns the repo root if the current directory is inside a
// codemob worktree, or empty string if not.
func InsideWorktree() string {
	root, _ := insideWorktreeEx()
	return root
}

// insideWorktreeEx detects if we're inside a codemob worktree.
// Returns (mainRepoRoot, toplevel). mainRepoRoot is non-empty only when inside
// a codemob worktree. toplevel is the git toplevel from the slow path (may be
// empty if the fast path matched or git is unavailable).
func insideWorktreeEx() (mainRoot, toplevel string) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", ""
	}

	// Fast path: project-dir mode (worktrees inside the repo)
	if idx := strings.Index(cwd, "/"+MobsDir+"/"); idx != -1 {
		return cwd[:idx], ""
	}

	// Slow path: external worktrees - use git to detect
	toplevel, err = gitutil.RepoRoot()
	if err != nil {
		return "", ""
	}
	commonDir, err := gitutil.CommonDir()
	if err != nil {
		return "", toplevel
	}

	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(toplevel, commonDir)
	}
	commonDir = filepath.Clean(commonDir)

	mainRoot = filepath.Dir(commonDir)

	// Resolve symlinks before comparing (macOS /var -> /private/var, etc.)
	if resolved, err := filepath.EvalSymlinks(mainRoot); err == nil {
		mainRoot = resolved
	}
	if resolved, err := filepath.EvalSymlinks(toplevel); err == nil {
		toplevel = resolved
	}

	// If toplevel == mainRoot, we're in the main repo, not a worktree
	if toplevel == mainRoot {
		return "", toplevel
	}

	// We're in a worktree. Check if the main repo has codemob initialized.
	if _, err := os.Stat(filepath.Join(mainRoot, ConfigFile)); err != nil {
		return "", toplevel
	}
	return mainRoot, toplevel
}

// IsInitialized checks if codemob is initialized in the given repo.
func IsInitialized(repoRoot string) bool {
	_, err := os.Stat(filepath.Join(repoRoot, CodemobDir))
	return err == nil
}

// LoadConfig reads the config from disk.
func LoadConfig(repoRoot string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, ConfigFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &cfg, nil
}

// SaveConfig writes the config to disk.
func SaveConfig(repoRoot string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(filepath.Join(repoRoot, ConfigFile), append(data, '\n'), 0644)
}

// Reconcile removes mobs from config whose worktree no longer exists on disk.
// Returns the names of removed mobs (empty if nothing changed).
func Reconcile(repoRoot string, cfg *Config) []string {
	var removed []string
	valid := make([]Mob, 0)
	for _, m := range cfg.Mobs {
		mobPath := MobPath(repoRoot, cfg, m.Name)
		if _, err := os.Stat(mobPath); err == nil {
			valid = append(valid, m)
		} else {
			removed = append(removed, m.Name)
		}
	}
	cfg.Mobs = valid
	return removed
}

// ValidateName checks if a mob name is safe for use in paths and branches.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("mob name cannot be empty")
	}
	if len(name) > 60 {
		return fmt.Errorf("mob name too long (max 60 characters)")
	}
	// Reject all-numeric names — ambiguous with index-based resolution
	allDigits := true
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-') {
			return fmt.Errorf("mob name can only contain letters, numbers, and hyphens")
		}
		if c < '0' || c > '9' {
			allDigits = false
		}
	}
	if allDigits {
		return fmt.Errorf("mob name cannot be purely numeric (conflicts with index-based selection)")
	}
	if name == "root" {
		return fmt.Errorf("mob name 'root' is reserved")
	}
	if name[0] == '-' || name[len(name)-1] == '-' {
		return fmt.Errorf("mob name cannot start or end with a hyphen")
	}
	return nil
}

// CurrentMobName returns the name of the mob we're currently inside, or "" if not in a mob.
func CurrentMobName() string {
	if name := os.Getenv("CODEMOB_MOB"); name != "" {
		return name
	}
	return currentMobNameFromCwd("")
}

// CurrentMobNameForRoot returns the mob name when the repo root is already known,
// avoiding a redundant InsideWorktree call.
func CurrentMobNameForRoot(repoRoot string) string {
	if name := os.Getenv("CODEMOB_MOB"); name != "" {
		return name
	}
	return currentMobNameFromCwd(repoRoot)
}

func currentMobNameFromCwd(knownRoot string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Fast path: path-based detection (works for project-dir mode)
	marker := "/" + MobsDir + "/"
	if idx := strings.Index(cwd, marker); idx != -1 {
		rest := cwd[idx+len(marker):]
		if slash := strings.Index(rest, "/"); slash != -1 {
			return rest[:slash]
		}
		return rest
	}

	// Slow path: external worktrees - load config and match cwd against mob paths
	root := knownRoot
	if root == "" {
		root = InsideWorktree()
		if root == "" {
			return ""
		}
	}
	cfg, err := LoadConfig(root)
	if err != nil {
		return ""
	}
	for _, m := range cfg.Mobs {
		mobPath := MobPath(root, cfg, m.Name)
		if cwd == mobPath || strings.HasPrefix(cwd, mobPath+"/") {
			return m.Name
		}
	}
	return ""
}

// CleanupExternalMobsDir removes an external mobs directory and its empty parent directories.
// Skips cleanup if the path is inside repoRoot (project-dir mode).
// Used by uninstall to fully remove the directory tree.
func CleanupExternalMobsDir(repoRoot, mobsDirPath string) {
	if mobsDirPath == "" || strings.HasPrefix(mobsDirPath, repoRoot+"/") {
		return
	}
	os.RemoveAll(mobsDirPath)
	parent := filepath.Dir(mobsDirPath)
	os.Remove(parent)
	grandparent := filepath.Dir(parent)
	if filepath.Base(grandparent) == ".codemob" {
		os.Remove(grandparent)
	}
}

// CleanMobsDirContents removes everything inside the mobs directory
// but keeps the directory itself so the configured path remains valid.
func CleanMobsDirContents(mobsDirPath string) {
	if mobsDirPath == "" {
		return
	}
	entries, err := os.ReadDir(mobsDirPath)
	if err != nil {
		return
	}
	for _, e := range entries {
		os.RemoveAll(filepath.Join(mobsDirPath, e.Name()))
	}
}

// RunPostCreateScript executes the configured post-create script in the given worktree directory.
// Returns nil if no script is configured. Returns an error if the script fails.
func RunPostCreateScript(cfg *Config, worktreePath string) error {
	if cfg.PostCreateScript == "" {
		return nil
	}

	scriptPath := cfg.PostCreateScript
	if !filepath.IsAbs(scriptPath) {
		scriptPath = filepath.Join(cfg.RepoRoot, scriptPath)
	}

	info, err := os.Stat(scriptPath)
	if err != nil {
		return fmt.Errorf("post_create_script not found: %s", scriptPath)
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("post_create_script is not executable: %s (run chmod +x)", scriptPath)
	}

	cmd := exec.Command(scriptPath)
	cmd.Dir = worktreePath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("post_create_script failed: %w", err)
	}
	return nil
}

// FindMob finds a mob by name.
func FindMob(cfg *Config, name string) *Mob {
	for i := range cfg.Mobs {
		if cfg.Mobs[i].Name == name {
			return &cfg.Mobs[i]
		}
	}
	return nil
}


// ActualBranch returns the current branch of a mob's worktree.
// Falls back to the config-stored branch if the worktree can't be read.
func ActualBranch(repoRoot string, cfg *Config, m *Mob) string {
	worktreePath := MobPath(repoRoot, cfg, m.Name)
	if branch, err := gitutil.CurrentBranch(worktreePath); err == nil {
		return branch
	}
	return m.Branch
}

// RelativeTime formats a timestamp as a human-readable relative time.
func RelativeTime(timestamp string) string {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return timestamp
	}
	diff := time.Since(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	}
}
