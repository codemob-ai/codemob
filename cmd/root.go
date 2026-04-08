package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	gitutil "github.com/codemob-ai/codemob/internal/git"
	"github.com/codemob-ai/codemob/internal/mob"
)

func brandPrefix(color string) string {
	if color == "" {
		color = "\033[38;2;231;220;96m"
	}
	return fmt.Sprintf("  %s【●】codemob\033[0m  ", color)
}

func mobStatus(msg string) {
	fmt.Println()
	fmt.Printf("%s%s\n", brandPrefix(""), msg)
	fmt.Println()
}

type progress struct{ active bool }

func mobProgress(msg string) *progress {
	fmt.Printf("\n%s%s", brandPrefix(""), msg)
	return &progress{active: true}
}

func (p *progress) Done(msg string) {
	if !p.active {
		return
	}
	p.active = false
	fmt.Printf("\r\033[2K%s%s\n\n", brandPrefix(""), msg)
}

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
	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	// Reject unknown commands before any side effects and decide upgrade check.
	// Keep in sync with the dispatch switch below (see CLAUDE.md).
	switch cmd {
	case "new", "list", "ls", "resume", "remove", "purge", "open", "info":
		repoRoot := ""
		if root, err := mob.FindRepoRoot(); err == nil {
			repoRoot = root
		}
		mob.CheckUpgrade(Version, repoRoot)
	case "switch", "list-others", "check-queue", "queue", "clear-queue", "inject-args", "path",
		"init", "reinit", "uninstall", "version", "--version", "-v", "help", "--help", "-h":
		// internal/setup commands: skip upgrade check
	default:
		return fmt.Errorf("unknown command: %s. Run 'codemob help' for usage.", cmd)
	}

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
	case "open":
		return cmdOpen(args)
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
	case "clear-queue":
		return cmdClearQueue(args)
	case "inject-args":
		return cmdInjectArgs(args)

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
	if idx, err := strconv.Atoi(nameOrIndex); err == nil {
		if idx >= 1 && idx <= len(cfg.Mobs) {
			return &cfg.Mobs[idx-1]
		}
	}
	return mob.FindMob(cfg, nameOrIndex)
}

type pickerOpts struct {
	repoRoot   string   // repo root for resolving actual worktree branches
	out        *os.File // output for table and prompt (default: os.Stdout)
	markerName string   // mob name to mark with ◀ (e.g., last session mob)
	defaultVal string   // pre-filled default shown in prompt bracket; enter selects it
	showRoot   bool     // show "0 — repo root" hint (for cd/path)
	useSession bool     // auto-populate markerName/defaultVal from session's last mob
}

func pickMob(cfg *mob.Config, opts pickerOpts) (string, error) {
	if opts.useSession && opts.repoRoot != "" {
		lastMob := readLastMob(opts.repoRoot)
		if lastMob != "" && mob.FindMob(cfg, lastMob) == nil {
			lastMob = ""
		}
		if lastMob != "" {
			if opts.markerName == "" {
				opts.markerName = lastMob
			}
			if opts.defaultVal == "" {
				opts.defaultVal = lastMob
			}
		}
	}

	if len(cfg.Mobs) == 0 {
		return "", fmt.Errorf("no mobs. Create one with: codemob new")
	}
	if len(cfg.Mobs) == 1 && opts.defaultVal == "" && !opts.showRoot {
		return cfg.Mobs[0].Name, nil
	}

	out := opts.out
	if out == nil {
		out = os.Stdout
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "#\tNAME\tBRANCH\tLAST AGENT\tCREATED")
	for i, m := range cfg.Mobs {
		marker := ""
		if m.Name == opts.markerName {
			marker = " ◀"
		}
		branch := mob.ActualBranch(opts.repoRoot, cfg, &m)
		fmt.Fprintf(w, "%d\t%s%s\t%s\t%s\t%s\n", i+1, m.Name, marker, branch, m.Agent, mob.RelativeTime(m.CreatedAt))
	}
	w.Flush()

	if opts.showRoot {
		fmt.Fprintf(out, "\n  \033[38;2;100;180;220m> enter 0 to cd back to repo root\033[0m\n")
	}

	if opts.defaultVal != "" {
		fmt.Fprintf(out, "\nWhich mob? (#/name) [%s]: ", opts.defaultVal)
	} else {
		fmt.Fprint(out, "\nWhich mob? (#/name): ")
	}

	var name string
	fmt.Scanln(&name)
	if name == "" {
		name = opts.defaultVal
	}
	if name == "" {
		return "", fmt.Errorf("no mob selected")
	}
	return name, nil
}

func cmdInit(_ []string, forceReprompt bool) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine install directory: %w", err)
	}
	installDir := filepath.Dir(exe)
	if err := mob.Init(installDir, forceReprompt); err != nil {
		return err
	}
	mob.WriteVersion(Version)
	return nil
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
	if cfg.RepoRoot != "" {
		resolvedCfgRoot, _ := filepath.EvalSymlinks(cfg.RepoRoot)
		resolvedRoot, _ := filepath.EvalSymlinks(root)
		if resolvedCfgRoot == "" {
			resolvedCfgRoot = cfg.RepoRoot
		}
		if resolvedRoot == "" {
			resolvedRoot = root
		}
		if resolvedCfgRoot != resolvedRoot {
			return "", nil, fmt.Errorf("repo has moved (was %s). Run 'codemob reinit' to update the config", cfg.RepoRoot)
		}
	}
	if cfg.MobsDirPath != "" {
		if _, err := os.Stat(cfg.MobsDirPath); err != nil {
			if len(cfg.Mobs) > 0 {
				return "", nil, fmt.Errorf("mobs directory %s no longer exists. Run 'codemob reinit' to fix", cfg.MobsDirPath)
			}
			if err := os.MkdirAll(cfg.MobsDirPath, 0755); err != nil {
				return "", nil, fmt.Errorf("failed to recreate mobs directory %s: %w", cfg.MobsDirPath, err)
			}
		}
	}
	if removed := mob.Reconcile(root, cfg); len(removed) > 0 {
		_ = mob.SaveConfig(root, cfg)
		cleanSessionFiles(root, removed...)
	}
	return root, cfg, nil
}

// createMob handles the full mob-creation sequence: name validation/generation,
// worktree creation, config update, and save. Returns the worktree path.
func createMob(root string, cfg *mob.Config, name, agent string) (string, error) {
	if name == "" {
		name = mob.GenerateUniqueName(cfg)
	} else {
		if err := mob.ValidateName(name); err != nil {
			return "", err
		}
		if m := mob.FindMob(cfg, name); m != nil {
			return "", fmt.Errorf("mob '%s' already exists", name)
		}
	}

	branch := "mob/" + name
	if gitutil.BranchExists(root, branch) {
		hint := fmt.Sprintf("git branch -D %s", branch)
		if wts, err := gitutil.WorktreeList(root); err == nil {
			for _, wt := range wts {
				if wt.Branch == "refs/heads/"+branch {
					hint = fmt.Sprintf("git worktree remove %s && git branch -D %s", wt.Path, branch)
					break
				}
			}
		}
		return "", fmt.Errorf("branch '%s' already exists (leftover from a previous mob?). Run:\n  %s", branch, hint)
	}
	worktreePath := mob.MobPath(root, cfg, name)

	p := mobProgress(fmt.Sprintf("Creating mob '%s'...", name))
	defer p.Clear()

	if err := gitutil.WorktreeAdd(root, worktreePath, branch, cfg.BaseBranch); err != nil {
		return "", err
	}

	mob.CopyClaudeSlashCommands(root, worktreePath)

	cfg.Mobs = append(cfg.Mobs, mob.Mob{
		Name:      name,
		Branch:    branch,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Agent:     agent,
	})

	if err := mob.SaveConfig(root, cfg); err != nil {
		return "", err
	}

	p.Done(fmt.Sprintf("Created mob '%s' on branch %s", name, branch))
	return worktreePath, nil
}

func runPostCreateScript(cfg *mob.Config, worktreePath string) error {
	if cfg.PostCreateScript == "" {
		return nil
	}
	dim := "\033[2m"
	reset := "\033[0m"
	mobStatus("Running post-create script...")
	fmt.Printf("  %s╭─%s\n", dim, reset)
	err := mob.RunPostCreateScript(cfg, worktreePath)
	fmt.Printf("  %s╰─%s\n", dim, reset)
	if err != nil {
		mobStatus("Post-create script failed")
		return err
	}
	mobStatus("Post-create script completed")
	return nil
}

func confirmRemove(name string) bool {
	r := mob.ColorRed
	rst := mob.ColorReset
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %s⚠ DESTRUCTIVE OPERATION%s\n", r, rst)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  This will permanently delete mob '%s'.\n", name)
	fmt.Fprintf(os.Stderr, "  Any %suncommitted or unpushed changes%s will be %spermanently lost%s.\n", r, rst, r, rst)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %sThis cannot be undone.%s\n", r, rst)
	fmt.Fprint(os.Stderr, "\n  Are you sure? [y/N]: ")
	var input string
	fmt.Scanln(&input)
	return input == "y" || input == "Y" || input == "yes"
}

func removeMob(root string, cfg *mob.Config, m *mob.Mob, force bool) error {
	worktreePath := mob.MobPath(root, cfg, m.Name)
	if _, err := os.Stat(worktreePath); err == nil {
		if err := gitutil.WorktreeRemove(root, worktreePath, force); err != nil {
			return err
		}
	}

	gitutil.BranchDelete(root, m.Branch)

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
	return nil
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
		switch {
		case args[i] == "--no-launch":
			noLaunch = true
		case args[i] == "--agent":
			if i+1 >= len(args) {
				return fmt.Errorf("--agent requires a value (e.g., --agent codex)")
			}
			agent = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--"):
			return fmt.Errorf("unknown flag for new: %s", args[i])
		default:
			if name == "" {
				name = args[i]
			}
		}
	}

	worktreePath, err := createMob(root, cfg, name, agent)
	if err != nil {
		return err
	}

	if err := runPostCreateScript(cfg, worktreePath); err != nil {
		if m := mob.FindMob(cfg, filepath.Base(worktreePath)); m != nil {
			_ = removeMob(root, cfg, m, true)
		}
		return err
	}

	if !noLaunch {
		return launchAgent(root, agent, worktreePath, false)
	}
	return nil
}

func cmdList(_ []string, excludeCurrent bool) error {
	root, cfg, err := requireInit()
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
		branch := mob.ActualBranch(root, cfg, &m)
		fmt.Fprintf(w, "%d\t%s%s\t%s\t%s\t%s\n", i+1, m.Name, marker, branch, m.Agent, mob.RelativeTime(m.CreatedAt))
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
		var err error
		name, err = pickMob(cfg, pickerOpts{
			repoRoot:   root,
			useSession: true,
		})
		if err != nil {
			return err
		}
	}

	m := resolveMob(cfg, name)
	if m == nil {
		return fmt.Errorf("mob '%s' not found", name)
	}

	worktreePath := mob.MobPath(root, cfg, m.Name)
	mobStatus(fmt.Sprintf("Resuming mob '%s'", m.Name))

	if !noLaunch {
		return launchAgent(root, m.Agent, worktreePath, true)
	}
	return nil
}

func cmdOpen(args []string) error {
	root, cfg, err := requireInit()
	if err != nil {
		return err
	}

	name := ""
	agent := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--agent":
			if i+1 >= len(args) {
				return fmt.Errorf("--agent requires a value (e.g., --agent codex)")
			}
			agent = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--"):
			return fmt.Errorf("unknown flag for open: %s", args[i])
		default:
			if name == "" {
				name = args[i]
			}
		}
	}

	if name == "" {
		var err error
		name, err = pickMob(cfg, pickerOpts{
			repoRoot:   root,
			useSession: true,
		})
		if err != nil {
			return err
		}
	}

	m := resolveMob(cfg, name)
	if m == nil {
		return fmt.Errorf("mob '%s' not found", name)
	}

	if agent == "" {
		agent = m.Agent
	} else if agent != m.Agent {
		m.Agent = agent
		_ = mob.SaveConfig(root, cfg)
	}

	worktreePath := mob.MobPath(root, cfg, m.Name)
	mobStatus(fmt.Sprintf("Opening mob '%s' (fresh session)", m.Name))

	return launchAgent(root, agent, worktreePath, false)
}

func cmdRemove(args []string) error {
	root, cfg, err := requireInit()
	if err != nil {
		return err
	}

	name := ""
	force := false
	for _, arg := range args {
		switch {
		case arg == "--force" || arg == "-f":
			force = true
		case strings.HasPrefix(arg, "--"):
			return fmt.Errorf("unknown flag for remove: %s", arg)
		default:
			if name == "" {
				name = arg
			}
		}
	}

	if name == "" {
		picked, err := pickMob(cfg, pickerOpts{repoRoot: root})
		if err != nil {
			return err
		}
		name = picked
	}

	m := resolveMob(cfg, name)
	if m == nil {
		return fmt.Errorf("mob '%s' not found", name)
	}

	if !force {
		if !confirmRemove(m.Name) {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if err := removeMob(root, cfg, m, force); err != nil {
		return err
	}

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
	fmt.Println("  The following mobs will be permanently removed:")
	fmt.Println()
	for _, m := range cfg.Mobs {
		branch := mob.ActualBranch(root, cfg, &m)
		fmt.Printf("    %s✗%s %s  (branch: %s)\n", r, rst, m.Name, branch)
	}
	fmt.Println()
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
		worktreePath := mob.MobPath(root, cfg, m.Name)
		if _, err := os.Stat(worktreePath); err == nil {
			_ = gitutil.WorktreeRemove(root, worktreePath, true)
		}
		gitutil.BranchDelete(root, m.Branch)
		fmt.Printf("  %s✗%s Removed '%s'\n", r, rst, m.Name)
	}

	mob.CleanMobsDirContents(cfg.MobsDirPath)

	cfg.Mobs = nil
	if err := mob.SaveConfig(root, cfg); err != nil {
		return err
	}

	os.RemoveAll(filepath.Join(root, mob.CodemobDir, "sessions"))

	fmt.Println()
	fmt.Printf("%sAll mobs purged\n", brandPrefix(r))
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

	if name == "" {
		inMob := mob.CurrentMobName() != ""
		var err error
		name, err = pickMob(cfg, pickerOpts{
			repoRoot:   root,
			out:        os.Stderr,
			showRoot:   inMob,
			useSession: true,
		})
		if err != nil {
			return err
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

	fmt.Println(mob.MobPath(root, cfg, m.Name))
	return nil
}

func cmdCheckNext(_ []string) error {
	root, err := mob.FindRepoRoot()
	if err != nil {
		return nil // not in a repo, nothing to do
	}
	sessionID, err := mob.QueueSessionID()
	if err != nil {
		return nil // no session queue available, nothing to do
	}

	next, err := mob.ReadQueuedAction(root, sessionID)
	if err != nil || next == nil {
		return nil // no queued action
	}
	mob.ClearQueue(root, sessionID)

	return executeNextAction(root, next)
}

func cmdClearQueue(args []string) error {
	if len(args) > 0 && strings.HasPrefix(args[0], "--") {
		return fmt.Errorf("unknown flag for clear-queue: %s", args[0])
	}
	if len(args) > 0 {
		return fmt.Errorf("usage: codemob clear-queue")
	}

	root, err := mob.FindRepoRoot()
	if err != nil {
		return nil
	}
	sessionID, err := mob.QueueSessionID()
	if err != nil {
		return err
	}

	mob.ClearQueue(root, sessionID)
	return nil
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
		return mob.MobPath(root, cfg, m.Name), m.Agent, true, nil

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
		return mob.MobPath(root, cfg, m.Name), newAgent, false, nil

	case "new":
		agent := next.Agent
		if agent == "" {
			agent = cfg.DefaultAgent
		}
		worktreePath, err := createMob(root, cfg, next.Target, agent)
		if err != nil {
			return "", "", false, err
		}
		if err := runPostCreateScript(cfg, worktreePath); err != nil {
			if m := mob.FindMob(cfg, filepath.Base(worktreePath)); m != nil {
				_ = removeMob(root, cfg, m, true)
			}
			return "", "", false, err
		}
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
		if !confirmRemove(m.Name) {
			fmt.Println("  Cancelled.")
			return "", "", false, nil
		}
		if err := removeMob(root, cfg, m, true); err != nil {
			return "", "", false, err
		}
		mobStatus(fmt.Sprintf("Removed mob '%s'", m.Name))
		return "", "", false, nil

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

	if target == "@self" {
		target = mob.CurrentMobName()
		if target == "" {
			return fmt.Errorf("@self: not inside a mob")
		}
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

	currentMob := mob.CurrentMobName()
	if currentMob == "" {
		return fmt.Errorf("codemob queue must be run from inside a mob")
	}
	sessionID, err := mob.QueueSessionID()
	if err != nil {
		return err
	}
	return mob.WriteQueuedAction(root, sessionID, q)
}

// launchAgent spawns the agent as a child process and implements the trampoline loop.
// After the agent exits, it checks for a next action (e.g., switch to another mob).
// On final exit, writes the last active mob name to .codemob/sessions/<session-id>
// (keyed by $CODEMOB_SESSION) so resume can default to it.
func launchAgent(root, agent, workdir string, resume bool) error {
	sessionID, err := mob.QueueSessionID()
	if err != nil {
		sessionID = ""
	}

	for {
		// Drop any leftover queue file for the session we're about to launch so a
		// stale action from an earlier run cannot immediately terminate a fresh one.
		if sessionID != "" && root != "" && filepath.IsAbs(root) {
			mob.ClearQueue(root, sessionID)
		}

		if err := runAgent(root, sessionID, agent, workdir, resume); err != nil {
			// Log non-signal errors (signal exits are normal — user pressed Ctrl+C)
			if _, ok := err.(*exec.ExitError); !ok {
				fmt.Fprintf(os.Stderr, "  [codemob] agent error: %v\n", err)
			}
		}

		fmt.Print("\033[2K")
		mobStatus(fmt.Sprintf("Session ended - mob '%s'", filepath.Base(workdir)))

		// Always check for queued action, regardless of how the agent exited
		if sessionID == "" {
			writeLastMob(workdir)
			return nil // normal exit with no queue session available
		}
		next, err := mob.ReadQueuedAction(root, sessionID)
		if err != nil || next == nil {
			writeLastMob(workdir)
			return nil // normal exit
		}
		mob.ClearQueue(root, sessionID)

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
func runAgent(root, sessionID, agent, workdir string, resume bool) error {
	binPath, resumeArgs, newArgs, err := agentArgs(agent, root)
	if err != nil {
		return err
	}

	if resume {
		err := spawnAgent(root, sessionID, binPath, resumeArgs, workdir)
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

	return spawnAgent(root, sessionID, binPath, newArgs, workdir)
}

func cmdInjectArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: codemob inject-args <agent>")
	}
	agent := args[0]

	repoRoot := mob.InsideWorktree()
	if repoRoot == "" {
		return nil
	}

	mobName := mob.CurrentMobNameForRoot(repoRoot)
	if mobName != "" {
		fmt.Printf("CODEMOB_MOB=%s\n", mobName)
	}

	hint := worktreeHint(repoRoot)

	switch agent {
	case "claude":
		fmt.Println("--add-dir")
		fmt.Println(repoRoot)
		fmt.Println("--append-system-prompt")
		fmt.Println(hint)
	}
	return nil
}

func worktreeHint(repoRoot string) string {
	return "IMPORTANT: You are working inside a codemob worktree. " +
		"This IS a full git repository — all files and history are available here. " +
		"Use your current working directory as the project root. " +
		"Do NOT navigate to " + repoRoot + " — that is the main repo and may be on a different branch with different files. " +
		"When spawning subagents (Explore, Agent, etc.), instruct them to work in the current directory, not " + repoRoot + "."
}

func agentArgs(agent, repoRoot string) (binPath string, resumeArgs, newArgs []string, err error) {
	hint := worktreeHint(repoRoot)

	switch agent {
	case "claude":
		binPath, err = exec.LookPath("claude")
		resumeArgs = []string{"--continue", "--add-dir", repoRoot, "--append-system-prompt", hint}
		newArgs = []string{"--add-dir", repoRoot, "--append-system-prompt", hint}
	case "codex":
		binPath, err = exec.LookPath("codex")
		resumeArgs = []string{"resume", "--last"}
		newArgs = []string{}
	default:
		return "", nil, nil, fmt.Errorf("unknown agent: %s", agent)
	}
	if err != nil {
		return "", nil, nil, fmt.Errorf("agent '%s' not found on PATH", agent)
	}
	return
}

func spawnAgent(root, sessionID, binPath string, args []string, workdir string) error {
	cmd := exec.Command(binPath, args...)
	cmd.Dir = workdir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "CODEMOB_MOB="+filepath.Base(workdir))

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan struct{})

	// Forward signals to child
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-sigCh:
			if cmd.Process != nil {
				cmd.Process.Signal(sig)
			}
		case <-done:
		}
	}()

	// Watch for the current session's queue file - auto-terminate the agent when a
	// queued action appears.
	if sessionID != "" && root != "" && filepath.IsAbs(root) {
		queuePath := mob.QueueFilePath(root, sessionID)
		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if _, err := os.Stat(queuePath); err == nil {
						mobStatus("Stopping agent...")
						if cmd.Process != nil {
							cmd.Process.Signal(syscall.SIGINT)
						}
						return
					}
				case <-done:
					return
				}
			}
		}()
	}

	err := cmd.Wait()
	signal.Stop(sigCh)
	close(done)

	// Drain kitty keyboard protocol stack in case the agent exited without
	// popping it (e.g. SIGTERM during queue-based switching). This is a no-op
	// if the protocol was never pushed.
	fmt.Fprint(os.Stdout, "\033[<10u")

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
	fmt.Println("  resume [name]      Resume a mob (continue previous session)")
	fmt.Println("  open [name]        Open a mob (fresh agent session)")
	fmt.Println("  init               Initialize codemob (global + repo setup)")
	fmt.Println("  reinit             Re-run initialization (idempotent)")
	fmt.Println("  remove <name>      Remove a mob")
	fmt.Println("  purge              Remove all mobs")
	fmt.Println("  info               Show diagnostic information")
	fmt.Println("  uninstall          Remove all codemob setup")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --agent <name>     Override agent (default: from config)")
	fmt.Println("  --force            Force remove")
	fmt.Println("  --help             Show this help")
	fmt.Println("  --version          Show version")
}
