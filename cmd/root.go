package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"text/tabwriter"
	"time"

	gitutil "github.com/codemob-ai/codemob/internal/git"
	"github.com/codemob-ai/codemob/internal/mob"
)

func Execute() error {
	// Clear stale next action on every invocation (except --check-next which reads it)
	if len(os.Args) < 2 || os.Args[1] != "--check-next" {
		if root, err := mob.FindRepoRoot(); err == nil {
			mob.ClearNextAction(root)
		}
	}

	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	// Flags (core workflow)
	case "--new":
		return cmdNew(args)
	case "--list", "--ls":
		return cmdList(args, false)
	case "--list-others":
		return cmdList(args, true)
	case "--resume", "--switch":
		return cmdResume(args)
	case "--check-next":
		return cmdCheckNext(args)

	// Subcommands (management)
	case "init", "reinit":
		return cmdInit(args)
	case "uninstall":
		return cmdUninstall(args)
	case "remove":
		return cmdRemove(args)
	case "clear":
		return cmdClear(args)

	// Internal
	case "new":
		return cmdNew(args)
	case "list":
		return cmdList(args, false)
	case "resolve":
		return cmdResolve(args)
	case "write-next":
		return cmdWriteNext(args)

	case "--version", "-v", "version":
		fmt.Println("codemob v0.1.0")
		return nil
	case "--help", "-h", "help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s. Run 'codemob --help' for usage.", cmd)
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

func cmdInit(_ []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine install directory: %w", err)
	}
	installDir := filepath.Dir(exe)
	return mob.Init(installDir)
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
	if mob.Reconcile(root, cfg) {
		_ = mob.SaveConfig(root, cfg)
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
			if i+1 < len(args) {
				agent = args[i+1]
				i++
			}
		default:
			if name == "" {
				name = args[i]
			}
		}
	}

	if name == "" {
		name = mob.GenerateName()
	}

	if m := mob.FindMob(cfg, name); m != nil {
		return fmt.Errorf("mob '%s' already exists", name)
	}

	branch := "mob/" + name
	worktreePath := filepath.Join(root, mob.MobsDir, name)

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

	fmt.Printf("Created mob '%s' on branch %s\n", name, branch)

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
		fmt.Println("No mobs. Create one with: codemob --new <name>")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "#\tNAME\tBRANCH\tAGENT\tCREATED")
	for i, m := range mobs {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i+1, m.Name, m.Branch, m.Agent, mob.RelativeTime(m.CreatedAt))
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
		switch arg {
		case "--no-launch":
			noLaunch = true
		default:
			if name == "" {
				name = arg
			}
		}
	}

	if name == "" {
		return fmt.Errorf("mob name required")
	}

	m := mob.FindMob(cfg, name)
	if m == nil {
		return fmt.Errorf("mob '%s' not found", name)
	}

	worktreePath := filepath.Join(root, mob.MobsDir, m.Name)
	fmt.Printf("Resuming mob '%s'\n", m.Name)

	if !noLaunch {
		return launchAgent(root, m.Agent, worktreePath, true)
	}
	return nil
}

// cmdResolve is the internal command used by the shell wrapper.
// Outputs KEY=VALUE lines for the shell to parse.
func cmdResolve(args []string) error {
	root, cfg, err := requireInit()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		return fmt.Errorf("mob name required")
	}
	name := args[0]

	m := mob.FindMob(cfg, name)
	if m == nil {
		return fmt.Errorf("mob '%s' not found", name)
	}

	worktreePath := filepath.Join(root, mob.MobsDir, m.Name)
	fmt.Printf("CODEMOB_PATH=%s\n", worktreePath)
	fmt.Printf("CODEMOB_AGENT=%s\n", m.Agent)
	fmt.Printf("CODEMOB_NAME=%s\n", m.Name)
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

	m := mob.FindMob(cfg, name)
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
		if existing.Name != name {
			remaining = append(remaining, existing)
		}
	}
	cfg.Mobs = remaining

	if err := mob.SaveConfig(root, cfg); err != nil {
		return err
	}

	fmt.Printf("Removed mob '%s'\n", name)
	return nil
}

func cmdClear(_ []string) error {
	root, cfg, err := requireInit()
	if err != nil {
		return err
	}

	if len(cfg.Mobs) == 0 {
		fmt.Println("No mobs to clear.")
		return nil
	}

	fmt.Printf("This will remove all %d mob(s) and their worktrees.\n", len(cfg.Mobs))
	fmt.Print("Are you sure? [y/N]: ")

	var input string
	fmt.Scanln(&input)
	if input != "y" && input != "yes" {
		fmt.Println("Cancelled.")
		return nil
	}

	for _, m := range cfg.Mobs {
		worktreePath := filepath.Join(root, mob.MobsDir, m.Name)
		if _, err := os.Stat(worktreePath); err == nil {
			_ = gitutil.WorktreeRemove(root, worktreePath, true)
		}
		_ = gitutil.BranchDelete(root, m.Branch)
		fmt.Printf("  Removed '%s'\n", m.Name)
	}

	cfg.Mobs = nil
	if err := mob.SaveConfig(root, cfg); err != nil {
		return err
	}

	fmt.Println("All mobs cleared.")
	return nil
}

func cmdCheckNext(_ []string) error {
	root, err := mob.FindRepoRoot()
	if err != nil {
		return nil // not in a repo, nothing to do
	}

	next, err := mob.ReadNextAction(root)
	if err != nil || next == nil {
		return nil // no queued action
	}
	mob.ClearNextAction(root)

	switch next.Action {
	case "switch":
		cfg, err := mob.LoadConfig(root)
		if err != nil {
			return err
		}
		m := mob.FindMob(cfg, next.Target)
		if m == nil {
			return fmt.Errorf("mob '%s' not found", next.Target)
		}
		worktreePath := filepath.Join(root, mob.MobsDir, m.Name)
		fmt.Printf("Switching to mob '%s'\n", m.Name)
		return launchAgent(root, m.Agent, worktreePath, true)
	default:
		return fmt.Errorf("unknown next action: %s", next.Action)
	}
}

func cmdDetectBranch(_ []string) error {
	root, err := mob.FindRepoRoot()
	if err != nil {
		root, _ = gitutil.RepoRoot()
	}
	if root == "" {
		fmt.Println("main")
		return nil
	}
	fmt.Println(gitutil.DetectDefaultBranch(root))
	return nil
}

// cmdWriteNext writes a next action for the trampoline.
// Used by slash commands: codemob write-next switch <mob-name>
func cmdWriteNext(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: codemob write-next <action> <target>")
	}
	root, err := mob.FindRepoRoot()
	if err != nil {
		return err
	}
	return mob.WriteNextAction(root, mob.NextAction{
		Action: args[0],
		Target: args[1],
	})
}

// launchAgent spawns the agent as a child process and implements the trampoline loop.
// After the agent exits, it checks for a next action (e.g., switch to another mob).
func launchAgent(root, agent, workdir string, resume bool) error {
	for {
		if err := runAgent(agent, workdir, resume); err != nil {
			return err
		}

		// Check for next action
		next, err := mob.ReadNextAction(root)
		if err != nil || next == nil {
			return nil // normal exit
		}
		mob.ClearNextAction(root)

		switch next.Action {
		case "switch":
			cfg, err := mob.LoadConfig(root)
			if err != nil {
				return err
			}
			m := mob.FindMob(cfg, next.Target)
			if m == nil {
				return fmt.Errorf("mob '%s' not found", next.Target)
			}
			workdir = filepath.Join(root, mob.MobsDir, m.Name)
			agent = m.Agent
			resume = true
			fmt.Printf("Switching to mob '%s'\n", m.Name)
		default:
			return fmt.Errorf("unknown next action: %s", next.Action)
		}
	}
}

// runAgent spawns the agent process and waits for it to exit.
// If resume is true and the agent fails (e.g., no session to continue), falls back to a new session.
func runAgent(agent, workdir string, resume bool) error {
	binPath, resumeArgs, newArgs, err := agentArgs(agent)
	if err != nil {
		return err
	}

	if resume {
		err := spawnAgent(binPath, resumeArgs, workdir)
		if err == nil {
			return nil
		}
		// Resume failed (no prior session) — fall back to new session
		fmt.Println("No previous session found, starting new session...")
	}

	return spawnAgent(binPath, newArgs, workdir)
}

func agentArgs(agent string) (binPath string, resumeArgs, newArgs []string, err error) {
	switch agent {
	case "claude":
		binPath, err = exec.LookPath("claude")
		resumeArgs = []string{"--continue"}
		newArgs = []string{}
	case "codex":
		binPath, err = exec.LookPath("codex")
		resumeArgs = []string{"resume", "--last"}
		newArgs = []string{}
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

	// Forward signals to child
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		for sig := range sigCh {
			if cmd.Process != nil {
				cmd.Process.Signal(sig)
			}
		}
	}()
	defer signal.Stop(sigCh)

	return cmd.Run()
}

func printUsage() {
	fmt.Println("codemob — AI agent workspace manager")
	fmt.Println("")
	fmt.Println("Usage: codemob <command>")
	fmt.Println("")
	fmt.Println("Workflow:")
	fmt.Println("  --new [name]       Create a new mob and launch agent")
	fmt.Println("  --list             List all mobs")
	fmt.Println("  --resume <name>    Resume a mob (launch agent in worktree)")
	fmt.Println("  --switch <name>    Alias for --resume")
	fmt.Println("")
	fmt.Println("Management:")
	fmt.Println("  init               Initialize codemob (global + repo setup)")
	fmt.Println("  reinit             Re-run initialization (idempotent)")
	fmt.Println("  remove <name>      Remove a mob")
	fmt.Println("  clear              Remove all mobs")
	fmt.Println("  uninstall          Remove all codemob setup")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --no-launch        Skip launching the agent")
	fmt.Println("  --agent <name>     Override agent (default: from config)")
	fmt.Println("  --force            Force remove")
	fmt.Println("  --help             Show this help")
	fmt.Println("  --version          Show version")
}
