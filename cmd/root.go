package cmd

import (
	"fmt"
	"os"
	"path/filepath"
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
	case "new":
		return cmdNew(args)
	case "list":
		return cmdList(args)
	case "resolve":
		return cmdResolve(args)
	case "remove":
		return cmdRemove(args)
	case "detect-branch":
		return cmdDetectBranch(args)
	case "version":
		fmt.Println("codemob-core v0.1.0")
		return nil
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
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

	// Output for codemob.sh
	fmt.Printf("CODEMOB_PATH=%s\n", worktreePath)
	fmt.Printf("CODEMOB_AGENT=%s\n", agent)
	fmt.Printf("CODEMOB_NAME=%s\n", name)
	fmt.Printf("CODEMOB_BRANCH=%s\n", branch)
	fmt.Printf("CODEMOB_NO_LAUNCH=%t\n", noLaunch)
	return nil
}

func cmdList(_ []string) error {
	root, cfg, err := requireInit()
	if err != nil {
		return err
	}
	_ = root

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

	// Remove from config
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

func printUsage() {
	fmt.Println("codemob-core — internal logic for codemob")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  new [name] [--no-launch] [--agent <name>]")
	fmt.Println("  list")
	fmt.Println("  resolve <name>")
	fmt.Println("  remove <name> [--force]")
	fmt.Println("  detect-branch")
	fmt.Println("  version")
}
