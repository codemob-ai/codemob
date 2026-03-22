package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"text/tabwriter"
	"time"

	gitutil "github.com/codemob-ai/codemob/internal/git"
	"github.com/codemob-ai/codemob/internal/mob"
)

func Execute() error {
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
		return cmdList(args)
	case "--resume", "--switch":
		return cmdResume(args)

	// Subcommands (management)
	case "init", "reinit":
		return cmdInit(args)
	case "uninstall":
		return cmdUninstall(args)
	case "remove":
		return cmdRemove(args)

	// Internal (used by shell wrapper)
	case "new":
		return cmdNew(args)
	case "list":
		return cmdList(args)
	case "resolve":
		return cmdResolve(args)

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
		return launchAgent(agent, worktreePath, false)
	}
	return nil
}

func cmdList(_ []string) error {
	_, cfg, err := requireInit()
	if err != nil {
		return err
	}

	if len(cfg.Mobs) == 0 {
		fmt.Println("No mobs. Create one with: codemob --new <name>")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tBRANCH\tAGENT\tCREATED")
	for _, m := range cfg.Mobs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Name, m.Branch, m.Agent, mob.RelativeTime(m.CreatedAt))
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
		return launchAgent(m.Agent, worktreePath, true)
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

// launchAgent replaces the current process with the agent in the given directory.
func launchAgent(agent, workdir string, resume bool) error {
	var agentBin string
	var agentArgs []string

	switch agent {
	case "claude":
		agentBin = "claude"
		if resume {
			agentArgs = []string{"claude", "--continue"}
		} else {
			agentArgs = []string{"claude"}
		}
	case "codex":
		agentBin = "codex"
		if resume {
			agentArgs = []string{"codex", "resume", "--last"}
		} else {
			agentArgs = []string{"codex"}
		}
	default:
		return fmt.Errorf("unknown agent: %s", agent)
	}

	// Find the agent binary on PATH
	binPath, err := exec.LookPath(agentBin)
	if err != nil {
		return fmt.Errorf("agent '%s' not found on PATH", agentBin)
	}

	// Change to worktree directory and exec the agent
	if err := os.Chdir(workdir); err != nil {
		return fmt.Errorf("could not change to worktree: %w", err)
	}

	return syscall.Exec(binPath, agentArgs, os.Environ())
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
	fmt.Println("  uninstall          Remove all codemob setup")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --no-launch        Skip launching the agent")
	fmt.Println("  --agent <name>     Override agent (default: from config)")
	fmt.Println("  --force            Force remove")
	fmt.Println("  --help             Show this help")
	fmt.Println("  --version          Show version")
}
