package mob

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gitutil "github.com/codemob-ai/codemob/internal/git"
)

// slashCommandDefs defines the slash command content. Each is installed under both
// "mob-*" and "codemob-*" names so either /mob-ls or /codemob-ls works.
var slashCommandDefs = map[string]string{
	"list": "List all codemob workspaces and their status.\n\nRun `codemob --list` using the Bash tool and display the results to the user.\n",
	"new": `Create a new codemob workspace and copy the launch command to clipboard.

Ask the user for a name for the new mob (or suggest generating one automatically).

Once you have the name, copy the following command to the clipboard using ` + "`echo \"codemob --new <name>\" | pbcopy`" + ` (replace ` + "`<name>`" + ` with the actual name).

Then tell the user: "The command has been copied to your clipboard. Exit this session (Ctrl+C) and paste (Cmd+V) to create the new mob."
`,
	"switch": `Switch to a different codemob workspace and copy the launch command to clipboard.

Run ` + "`codemob --list`" + ` using the Bash tool and display the results to the user.

Ask the user which mob they want to switch to.

Once they pick one, copy the following command to the clipboard using ` + "`echo \"codemob --switch <name>\" | pbcopy`" + ` (replace ` + "`<name>`" + ` with the chosen mob name).

Then tell the user: "The command has been copied to your clipboard. Exit this session (Ctrl+C) and paste (Cmd+V) to switch to the mob."
`,
	"remove": `Remove a codemob workspace (worktree + branch).

First, check if the current working directory is inside a codemob mob worktree by checking if the path contains ` + "`.codemob/mobs/`" + `. If it does, extract the mob name from the path and ask the user to confirm removing THIS mob (the one they're currently working in).

If not inside a mob worktree, run ` + "`codemob --list`" + ` using the Bash tool, display the results, and ask which mob to remove.

Once confirmed, run ` + "`codemob remove <name>`" + ` using the Bash tool and display the result.
`,
}

// SlashCommands returns the full map of filename → content, with both mob-* and codemob-* variants.
func SlashCommands() map[string]string {
	cmds := make(map[string]string)
	for name, content := range slashCommandDefs {
		cmds["mob-"+name+".md"] = content
		cmds["codemob-"+name+".md"] = content
	}
	return cmds
}

const (
	green  = "\033[0;32m"
	yellow = "\033[0;33m"
	red    = "\033[0;31m"
	reset  = "\033[0m"
)

func info(msg string)  { fmt.Printf("%s✓%s %s\n", green, reset, msg) }
func warn(msg string)  { fmt.Printf("%s!%s %s\n", yellow, reset, msg) }
func errMsg(msg string) { fmt.Fprintf(os.Stderr, "%s✗%s %s\n", red, reset, msg) }

// Init performs the full codemob initialization.
// installDir is the directory where codemob-shell.sh lives.
func Init(installDir string) error {
	fmt.Println("codemob init")
	fmt.Println("────────────")
	fmt.Println()

	fmt.Println("Global setup:")
	if err := checkDependencies(); err != nil {
		return err
	}
	setupGlobalGitignore()
	setupShellIntegration(installDir)

	fmt.Println()
	fmt.Println("Repo setup:")
	repoRoot := setupRepo()
	setupClaudeCommands(repoRoot)

	rcFile, rcName := detectShellRC()
	fmt.Println()
	fmt.Println("────────────────────────────────────────────────────────")
	warn("codemob won't work until you reload your shell!")
	fmt.Println()
	fmt.Println("  Either open a new terminal, or run:")
	fmt.Printf("  source %s\n", rcName)
	fmt.Println("────────────────────────────────────────────────────────")
	_ = rcFile
	return nil
}

func checkDependencies() error {
	if _, err := exec.LookPath("git"); err != nil {
		errMsg("git is not installed. codemob requires git.")
		return fmt.Errorf("git not found")
	}
	return nil
}

func setupGlobalGitignore() {
	// Find the global gitignore file
	gitignoreFile := ""
	out, err := exec.Command("git", "config", "--global", "core.excludesFile").Output()
	if err == nil {
		gitignoreFile = strings.TrimSpace(string(out))
	}

	if gitignoreFile == "" {
		gitignoreFile = filepath.Join(os.Getenv("HOME"), ".config", "git", "ignore")
	} else {
		// Expand ~ if present
		if strings.HasPrefix(gitignoreFile, "~") {
			gitignoreFile = filepath.Join(os.Getenv("HOME"), gitignoreFile[1:])
		}
	}

	// Ensure parent dir exists
	os.MkdirAll(filepath.Dir(gitignoreFile), 0755)

	// Check if already set up
	if fileContains(gitignoreFile, ".codemob/") && fileContains(gitignoreFile, "mob-*.md") {
		info("Global gitignore already configured for codemob")
		return
	}

	// Append
	f, err := os.OpenFile(gitignoreFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		warn(fmt.Sprintf("Could not write to %s: %v", gitignoreFile, err))
		return
	}
	defer f.Close()

	f.WriteString("\n# codemob\n.codemob/\n.claude/commands/mob-*.md\n.claude/commands/codemob-*.md\n")
	info(fmt.Sprintf("Added codemob entries to global gitignore (%s)", gitignoreFile))
}

func detectShellRC() (string, string) {
	shell := os.Getenv("SHELL")
	home := os.Getenv("HOME")

	switch {
	case strings.HasSuffix(shell, "/zsh"):
		return filepath.Join(home, ".zshrc"), "~/.zshrc"
	case strings.HasSuffix(shell, "/bash"):
		// Prefer .bashrc, fall back to .bash_profile
		bashrc := filepath.Join(home, ".bashrc")
		if _, err := os.Stat(bashrc); err == nil {
			return bashrc, "~/.bashrc"
		}
		return filepath.Join(home, ".bash_profile"), "~/.bash_profile"
	default:
		// Default to .profile
		return filepath.Join(home, ".profile"), "~/.profile"
	}
}

func setupShellIntegration(installDir string) {
	rcFile, rcName := detectShellRC()
	sourceLine := fmt.Sprintf(`source "%s/codemob-shell.sh"`, installDir)

	// Check if any codemob source line exists
	if fileContains(rcFile, "codemob-shell.sh") {
		existing := fileLineContaining(rcFile, "codemob-shell.sh")
		if existing == sourceLine {
			info(fmt.Sprintf("Shell integration already configured in %s", rcName))
			return
		}
		replaceLineInFile(rcFile, "codemob-shell.sh", sourceLine)
		info(fmt.Sprintf("Updated codemob source path in %s", rcName))
		return
	}

	// Append
	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		warn(fmt.Sprintf("Could not write to %s: %v", rcName, err))
		return
	}
	defer f.Close()

	f.WriteString("\n# codemob - AI agent workspace manager\n")
	f.WriteString(sourceLine + "\n")
	info(fmt.Sprintf("Added shell integration to %s", rcName))
}

func setupClaudeCommands(repoRoot string) {
	if repoRoot == "" {
		warn("Not inside a git repository. Skipping Claude commands setup.")
		return
	}
	commandsDir := filepath.Join(repoRoot, ".claude", "commands")
	os.MkdirAll(commandsDir, 0755)

	installed := 0
	for name, content := range SlashCommands() {
		dest := filepath.Join(commandsDir, name)
		// Check if file exists and has same content
		existing, err := os.ReadFile(dest)
		if err == nil && string(existing) == content {
			continue
		}
		if err := os.WriteFile(dest, []byte(content), 0644); err != nil {
			warn(fmt.Sprintf("Could not write %s: %v", dest, err))
			continue
		}
		installed++
	}

	if installed > 0 {
		info("Installed Claude slash commands")
	} else {
		info("Claude slash commands are up to date")
	}
}

func setupRepo() string {
	root, err := gitutil.RepoRoot()
	if err != nil {
		warn("Not inside a git repository. Skipping repo setup.")
		warn("Run 'codemob init' again from inside a git repo to set up a project.")
		return ""
	}

	codemobDir := filepath.Join(root, CodemobDir)
	configFile := filepath.Join(root, ConfigFile)

	// Create directories
	os.MkdirAll(filepath.Join(root, MobsDir), 0755)

	if _, err := os.Stat(configFile); err == nil {
		info(fmt.Sprintf("Repo already initialized at %s", root))
		return root
	}

	// Detect base branch
	defaultBranch := gitutil.DetectDefaultBranch(root)

	// Prompt
	fmt.Println()
	fmt.Printf("Base branch for new mobs [%s]: ", defaultBranch)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		defaultBranch = input
	}

	// Create config
	cfg := Config{
		DefaultAgent: "claude",
		BaseBranch:   defaultBranch,
		Mobs:         []Mob{},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configFile, append(data, '\n'), 0644)

	_ = codemobDir
	info(fmt.Sprintf("Created %s (base_branch: %s)", configFile, defaultBranch))
	return root
}

// ─── Uninstall ────────────────────────────────────────────────────────────────

// Uninstall removes all codemob setup — global and local.
func Uninstall(installDir string) error {
	fmt.Println("codemob uninstall")
	fmt.Println("─────────────────")
	fmt.Println()
	warn("This will:")
	fmt.Println("  - Remove shell integration (codemob/mob/claude functions) from your shell RC file")
	fmt.Println("  - Remove codemob entries from global gitignore")

	// Check if we're in a codemob project
	repoRoot, _ := gitutil.RepoRoot()
	hasProject := repoRoot != "" && IsInitialized(repoRoot)
	if hasProject {
		fmt.Printf("  - Remove .codemob/ directory and all worktrees from %s\n", repoRoot)
		fmt.Println("  - Remove codemob slash commands from .claude/commands/")
	}

	fmt.Println()
	warn("codemob will stop working in ALL projects after this.")
	fmt.Println()
	fmt.Print("Are you sure? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input != "y" && input != "yes" {
		fmt.Println("Cancelled.")
		return nil
	}

	fmt.Println()

	// Remove local project data
	if hasProject {
		// Remove all worktrees first (git worktree remove)
		cfg, err := LoadConfig(repoRoot)
		if err == nil {
			for _, m := range cfg.Mobs {
				worktreePath := filepath.Join(repoRoot, MobsDir, m.Name)
				_ = gitutil.WorktreeRemove(repoRoot, worktreePath, true)
				_ = gitutil.BranchDelete(repoRoot, m.Branch)
			}
		}
		// Remove .codemob/ directory
		os.RemoveAll(filepath.Join(repoRoot, CodemobDir))
		info("Removed .codemob/ and all worktrees")

		// Remove slash commands from project
		for name := range SlashCommands() {
			os.Remove(filepath.Join(repoRoot, ".claude", "commands", name))
		}
		info("Removed codemob slash commands from .claude/commands/")
	}

	// Remove shell integration
	rcFile, rcName := detectShellRC()
	if removeLinesFromFile(rcFile, "codemob") {
		info(fmt.Sprintf("Removed codemob lines from %s", rcName))
	} else {
		info(fmt.Sprintf("No codemob lines found in %s", rcName))
	}

	// Remove from global gitignore
	gitignoreFile := ""
	out, err := exec.Command("git", "config", "--global", "core.excludesFile").Output()
	if err == nil {
		gitignoreFile = strings.TrimSpace(string(out))
	}
	if gitignoreFile == "" {
		gitignoreFile = filepath.Join(os.Getenv("HOME"), ".config", "git", "ignore")
	} else if strings.HasPrefix(gitignoreFile, "~") {
		gitignoreFile = filepath.Join(os.Getenv("HOME"), gitignoreFile[1:])
	}

	if removeLinesFromFile(gitignoreFile, "codemob") {
		info("Removed codemob entries from global gitignore")
	} else {
		info("No codemob entries found in global gitignore")
	}

	fmt.Println()
	info("Uninstalled. Open a new terminal for changes to take effect.")
	return nil
}

// removeLinesFromFile removes all lines containing substr from a file.
// Returns true if any lines were removed.
func removeLinesFromFile(path, substr string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	lines := strings.Split(string(data), "\n")
	var kept []string
	removed := false

	for _, line := range lines {
		if strings.Contains(line, substr) {
			removed = true
			// Also skip preceding blank line
			if len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
				kept = kept[:len(kept)-1]
			}
			continue
		}
		kept = append(kept, line)
	}

	if removed {
		os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0644)
	}
	return removed
}

// ─── File helpers ─────────────────────────────────────────────────────────────

func fileContains(path, substr string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), substr)
}

func fileLineContaining(path, substr string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, substr) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func replaceLineInFile(path, match, replacement string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if strings.Contains(line, match) {
			lines[i] = replacement
		}
	}
	os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}
