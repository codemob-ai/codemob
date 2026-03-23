package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/codemob-ai/codemob/internal/mob"
)

func cmdInfo() error {
	section := func(title string) { fmt.Printf("\n\033[1m%s\033[0m\n", title) }
	kv := func(key, val string) { fmt.Printf("  %-24s %s\n", key+":", val) }

	// --- General ---
	section("General")
	kv("version", Version)
	kv("os/arch", runtime.GOOS+"/"+runtime.GOARCH)
	kv("go", runtime.Version())

	cwd, _ := os.Getwd()
	kv("cwd", cwd)

	currentMob := mob.CurrentMobName()
	if currentMob != "" {
		kv("inside mob", currentMob)
	} else {
		kv("inside mob", "(no)")
	}

	// --- Repo ---
	section("Repo")
	root, err := mob.FindRepoRoot()
	if err != nil {
		kv("repo root", "(not found: "+err.Error()+")")
		fmt.Println()
		return nil
	}
	kv("repo root", root)
	kv("initialized", fmt.Sprintf("%v", mob.IsInitialized(root)))

	// --- Config ---
	section("Config")
	cfg, err := mob.LoadConfig(root)
	if err != nil {
		kv("config", "(error: "+err.Error()+")")
	} else {
		data, _ := json.MarshalIndent(cfg, "  ", "  ")
		fmt.Printf("  %s\n", data)
	}

	// --- Worktrees on disk ---
	section("Worktrees on disk")
	mobsPath := filepath.Join(root, mob.MobsDir)
	entries, err := os.ReadDir(mobsPath)
	if err != nil {
		kv("mobs dir", "(not found)")
	} else if len(entries) == 0 {
		kv("mobs dir", "(empty)")
	} else {
		for _, e := range entries {
			if e.IsDir() {
				kv(e.Name(), filepath.Join(mobsPath, e.Name()))
			}
		}
	}

	// --- Git worktrees ---
	section("Git worktrees")
	out, err := exec.Command("git", "-C", root, "worktree", "list").Output()
	if err != nil {
		kv("git worktree list", "(error: "+err.Error()+")")
	} else {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			fmt.Printf("  %s\n", line)
		}
	}

	// --- Queue ---
	section("Queue")
	queuePath := filepath.Join(root, mob.CodemobDir, "queue.json")
	queueData, err := os.ReadFile(queuePath)
	if err != nil {
		kv("queue.json", "(none)")
	} else {
		fmt.Printf("  %s\n", strings.TrimSpace(string(queueData)))
	}

	// --- Session ---
	section("Session")
	sessionID := os.Getenv("CODEMOB_SESSION")
	if sessionID == "" {
		kv("CODEMOB_SESSION", "(not set)")
	} else {
		kv("CODEMOB_SESSION", sessionID)
		sessFile := filepath.Join(root, mob.CodemobDir, "sessions", sessionID)
		data, err := os.ReadFile(sessFile)
		if err != nil {
			kv("last mob (this session)", "(none)")
		} else {
			kv("last mob (this session)", strings.TrimSpace(string(data)))
		}
	}

	sessDir := filepath.Join(root, mob.CodemobDir, "sessions")
	sessEntries, err := os.ReadDir(sessDir)
	if err == nil && len(sessEntries) > 0 {
		fmt.Println()
		kv("all session files", "")
		for _, e := range sessEntries {
			if e.IsDir() {
				continue
			}
			data, _ := os.ReadFile(filepath.Join(sessDir, e.Name()))
			val := strings.TrimSpace(string(data))
			marker := ""
			if e.Name() == sessionID {
				marker = " (current)"
			}
			fmt.Printf("    %-40s → %s%s\n", e.Name(), val, marker)
		}
	}

	// --- Agents ---
	section("Agents")
	for _, agent := range []string{"claude", "codex"} {
		path, err := exec.LookPath(agent)
		if err != nil {
			kv(agent, "(not found)")
			continue
		}
		ver, err := exec.Command(path, "--version").Output()
		if err != nil {
			kv(agent, path+" (version unknown)")
		} else {
			v := strings.TrimSpace(string(ver))
			if i := strings.IndexByte(v, '\n'); i != -1 {
				v = v[:i]
			}
			kv(agent, path+" ("+v+")")
		}
	}

	// --- Shell ---
	section("Shell")
	kv("SHELL", os.Getenv("SHELL"))

	fmt.Println()
	return nil
}
