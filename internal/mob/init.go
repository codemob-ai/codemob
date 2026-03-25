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

type commandDef struct {
	Description string // shown in agent's command picker
	Body        string // instructions for the agent
}

const triggerGuard = "IMPORTANT: Only invoke this command when the user explicitly mentions " +
	"\"mob\" or \"codemob\". Generic requests like \"list\", \"create\", " +
	"\"remove\", or \"switch\" without mentioning mob/codemob should NOT trigger this.\n\n"

var slashCommandDefs = map[string]commandDef{
	"list": {
		Description: "List all codemob workspaces and their status",
		Body: triggerGuard +
			"Run exactly this command using the Bash tool: codemob list\n\nDo NOT use go run, do NOT cd anywhere. Just run: codemob list\n\nDisplay the output to the user.\n",
	},
	"new": {
		Description: "Create a new codemob workspace",
		Body: triggerGuard + `Ask the user if they want to provide a name or have one auto-generated.

If they provide a name, validate it against these rules before running the command:
- Only letters (a-z, A-Z), numbers, and hyphens allowed (no spaces or special characters)
- Cannot start or end with a hyphen
- Cannot be purely numeric
- Cannot be "root" (reserved)
- Max 60 characters

If the name is invalid, tell the user what's wrong and ask them to pick a different name. Do NOT pass an invalid name to codemob.

If the name is valid, run: ` + "`codemob queue new <name>`" + ` (replace ` + "`<name>`" + ` with their choice).
If they want auto-generated, run: ` + "`codemob queue new`" + ` (no name argument — codemob generates one).

Do NOT generate a name yourself — codemob handles name generation.

Then tell the user: "New mob queued. Exit this session (Ctrl+C) and codemob will automatically create and launch the new mob."
`,
	},
	"switch": {
		Description: "Switch to a different codemob workspace",
		Body: triggerGuard + `Run ` + "`codemob list-others`" + ` using the Bash tool.

If the output says "No mobs", tell the user there are no other mobs to switch to and suggest using /mob-new or /codemob-new to create one.

Otherwise, display the results and ask the user which mob they want to switch to.

Once they pick one, run ` + "`codemob queue switch <name>`" + ` using the Bash tool (replace ` + "`<name>`" + ` with the chosen mob name).

Then tell the user: "Switch queued. Exit this session (Ctrl+C) and codemob will automatically launch the new mob."
`,
	},
	"change-agent": {
		Description: "Switch the current mob to a different AI agent",
		Body: triggerGuard + `codemob supports claude and codex out of the box.

Determine the current agent by checking which tool you are (claude or codex). Offer the OTHER agent — do not suggest the one already running.

Once the user confirms, run ` + "`codemob queue change-agent <agent>`" + ` using the Bash tool (replace ` + "`<agent>`" + ` with the chosen agent name).

Then tell the user: "Agent switch queued. Exit this session (Ctrl+C) and codemob will relaunch with the new agent."
`,
	},
	"remove": {
		Description: "Remove a codemob workspace",
		Body: triggerGuard + `Run ` + "`codemob list`" + ` using the Bash tool and display the results. The current mob is marked with ◀.

Ask the user which mob they want to remove.

If they choose a DIFFERENT mob (not the one marked with ◀), run ` + "`codemob remove <name>`" + ` directly.

If they choose the CURRENT mob (marked with ◀), run this exact command:

` + "```" + `
codemob queue remove "$CODEMOB_MOB"
` + "```" + `

$CODEMOB_MOB is already set in your environment. There is no need to echo it - the command above will resolve it automatically.

Then tell the user: "Removal queued. Exit this session (Ctrl+C) and codemob will remove the mob."
`,
	},
	"drop": {
		Description: "Remove the current codemob workspace and exit",
		Body: triggerGuard + `Run this exact command using the Bash tool:

` + "```" + `
codemob queue remove "$CODEMOB_MOB"
` + "```" + `

$CODEMOB_MOB is already set in your environment. There is no need to echo it - the command above will resolve it automatically.

If the command fails, tell the user: "This command can only be used from within a codemob workspace." and stop.

Otherwise, tell the user: "Mob queued for removal. Exit this session (Ctrl+C) and codemob will remove it."
`,
	},
}

// SlashCommands returns Claude Code slash commands (description as first line, then body).
// When multipleAgents is false, the change-agent command is omitted.
func SlashCommands(multipleAgents bool) map[string]string {
	cmds := make(map[string]string)
	for name, def := range slashCommandDefs {
		if name == "change-agent" && !multipleAgents {
			continue
		}
		content := def.Description + ".\n\n" + def.Body
		cmds["mob-"+name+".md"] = content
		cmds["codemob-"+name+".md"] = content
	}
	return cmds
}

// CodexPrompts returns Codex custom prompts (YAML front matter with description, then body).
// When multipleAgents is false, the change-agent prompt is omitted.
func CodexPrompts(multipleAgents bool) map[string]string {
	prompts := make(map[string]string)
	for name, def := range slashCommandDefs {
		if name == "change-agent" && !multipleAgents {
			continue
		}
		prompt := fmt.Sprintf("---\ndescription: %s\n---\n\n%s\n", def.Description, def.Body)
		prompts["mob-"+name+".md"] = prompt
		prompts["codemob-"+name+".md"] = prompt
	}
	return prompts
}

const (
	green  = "\033[38;2;106;191;105m" // #6abf69 — bright green derived from brand #002900
	accent = "\033[38;2;231;220;96m"  // #e7dc60 — brand accent
	red    = "\033[38;2;217;83;79m"   // #d9534f — warm red to match brand palette
	reset  = "\033[0m"

	ColorRed   = red
	ColorReset = reset
)

func printBanner() {
	PrintBanner(accent)
}

func PrintBanner(color string) {
	fmt.Println()
	fmt.Println()
	fmt.Print(color)
	fmt.Println("  ▄████▄  ▒█████  ▓█████▄ ▓█████ ███▄ ▄███▓ ▒█████   ▄▄▄▄   ")
	fmt.Println("▒██▀ ▀█ ▒██▒  ██▒▒██▀ ██▌▓█   ▀▓██▒▀█▀ ██▒▒██▒  ██▒▓█████▄ ")
	fmt.Println("▒▓█    ▄▒██░  ██▒░██   █▌▒███  ▓██    ▓██░▒██░  ██▒▒██▒ ▄██")
	fmt.Println("▒▓▓▄ ▄██▒██   ██░░▓█▄   ▌▒▓█  ▄▒██    ▒██ ▒██   ██░▒██░█▀  ")
	fmt.Println("▒ ▓███▀ ░ ████▓▒░░▒████▓ ░▒████▒██▒   ░██▒░ ████▓▒░░▓█  ▀█▓")
	fmt.Println("░ ░▒ ▒  ░ ▒░▒░▒░  ▒▒▓  ▒ ░░ ▒░ ░ ▒░   ░  ░░ ▒░▒░▒░ ░▒▓███▀▒")
	fmt.Println("  ░  ▒    ░ ▒ ▒░  ░ ▒  ▒  ░ ░  ░  ░      ░  ░ ▒ ▒░ ▒░▒   ░ ")
	fmt.Println("░       ░ ░ ░ ▒   ░ ░  ░    ░  ░      ░   ░ ░ ░ ▒   ░    ░ ")
	fmt.Println("░ ░         ░ ░     ░       ░  ░      ░       ░ ░   ░      ")
	fmt.Println("░                 ░                                      ░ ")
	fmt.Print(reset)
	fmt.Println()
}

func info(msg string)   { fmt.Printf("%s✓ %s%s\n", green, msg, reset) }
func warn(msg string)   { fmt.Printf("%s! %s%s\n", accent, msg, reset) }
func errMsg(msg string) { fmt.Printf("%s✗ %s%s\n", red, msg, reset) }

// Init performs the full codemob initialization.
// installDir is the directory where codemob-shell.sh lives.
func Init(installDir string, forceReprompt bool) error {
	printBanner()
	fmt.Println("codemob init")
	fmt.Println("────────────")
	fmt.Println()
	warn("This will:")
	fmt.Println("  - Add shell integration (mob alias, claude/codex wrappers) to your shell RC file")
	fmt.Println("  - Add codemob entries to global gitignore")
	fmt.Println("  - Add codemob permissions to Claude settings (if installed)")
	fmt.Println("  - Initialize .codemob/ config in the current repo")
	fmt.Println("  - Install slash commands for your AI agents")
	fmt.Println()
	fmt.Printf("  All of this can be easily reverted with: %scodemob uninstall%s\n", green, reset)
	fmt.Println()
	fmt.Print("Continue? [Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "n" || input == "no" {
		fmt.Println("Cancelled.")
		return nil
	}

	fmt.Println()
	fmt.Println("Global setup:")
	claude, codex, err := checkDependencies()
	if err != nil {
		return err
	}
	setupGlobalGitignore()
	setupShellIntegration(installDir)
	if claude.installed {
		setupClaudePermissions()
	}

	fmt.Println()
	fmt.Println("Repo setup:")
	bothReady := claude.installed && codex.installed
	repoRoot := setupRepo(forceReprompt)
	if repoRoot == "" {
		return nil // not in a git repo, or user cancelled
	}
	if claude.installed {
		setupClaudeCommands(repoRoot, bothReady)
	}
	if codex.installed {
		setupCodexPrompts(bothReady)
	}

	_, rcName := detectShellRC()
	fmt.Println()
	fmt.Println("────────────────────────────────────────────────────────")
	warn("codemob won't work until you reload your shell!")
	fmt.Println()
	fmt.Println("  Either open a new terminal, or run:")
	fmt.Printf("  source %s\n", rcName)
	fmt.Println("────────────────────────────────────────────────────────")
	return nil
}

type agentStatus struct {
	installed bool
}

// checkDependencies checks git and agent availability. Returns per-agent status.
func checkDependencies() (claude, codex agentStatus, err error) {
	if _, err := exec.LookPath("git"); err != nil {
		errMsg("git is not installed. codemob requires git.")
		return agentStatus{}, agentStatus{}, fmt.Errorf("git not found")
	}
	info("git found")

	claude = checkAgent("claude", "npm install -g @anthropic-ai/claude-code")
	codex = checkAgent("codex", "npm install -g @openai/codex")

	if !claude.installed && !codex.installed {
		fmt.Println()
		errMsg("No AI agents found. codemob requires at least one (claude or codex).")
		return claude, codex, fmt.Errorf("no agents found")
	}
	return claude, codex, nil
}

func checkAgent(name, installHint string) agentStatus {
	_, err := exec.LookPath(name)
	if err != nil {
		errMsg(fmt.Sprintf("%s not found — install: %s", name, installHint))
		return agentStatus{}
	}

	out, err := exec.Command(name, "--version").Output()
	if err != nil {
		warn(fmt.Sprintf("%s found but could not determine version", name))
		return agentStatus{installed: true}
	}

	version := strings.TrimSpace(string(out))
	// Some tools output multi-line; take first line only.
	if i := strings.IndexByte(version, '\n'); i != -1 {
		version = version[:i]
	}
	info(fmt.Sprintf("%s found (%s)", name, version))

	checkAgentAuth(name)
	return agentStatus{installed: true}
}

// checkAgentAuth is a best-effort auth check. Never blocks init.
func checkAgentAuth(name string) {
	switch name {
	case "claude":
		out, err := exec.Command("claude", "auth", "status", "--json").Output()
		if err != nil {
			warn(fmt.Sprintf("Could not verify %s auth — unauthenticated agents may not work as expected", name))
			return
		}
		var result struct {
			LoggedIn bool `json:"loggedIn"`
		}
		if err := json.Unmarshal(out, &result); err != nil || !result.LoggedIn {
			errMsg(fmt.Sprintf("%s is not authenticated — run: claude auth login", name))
			return
		}
		info(fmt.Sprintf("%s authenticated", name))

	case "codex":
		// Exit code 0 = logged in, non-zero = not logged in or command failed
		if err := exec.Command("codex", "login", "status").Run(); err != nil {
			errMsg(fmt.Sprintf("%s is not authenticated — run: codex login", name))
			return
		}
		info(fmt.Sprintf("%s authenticated", name))
	}
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
		// Tell git about it
		exec.Command("git", "config", "--global", "core.excludesFile", gitignoreFile).Run()
	} else {
		if strings.HasPrefix(gitignoreFile, "~") {
			gitignoreFile = filepath.Join(os.Getenv("HOME"), gitignoreFile[1:])
		}
	}

	// Ensure parent dir exists
	os.MkdirAll(filepath.Dir(gitignoreFile), 0755)

	patterns := map[string]string{
		".codemob/":                      ".codemob/",
		".claude/commands/mob-*.md":      ".claude/commands/mob-*.md",
		".claude/commands/codemob-*.md":  ".claude/commands/codemob-*.md",
	}

	var missing []string
	for check, line := range patterns {
		if !fileContains(gitignoreFile, check) {
			missing = append(missing, line)
		}
	}
	if len(missing) == 0 {
		info("Global gitignore already configured for codemob")
		return
	}

	f, err := os.OpenFile(gitignoreFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		warn(fmt.Sprintf("Could not write to %s: %v", gitignoreFile, err))
		return
	}
	defer f.Close()

	if !fileContains(gitignoreFile, "# codemob") {
		f.WriteString("\n# codemob\n")
	}
	for _, line := range missing {
		f.WriteString(line + "\n")
	}
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

func findShellScript(binDir string) string {
	// Standard install: <prefix>/bin/codemob → <prefix>/share/codemob/codemob-shell.sh
	shareDir := filepath.Join(filepath.Dir(binDir), "share", "codemob", "codemob-shell.sh")
	if _, err := os.Stat(shareDir); err == nil {
		return shareDir
	}
	// Fallback: shell script next to binary
	return filepath.Join(binDir, "codemob-shell.sh")
}

func setupShellIntegration(installDir string) {
	rcFile, rcName := detectShellRC()
	shellScript := findShellScript(installDir)
	sourceLine := fmt.Sprintf(`source "%s"`, shellScript)

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

var codemobPermissions = []string{
	"Bash(codemob *)",
	"Bash(mob *)",
}

func setupClaudePermissions() {
	settingsPath := filepath.Join(os.Getenv("HOME"), ".claude", "settings.json")

	// Read existing settings or start fresh
	var settings map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			warn(fmt.Sprintf("Could not parse Claude settings: %v", err))
			return
		}
	}
	if settings == nil {
		settings = make(map[string]interface{})
	}

	// Get or create permissions.allow
	perms, _ := settings["permissions"].(map[string]interface{})
	if perms == nil {
		perms = make(map[string]interface{})
	}

	allowList, _ := perms["allow"].([]interface{})

	// Build set of existing permissions
	existing := make(map[string]bool)
	for _, p := range allowList {
		if s, ok := p.(string); ok {
			existing[s] = true
		}
	}

	// Add missing codemob permissions
	added := 0
	for _, perm := range codemobPermissions {
		if !existing[perm] {
			allowList = append(allowList, perm)
			added++
		}
	}

	if added == 0 {
		info("Claude permissions already configured for codemob")
		return
	}

	perms["allow"] = allowList
	settings["permissions"] = perms

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		warn(fmt.Sprintf("Could not create Claude settings directory: %v", err))
		return
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		warn(fmt.Sprintf("Could not serialize Claude settings: %v", err))
		return
	}
	if err := os.WriteFile(settingsPath, append(out, '\n'), 0644); err != nil {
		warn(fmt.Sprintf("Could not write Claude settings: %v", err))
		return
	}

	info("Added codemob permissions to Claude settings")
}

func removeClaudePermissions() {
	settingsPath := filepath.Join(os.Getenv("HOME"), ".claude", "settings.json")

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		info("No Claude settings to clean up")
		return
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		warn(fmt.Sprintf("Could not parse Claude settings: %v", err))
		return
	}

	perms, _ := settings["permissions"].(map[string]interface{})
	if perms == nil {
		info("No Claude permissions to clean up")
		return
	}

	allowList, _ := perms["allow"].([]interface{})
	if len(allowList) == 0 {
		info("No Claude permissions to clean up")
		return
	}

	toRemove := make(map[string]bool)
	for _, perm := range codemobPermissions {
		toRemove[perm] = true
	}

	filtered := make([]interface{}, 0)
	removed := 0
	for _, p := range allowList {
		if s, ok := p.(string); ok && toRemove[s] {
			removed++
			continue
		}
		filtered = append(filtered, p)
	}

	if removed == 0 {
		info("No codemob permissions found in Claude settings")
		return
	}

	perms["allow"] = filtered
	settings["permissions"] = perms

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		warn(fmt.Sprintf("Could not serialize Claude settings: %v", err))
		return
	}
	if err := os.WriteFile(settingsPath, append(out, '\n'), 0644); err != nil {
		warn(fmt.Sprintf("Could not write Claude settings: %v", err))
		return
	}

	info("Removed codemob permissions from Claude settings")
}

func setupClaudeCommands(repoRoot string, multipleAgents bool) {
	if repoRoot == "" {
		warn("Not inside a git repository. Skipping Claude commands setup.")
		return
	}
	commandsDir := filepath.Join(repoRoot, ".claude", "commands")
	os.MkdirAll(commandsDir, 0755)

	installed := 0
	for name, content := range SlashCommands(multipleAgents) {
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

func setupCodexPrompts(multipleAgents bool) {
	promptsDir := filepath.Join(os.Getenv("HOME"), ".codex", "prompts")
	os.MkdirAll(promptsDir, 0755)

	installed := 0
	for name, content := range CodexPrompts(multipleAgents) {
		dest := filepath.Join(promptsDir, name)
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
		info("Installed Codex prompts")
	} else {
		info("Codex prompts are up to date")
	}
}

func setupRepo(reprompt bool) string {
	if mainRoot := InsideWorktree(); mainRoot != "" {
		warn("You're inside a mob worktree.")
		fmt.Println("  Run 'codemob cd root' to go back to the main repo, then try again.")
		return ""
	}

	root, err := gitutil.RepoRoot()
	if err != nil {
		warn("Not inside a git repository. Skipping repo setup.")
		warn("Run 'codemob init' again from inside a git repo to set up a project.")
		return ""
	}

	// Load existing config or start with defaults
	cfg, _ := LoadConfig(root)
	isNew := cfg == nil
	if isNew {
		cfg = &Config{
			DefaultAgent: "claude",
			BaseBranch:   gitutil.DetectDefaultBranch(root),
			Mobs:         []Mob{},
		}
	}

	fullyConfigured := cfg.RepoRoot != ""

	if !isNew && !reprompt && fullyConfigured {
		info(fmt.Sprintf("Repo already initialized at %s", root))
		return root
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println()
	fmt.Printf("Base branch for new mobs [%s]: ", cfg.BaseBranch)
	input, _ := reader.ReadString('\n')
	if v := strings.TrimSpace(input); v != "" {
		cfg.BaseBranch = v
	}

	fmt.Printf("Default agent (claude/codex) [%s]: ", cfg.DefaultAgent)
	input, _ = reader.ReadString('\n')
	if v := strings.TrimSpace(input); v != "" {
		cfg.DefaultAgent = v
	}

	// Mobs directory prompt
	repoName := filepath.Base(root)
	home := os.Getenv("HOME")
	enclosingPath := filepath.Join(filepath.Dir(root), ".codemob", repoName, "mobs")
	globalPath := filepath.Join(home, ".codemob", repoName, "mobs")

	currentDefault := "1"
	switch cfg.MobsDirPath {
	case enclosingPath:
		currentDefault = "2"
	case globalPath:
		currentDefault = "3"
	}

	fmt.Println()
	fmt.Println("Where should mob worktrees live?")
	fmt.Printf("  1) Project dir    %s/\n", filepath.Join(root, MobsDir))
	fmt.Printf("  2) Enclosing dir  %s/\n", enclosingPath)
	fmt.Printf("  3) Global dir     %s/\n", globalPath)
	fmt.Printf("\nMobs directory [%s]: ", currentDefault)
	input, _ = reader.ReadString('\n')
	choice := strings.TrimSpace(input)
	if choice == "" {
		choice = currentDefault
	}

	oldMobsDir := cfg.MobsDirPath
	switch choice {
	case "2":
		cfg.MobsDirPath = enclosingPath
	case "3":
		cfg.MobsDirPath = globalPath
	default:
		cfg.MobsDirPath = filepath.Join(root, MobsDir)
	}

	if !isNew && oldMobsDir != cfg.MobsDirPath && len(cfg.Mobs) > 0 {
		fmt.Println()
		errMsg(fmt.Sprintf("You have %d existing mob(s) at the old location.", len(cfg.Mobs)))
		fmt.Println("  codemob will no longer track them, but the worktrees (and the linked git branches) will remain on disk.")
		fmt.Println("  Run 'codemob purge' or 'codemob remove' first to clean them up.")
		fmt.Print("\nContinue anyway? [y/N]: ")
		input, _ = reader.ReadString('\n')
		if v := strings.TrimSpace(strings.ToLower(input)); v != "y" && v != "yes" {
			fmt.Println("Cancelled.")
			return ""
		}
	}

	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		cfg.RepoRoot = resolved
	} else {
		cfg.RepoRoot = root
	}

	// Create the mobs directory
	os.MkdirAll(MobsPath(root, cfg), 0755)

	if err := SaveConfig(root, cfg); err != nil {
		warn(fmt.Sprintf("Could not write config: %v", err))
		return root
	}

	if isNew {
		info(fmt.Sprintf("Created config (base_branch: %s, default_agent: %s)", cfg.BaseBranch, cfg.DefaultAgent))
	} else {
		info(fmt.Sprintf("Updated config (base_branch: %s, default_agent: %s)", cfg.BaseBranch, cfg.DefaultAgent))
	}
	return root
}

// ─── Uninstall ────────────────────────────────────────────────────────────────

// Uninstall removes all codemob setup — global and local.
func Uninstall(installDir string) error {
	printBanner()
	fmt.Println("codemob uninstall")
	fmt.Println("─────────────────")
	fmt.Println()
	warn("This will:")
	fmt.Println("  - Remove shell integration (codemob/mob/claude functions) from your shell RC file")
	fmt.Println("  - Remove codemob entries from global gitignore")

	// Check if we're in a codemob project
	repoRoot := InsideWorktree()
	if repoRoot == "" {
		repoRoot, _ = gitutil.RepoRoot()
	}
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
				worktreePath := MobPath(repoRoot, cfg, m.Name)
				_ = gitutil.WorktreeRemove(repoRoot, worktreePath, true)
				gitutil.BranchDelete(repoRoot, m.Branch)
			}
		}
		if cfg != nil {
			CleanupExternalMobsDir(repoRoot, cfg.MobsDirPath)
		}
		// Remove .codemob/ directory
		os.RemoveAll(filepath.Join(repoRoot, CodemobDir))
		info("Removed .codemob/ and all worktrees")

		// Remove slash commands from project
		for name := range SlashCommands(true) {
			os.Remove(filepath.Join(repoRoot, ".claude", "commands", name))
		}
		info("Removed codemob slash commands from .claude/commands/")
	}

	// Remove shell integration
	rcFile, rcName := detectShellRC()
	if removeCodemobLines(rcFile) {
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

	if removeCodemobLines(gitignoreFile) {
		info("Removed codemob entries from global gitignore")
	} else {
		info("No codemob entries found in global gitignore")
	}

	// Remove Claude permissions
	removeClaudePermissions()

	// Remove Codex prompts
	promptsDir := filepath.Join(os.Getenv("HOME"), ".codex", "prompts")
	removedPrompts := 0
	for name := range CodexPrompts(true) {
		if err := os.Remove(filepath.Join(promptsDir, name)); err == nil {
			removedPrompts++
		}
	}
	if removedPrompts > 0 {
		info("Removed Codex prompts")
	} else {
		info("No Codex prompts to clean up")
	}

	// Remove global version file
	os.Remove(globalVersionFile())

	fmt.Println()
	info("Uninstalled. Open a new terminal for changes to take effect.")
	return nil
}

// removeCodemobLines removes lines matching specific codemob markers from a file.
// Only removes lines containing "codemob-shell.sh", "codemob.sh", or "# codemob".
// Returns true if any lines were removed.
func removeCodemobLines(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	markers := []string{"codemob-shell.sh", "codemob.sh", "# codemob"}
	lines := strings.Split(string(data), "\n")
	var kept []string
	removed := false

	for _, line := range lines {
		matched := false
		for _, marker := range markers {
			if strings.Contains(line, marker) {
				matched = true
				break
			}
		}
		if matched {
			removed = true
			if len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
				kept = kept[:len(kept)-1]
			}
			continue
		}
		kept = append(kept, line)
	}

	if removed {
		if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0644); err != nil {
			warn(fmt.Sprintf("Could not write %s: %v", path, err))
			return false
		}
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
