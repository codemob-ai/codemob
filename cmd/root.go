package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	gitutil "github.com/codemob-ai/codemob/internal/git"
	"github.com/codemob-ai/codemob/internal/mob"
)

func mobStatus(msg string) {
	fmt.Println()
	// Brand accent: #e7dc60
	fmt.Printf("  \033[38;2;231;220;96m● codemob\033[0m  %s\n", msg)
	fmt.Println()
}

// mobProgress prints a transient progress line and returns a handle to finish it.
// Use defer p.Clear() immediately after calling to guarantee cleanup on any exit path.
type progress struct{ active bool }

func mobProgress(msg string) *progress {
	// Leading blank line + message, no trailing newline — cursor stays on this line
	fmt.Printf("\n  \033[38;2;231;220;96m● codemob\033[0m  %s", msg)
	return &progress{active: true}
}

// Done replaces the progress line with a final status message.
func (p *progress) Done(msg string) {
	if !p.active {
		return
	}
	p.active = false
	fmt.Printf("\r\033[2K  \033[38;2;231;220;96m● codemob\033[0m  %s\n\n", msg)
}

// Clear removes the progress line without replacing it.
func (p *progress) Clear() {
	if !p.active {
		return
	}
	p.active = false
	fmt.Printf("\r\033[2K")
}

// Version is set at build time via ldflags.
var Version = "dev"

func Execute() error {
	// Clear stale next action on every invocation (except check-queue which reads it)
	if len(os.Args) < 2 || os.Args[1] != "check-queue" {
		if root, err := mob.FindRepoRoot(); err == nil {
			mob.ClearQueue(root)
		}
	}

	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	// Commands
	case "new":
		return cmdNew(args)
	case "list", "ls":
		return cmdList(args, false)
	case "resume":
		return cmdResume(args)
	case "init":
		return cmdInit(args, false)
	case "reinit":
		return cmdInit(args, true)
	case "uninstall":
		return cmdUninstall(args)
	case "remove":
		return cmdRemove(args)
	case "purge":
		return cmdPurge(args)
	case "path":
		return cmdPath(args)
	case "info":
		return cmdInfo()

	// Internal (used by shell wrapper and slash commands)
	case "switch":
		return cmdResume(args)
	case "list-others":
		return cmdList(args, true)
	case "check-queue":
		return cmdCheckNext(args)
	case "queue":
		return cmdWriteNext(args)

	case "version", "--version", "-v":
		fmt.Printf("codemob %s\n", Version)
		return nil
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s. Run 'codemob help' for usage.", cmd)
	}
}

func cmdUninstall(_ []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine install directory: %w", err)
	}
	installDir := filepath.Dir(exe)
	return mob.Uninstall(installDir)
}

// resolveMob finds a mob by name or 1-based index.
func resolveMob(cfg *mob.Config, nameOrIndex string) *mob.Mob {
	// Try as index first
	if idx, err := strconv.Atoi(nameOrIndex); err == nil {
		if idx >= 1 && idx <= len(cfg.Mobs) {
			return &cfg.Mobs[idx-1]
		}
	}
	// Fall back to name
	return mob.FindMob(cfg, nameOrIndex)
}

func cmdInit(_ []string, forceReprompt bool) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine install directory: %w", err)
	}
	installDir := filepath.Dir(exe)
	return mob.Init(installDir, forceReprompt)
}

func requireInit() (string, *mob.Config, error) {
	root, err := mob.FindRepoRoot()
	if err != nil {
		return "", nil, err
	}
	if !mob.IsInitialized(root) {
		return "", nil, fmt.Errorf("codemob not initialized in this repo. Run: codemob init")
	}
	cfg, err := mob.LoadConfig(root)
	if err != nil {
		return "", nil, err
	}
	if removed := mob.Reconcile(root, cfg); len(removed) > 0 {
		_ = mob.SaveConfig(root, cfg)
		cleanSessionFiles(root, removed...)
	}
	return root, cfg, nil
}

func cmdNew(args []string) error {
	root, cfg, err := requireInit()
	if err != nil {
		return err
	}

	name := ""
	agent := cfg.DefaultAgent
	noLaunch := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--no-launch":
			noLaunch = true
		case "--agent":
			if i+1 >= len(args) {
				return fmt.Errorf("--agent requires a value (e.g., --agent codex)")
			}
			agent = args[i+1]
			i++
		default:
			if name == "" {
				name = args[i]
			}
		}
	}

	if name == "" {
		name = mob.GenerateUniqueName(cfg)
	} else {
		if err := mob.ValidateName(name); err != nil {
			return err
		}
		if m := mob.FindMob(cfg, name); m != nil {
			return fmt.Errorf("mob '%s' already exists", name)
		}
	}

	branch := "mob/" + name
	worktreePath := filepath.Join(root, mob.MobsDir, name)

	p := mobProgress(fmt.Sprintf("Creating mob '%s'...", name))
	defer p.Clear()

	if err := gitutil.WorktreeAdd(root, worktreePath, branch, cfg.BaseBranch); err != nil {
		return err
	}

	cfg.Mobs = append(cfg.Mobs, mob.Mob{
		Name:      name,
		Branch:    branch,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Agent:     agent,
	})

	if err := mob.SaveConfig(root, cfg); err != nil {
		return err
	}

	p.Done(fmt.Sprintf("Created mob '%s' on branch %s", name, branch))

	if !noLaunch {
		return launchAgent(root, agent, worktreePath, false)
	}
	return nil
}

func cmdList(_ []string, excludeCurrent bool) error {
	_, cfg, err := requireInit()
	if err != nil {
		return err
	}

	mobs := cfg.Mobs
	if excludeCurrent {
		current := mob.CurrentMobName()
		var filtered []mob.Mob
		for _, m := range mobs {
			if m.Name != current {
				filtered = append(filtered, m)
			}
		}
		mobs = filtered
	}

	if len(mobs) == 0 {
		fmt.Println("No mobs. Create one with: codemob new <name>")
		return nil
	}

	currentMob := mob.CurrentMobName()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "#\tNAME\tBRANCH\tLAST AGENT\tCREATED")
	for i, m := range mobs {
		marker := ""
		if m.Name == currentMob {
			marker = " ◀"
		}
		fmt.Fprintf(w, "%d\t%s%s\t%s\t%s\t%s\n", i+1, m.Name, marker, m.Branch, m.Agent, mob.RelativeTime(m.CreatedAt))
	}
	w.Flush()
	return nil
}

func cmdResume(args []string) error {
	root, cfg, err := requireInit()
	if err != nil {
		return err
	}

	name := ""
	noLaunch := false
	for _, arg := range args {
		switch {
		case arg == "--no-launch":
			noLaunch = true
		case strings.HasPrefix(arg, "--"):
			return fmt.Errorf("unknown flag for resume: %s", arg)
		default:
			if name == "" {
				name = arg
			}
		}
	}

	if name == "" {
		if len(cfg.Mobs) == 0 {
			return fmt.Errorf("no mobs. Create one with: codemob new")
		}
		if len(cfg.Mobs) == 1 {
			name = cfg.Mobs[0].Name
		} else {
			lastMob := readLastMob(root)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "#\tNAME\tBRANCH\tLAST AGENT\tCREATED")
			for i, m := range cfg.Mobs {
				marker := ""
				if m.Name == lastMob {
					marker = " ◀"
				}
				fmt.Fprintf(w, "%d\t%s%s\t%s\t%s\t%s\n", i+1, m.Name, marker, m.Branch, m.Agent, mob.RelativeTime(m.CreatedAt))
			}
			w.Flush()
			if lastMob != "" && mob.FindMob(cfg, lastMob) != nil {
				fmt.Printf("\nWhich mob? (#/name) [%s]: ", lastMob)
			} else {
				fmt.Print("\nWhich mob? (#/name): ")
				lastMob = ""
			}
			fmt.Scanln(&name)
			if name == "" {
				name = lastMob
			}
		}
		if name == "" {
			return fmt.Errorf("no mob selected")
		}
	}

	m := resolveMob(cfg, name)
	if m == nil {
		return fmt.Errorf("mob '%s' not found", name)
	}

	worktreePath := filepath.Join(root, mob.MobsDir, m.Name)
	mobStatus(fmt.Sprintf("Resuming mob '%s'", m.Name))

	if !noLaunch {
		return launchAgent(root, m.Agent, worktreePath, true)
	}
	return nil
}


func cmdRemove(args []string) error {
	root, cfg, err := requireInit()
	if err != nil {
		return err
	}

	name := ""
	force := false
	for _, arg := range args {
		switch arg {
		case "--force", "-f":
			force = true
		default:
			if name == "" {
				name = arg
			}
		}
	}

	if name == "" {
		return fmt.Errorf("mob name required")
	}

	m := resolveMob(cfg, name)
	if m == nil {
		return fmt.Errorf("mob '%s' not found", name)
	}

	worktreePath := filepath.Join(root, mob.MobsDir, m.Name)
	if _, err := os.Stat(worktreePath); err == nil {
		if err := gitutil.WorktreeRemove(root, worktreePath, force); err != nil {
			return err
		}
	}

	_ = gitutil.BranchDelete(root, m.Branch)

	var remaining []mob.Mob
	for _, existing := range cfg.Mobs {
		if existing.Name != m.Name {
			remaining = append(remaining, existing)
		}
	}
	cfg.Mobs = remaining

	if err := mob.SaveConfig(root, cfg); err != nil {
		return err
	}

	cleanSessionFiles(root, m.Name)
	mobStatus(fmt.Sprintf("Removed mob '%s'", m.Name))
	return nil
}

// cleanSessionFiles removes session files that point to any of the given mob names.
func cleanSessionFiles(root string, names ...string) {
	sessDir := filepath.Join(root, mob.CodemobDir, "sessions")
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		return
	}
	remove := make(map[string]bool, len(names))
	for _, n := range names {
		remove[n] = true
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sessDir, e.Name()))
		if err != nil {
			continue
		}
		if remove[strings.TrimSpace(string(data))] {
			os.Remove(filepath.Join(sessDir, e.Name()))
		}
	}
}

func cmdPurge(_ []string) error {
	root, cfg, err := requireInit()
	if err != nil {
		return err
	}

	if len(cfg.Mobs) == 0 {
		fmt.Println("No mobs to purge.")
		return nil
	}

	r := mob.ColorRed
	rst := mob.ColorReset

	fmt.Println()
	fmt.Printf("  %s⚠ DESTRUCTIVE OPERATION%s\n", r, rst)
	fmt.Println()
	fmt.Printf("  This will permanently remove all %s%d mob(s)%s and their worktrees.\n", r, len(cfg.Mobs), rst)
	fmt.Printf("  Any %suncommitted or unpushed changes%s in those worktrees will be %spermanently lost%s.\n", r, rst, r, rst)
	fmt.Println()
	fmt.Printf("  %sThis cannot be undone.%s\n", r, rst)
	fmt.Print("\n  Are you sure? [y/N]: ")

	var input string
	fmt.Scanln(&input)
	if input != "y" && input != "yes" {
		fmt.Println("  Cancelled.")
		return nil
	}

	mob.PrintBanner(mob.ColorRed)

	for _, m := range cfg.Mobs {
		worktreePath := filepath.Join(root, mob.MobsDir, m.Name)
		if _, err := os.Stat(worktreePath); err == nil {
			_ = gitutil.WorktreeRemove(root, worktreePath, true)
		}
		_ = gitutil.BranchDelete(root, m.Branch)
		fmt.Printf("  %s✗%s Removed '%s'\n", r, rst, m.Name)
	}

	cfg.Mobs = nil
	if err := mob.SaveConfig(root, cfg); err != nil {
		return err
	}

	os.RemoveAll(filepath.Join(root, mob.CodemobDir, "sessions"))

	fmt.Println()
	fmt.Printf("  %s● codemob%s  All mobs purged\n", r, rst)
	fmt.Println()
	return nil
}

func cmdPath(args []string) error {
	root, cfg, err := requireInit()
	if err != nil {
		return err
	}

	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	if name == "root" {
		fmt.Println(root)
		return nil
	}

	if len(cfg.Mobs) == 0 {
		return fmt.Errorf("no mobs. Create one with: codemob new")
	}

	if name == "" {
		if len(cfg.Mobs) == 1 && mob.CurrentMobName() == "" {
			name = cfg.Mobs[0].Name
		} else {
			inMob := mob.CurrentMobName() != ""
			w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "#\tNAME\tBRANCH\tLAST AGENT\tCREATED")
			for i, m := range cfg.Mobs {
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i+1, m.Name, m.Branch, m.Agent, mob.RelativeTime(m.CreatedAt))
			}
			w.Flush()
			if inMob {
				fmt.Fprintf(os.Stderr, "\n  \033[38;2;100;180;220m> enter 0 to cd back to repo root\033[0m\n")
			}
			fmt.Fprint(os.Stderr, "\nWhich mob? (#/name): ")
			fmt.Scanln(&name)
		}
		if name == "" {
			return fmt.Errorf("no mob selected")
		}
	}

	if name == "0" || name == "root" {
		fmt.Println(root)
		return nil
	}

	m := resolveMob(cfg, name)
	if m == nil {
		return fmt.Errorf("mob '%s' not found", name)
	}

	fmt.Println(filepath.Join(root, mob.MobsDir, m.Name))
	return nil
}

func cmdCheckNext(_ []string) error {
	root, err := mob.FindRepoRoot()
	if err != nil {
		return nil // not in a repo, nothing to do
	}

	next, err := mob.ReadQueuedAction(root)
	if err != nil || next == nil {
		return nil // no queued action
	}
	mob.ClearQueue(root)

	return executeNextAction(root, next)
}

// resolveNextAction resolves a next action to a workdir, agent, and resume flag.
func resolveNextAction(root string, next *mob.QueuedAction) (workdir, agent string, resume bool, err error) {
	cfg, err := mob.LoadConfig(root)
	if err != nil {
		return "", "", false, err
	}

	switch next.Action {
	case "switch":
		m := mob.FindMob(cfg, next.Target)
		if m == nil {
			return "", "", false, fmt.Errorf("mob '%s' not found", next.Target)
		}
		mobStatus(fmt.Sprintf("Switching to mob '%s'", m.Name))
		return filepath.Join(root, mob.MobsDir, m.Name), m.Agent, true, nil

	case "change-agent":
		currentName := next.Mob
		if currentName == "" {
			return "", "", false, fmt.Errorf("could not determine current mob")
		}
		m := mob.FindMob(cfg, currentName)
		if m == nil {
			return "", "", false, fmt.Errorf("mob '%s' not found", currentName)
		}
		newAgent := next.Target
		if newAgent == "" {
			return "", "", false, fmt.Errorf("agent name required")
		}
		if _, err := exec.LookPath(newAgent); err != nil {
			return "", "", false, fmt.Errorf("agent '%s' is not installed", newAgent)
		}
		// Update the mob's agent in config
		m.Agent = newAgent
		_ = mob.SaveConfig(root, cfg)
		mobStatus(fmt.Sprintf("Switching mob '%s' to agent '%s'", m.Name, newAgent))
		return filepath.Join(root, mob.MobsDir, m.Name), newAgent, false, nil

	case "new":
		name := next.Target
		if name == "" {
			name = mob.GenerateUniqueName(cfg)
		} else {
			if err := mob.ValidateName(name); err != nil {
				return "", "", false, err
			}
			if m := mob.FindMob(cfg, name); m != nil {
				return "", "", false, fmt.Errorf("mob '%s' already exists", name)
			}
		}
		branch := "mob/" + name
		worktreePath := filepath.Join(root, mob.MobsDir, name)

		p := mobProgress(fmt.Sprintf("Creating mob '%s'...", name))
		defer p.Clear()

		if err := gitutil.WorktreeAdd(root, worktreePath, branch, cfg.BaseBranch); err != nil {
			return "", "", false, err
		}

		agent := next.Agent
		if agent == "" {
			agent = cfg.DefaultAgent
		}

		cfg.Mobs = append(cfg.Mobs, mob.Mob{
			Name:      name,
			Branch:    branch,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			Agent:     agent,
		})
		if err := mob.SaveConfig(root, cfg); err != nil {
			return "", "", false, err
		}

		p.Done(fmt.Sprintf("Created mob '%s' on branch %s", name, branch))
		return worktreePath, agent, false, nil

	case "remove":
		name := next.Target
		if name == "" {
			return "", "", false, fmt.Errorf("mob name required for remove")
		}
		m := mob.FindMob(cfg, name)
		if m == nil {
			return "", "", false, fmt.Errorf("mob '%s' not found", name)
		}
		worktreePath := filepath.Join(root, mob.MobsDir, m.Name)
		if _, statErr := os.Stat(worktreePath); statErr == nil {
			_ = gitutil.WorktreeRemove(root, worktreePath, true)
		}
		_ = gitutil.BranchDelete(root, m.Branch)
		var remaining []mob.Mob
		for _, existing := range cfg.Mobs {
			if existing.Name != m.Name {
				remaining = append(remaining, existing)
			}
		}
		cfg.Mobs = remaining
		_ = mob.SaveConfig(root, cfg)
		cleanSessionFiles(root, m.Name)
		mobStatus(fmt.Sprintf("Removed mob '%s'", m.Name))
		return "", "", false, nil // no agent to launch

	default:
		return "", "", false, fmt.Errorf("unknown next action: %s", next.Action)
	}
}

// executeNextAction resolves and immediately launches the agent for a next action.
func executeNextAction(root string, next *mob.QueuedAction) error {
	workdir, agent, resume, err := resolveNextAction(root, next)
	if err != nil {
		return err
	}
	if workdir == "" {
		return nil // action completed, no agent to launch (e.g., remove)
	}
	return launchAgent(root, agent, workdir, resume)
}


// cmdWriteNext writes a next action for the trampoline.
// Used by slash commands: codemob queue switch <mob-name>
func cmdWriteNext(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: codemob queue <action> [target]")
	}
	root, err := mob.FindRepoRoot()
	if err != nil {
		return err
	}
	action := args[0]
	if !mob.ValidQueueActions[action] {
		return fmt.Errorf("unknown queue action: %s", action)
	}
	target := ""
	if len(args) >= 2 {
		target = args[1]
	}

	// Validate: switch, remove, and change-agent require a target
	if target == "" && (action == "switch" || action == "remove" || action == "change-agent") {
		return fmt.Errorf("codemob queue %s requires a target", action)
	}

	q := mob.QueuedAction{Action: action, Target: target}

	// For change-agent, record which mob we're in right now
	if action == "change-agent" {
		q.Mob = mob.CurrentMobName()
	}

	// For new, carry the current mob's agent so the new mob uses the same one
	if action == "new" {
		currentName := mob.CurrentMobName()
		if currentName != "" {
			cfg, err := mob.LoadConfig(root)
			if err == nil {
				if m := mob.FindMob(cfg, currentName); m != nil {
					q.Agent = m.Agent
				}
			}
		}
	}

	return mob.WriteQueuedAction(root, q)
}

// launchAgent spawns the agent as a child process and implements the trampoline loop.
// After the agent exits, it checks for a next action (e.g., switch to another mob).
// On final exit, writes the last active mob name to .codemob/sessions/<session-id>
// (keyed by $CODEMOB_SESSION) so resume can default to it.
func launchAgent(root, agent, workdir string, resume bool) error {
	for {
		if err := runAgent(root, agent, workdir, resume); err != nil {
			// Log non-signal errors (signal exits are normal — user pressed Ctrl+C)
			if _, ok := err.(*exec.ExitError); !ok {
				fmt.Fprintf(os.Stderr, "  [codemob] agent error: %v\n", err)
			}
		}

		// Always check for queued action, regardless of how the agent exited
		next, err := mob.ReadQueuedAction(root)
		if err != nil || next == nil {
			writeLastMob(workdir)
			return nil // normal exit
		}
		mob.ClearQueue(root)

		newWorkdir, newAgent, newResume, err := resolveNextAction(root, next)
		if err != nil {
			return err
		}
		if newWorkdir == "" {
			return nil // action completed (e.g., remove) — don't write last mob
		}
		workdir = newWorkdir
		agent = newAgent
		resume = newResume
	}
}

// readLastMob returns the last active mob name for this terminal session.
func readLastMob(repoRoot string) string {
	sessionID := os.Getenv("CODEMOB_SESSION")
	if sessionID == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, mob.CodemobDir, "sessions", sessionID))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// writeLastMob persists the last active mob for this terminal session.
// Uses $CODEMOB_SESSION (set once per terminal by codemob-shell.sh) as the
// file key under .codemob/sessions/, so parallel terminals don't collide.
func writeLastMob(workdir string) {
	sessionID := os.Getenv("CODEMOB_SESSION")
	if sessionID == "" {
		return
	}
	root, err := mob.FindRepoRoot()
	if err != nil {
		return
	}
	sessDir := filepath.Join(root, mob.CodemobDir, "sessions")
	os.MkdirAll(sessDir, 0755)
	os.WriteFile(filepath.Join(sessDir, sessionID), []byte(filepath.Base(workdir)), 0644)
}

// runAgent spawns the agent process and waits for it to exit.
// If resume is true and the agent fails (e.g., no session to continue), falls back to a new session.
func runAgent(root, agent, workdir string, resume bool) error {
	binPath, resumeArgs, newArgs, err := agentArgs(agent, root)
	if err != nil {
		return err
	}

	if resume {
		err := spawnAgent(binPath, resumeArgs, workdir)
		if err == nil {
			return nil
		}
		// Only fall back to new session if the agent exited with a non-zero code
		// (typical for "no session to continue"). Other errors (binary not found,
		// permission denied) should propagate.
		if _, ok := err.(*exec.ExitError); !ok {
			return err
		}
		mobStatus("No previous session found, starting new session")
	}

	return spawnAgent(binPath, newArgs, workdir)
}

func agentArgs(agent, repoRoot string) (binPath string, resumeArgs, newArgs []string, err error) {
	worktreeHint := "IMPORTANT: You are working inside a codemob worktree. " +
		"This IS a full git repository — all files and history are available here. " +
		"Use your current working directory as the project root. " +
		"Do NOT navigate to " + repoRoot + " — that is the main repo and may be on a different branch with different files. " +
		"When spawning subagents (Explore, Agent, etc.), instruct them to work in the current directory, not " + repoRoot + "."

	switch agent {
	case "claude":
		binPath, err = exec.LookPath("claude")
		resumeArgs = []string{"--continue", "--add-dir", repoRoot, "--append-system-prompt", worktreeHint}
		newArgs = []string{"--add-dir", repoRoot, "--append-system-prompt", worktreeHint}
	case "codex":
		binPath, err = exec.LookPath("codex")
		resumeArgs = []string{"resume", "--last", "--add-dir", repoRoot}
		newArgs = []string{"--add-dir", repoRoot}
	default:
		err = fmt.Errorf("unknown agent: %s", agent)
	}
	if err != nil {
		return "", nil, nil, fmt.Errorf("agent '%s' not found on PATH", agent)
	}
	return
}

func spawnAgent(binPath string, args []string, workdir string) error {
	cmd := exec.Command(binPath, args...)
	cmd.Dir = workdir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Forward signals to child, clean up goroutine on exit
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	done := make(chan struct{})
	go func() {
		select {
		case sig := <-sigCh:
			if cmd.Process != nil {
				cmd.Process.Signal(sig)
			}
		case <-done:
		}
	}()

	err := cmd.Run()
	signal.Stop(sigCh)
	close(done)
	return err
}

func printUsage() {
	fmt.Println("codemob — AI agent workspace manager")
	fmt.Println("")
	fmt.Println("Usage: codemob <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  new [name]         Create a new mob and launch agent")
	fmt.Println("  list               List all mobs")
	fmt.Println("  resume [name]      Resume a mob (launch agent in worktree)")
	fmt.Println("  init               Initialize codemob (global + repo setup)")
	fmt.Println("  reinit             Re-run initialization (idempotent)")
	fmt.Println("  remove <name>      Remove a mob")
	fmt.Println("  purge              Remove all mobs")
	fmt.Println("  path [name]        Print worktree path (interactive if no name)")
	fmt.Println("  info               Show diagnostic information")
	fmt.Println("  uninstall          Remove all codemob setup")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --no-launch        Skip launching the agent")
	fmt.Println("  --agent <name>     Override agent (default: from config)")
	fmt.Println("  --force            Force remove")
	fmt.Println("  --help             Show this help")
	fmt.Println("  --version          Show version")
}
