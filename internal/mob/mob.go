package mob

import (
	"encoding/json"
	"fmt"
	"os"
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
	DefaultAgent string `json:"default_agent"`
	BaseBranch   string `json:"base_branch"`
	Mobs         []Mob  `json:"mobs"`
}

// FindRepoRoot finds the main repo root, accounting for being inside a mob worktree.
func FindRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Check if we're inside a mob worktree
	if idx := strings.Index(cwd, "/"+MobsDir+"/"); idx != -1 {
		return cwd[:idx], nil
	}

	return gitutil.RepoRoot()
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
func Reconcile(repoRoot string, cfg *Config) bool {
	changed := false
	var valid []Mob
	for _, m := range cfg.Mobs {
		mobPath := filepath.Join(repoRoot, MobsDir, m.Name)
		if _, err := os.Stat(mobPath); err == nil {
			valid = append(valid, m)
		} else {
			changed = true
		}
	}
	cfg.Mobs = valid
	return changed
}

// CurrentMobName returns the name of the mob we're currently inside, or "" if not in a mob.
func CurrentMobName() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	marker := "/" + MobsDir + "/"
	idx := strings.Index(cwd, marker)
	if idx == -1 {
		return ""
	}
	rest := cwd[idx+len(marker):]
	// Take the first path component
	if slash := strings.Index(rest, "/"); slash != -1 {
		return rest[:slash]
	}
	return rest
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
