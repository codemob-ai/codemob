#!/usr/bin/env bash
# init.sh — standalone setup script for codemob
# Handles global setup (gitignore, shell integration, slash commands)
# and repo setup (.codemob/ directory, config.json).
# Fully idempotent — safe to re-run.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

info()  { echo -e "${GREEN}✓${NC} $1"; }
warn()  { echo -e "${YELLOW}!${NC} $1"; }
err()   { echo -e "${RED}✗${NC} $1" >&2; }

# ─── Global Setup ─────────────────────────────────────────────────────────────

check_dependencies() {
  if ! command -v jq &> /dev/null; then
    warn "jq is not installed. codemob requires jq for JSON parsing."
    warn "Install with: brew install jq"
  fi

  if ! command -v git &> /dev/null; then
    err "git is not installed. codemob requires git."
    exit 1
  fi
}

setup_global_gitignore() {
  local gitignore_file

  # Check git config for excludesFile
  gitignore_file="$(git config --global core.excludesFile 2>/dev/null || true)"

  if [[ -z "$gitignore_file" ]]; then
    gitignore_file="$HOME/.config/git/ignore"
    mkdir -p "$(dirname "$gitignore_file")"
  else
    # Expand ~ if present
    gitignore_file="${gitignore_file/#\~/$HOME}"
    mkdir -p "$(dirname "$gitignore_file")"
  fi

  touch "$gitignore_file"

  if ! grep -qF '.codemob/' "$gitignore_file" 2>/dev/null; then
    echo "" >> "$gitignore_file"
    echo "# codemob workspaces" >> "$gitignore_file"
    echo ".codemob/" >> "$gitignore_file"
    info "Added .codemob/ to global gitignore ($gitignore_file)"
  else
    info "Global gitignore already contains .codemob/"
  fi
}

setup_shell_integration() {
  local zshrc="$HOME/.zshrc"
  local source_line="source \"$SCRIPT_DIR/codemob.sh\""

  if [[ ! -f "$zshrc" ]]; then
    touch "$zshrc"
  fi

  # Check if any codemob source line exists (might be from a different path)
  if grep -qF "codemob.sh" "$zshrc" 2>/dev/null; then
    # Update existing line if path changed
    local existing
    existing="$(grep "codemob.sh" "$zshrc")"
    if [[ "$existing" != "$source_line" ]]; then
      # Replace old source line with new one
      local temp_file
      temp_file="$(mktemp)"
      sed "s|.*codemob.sh.*|$source_line|" "$zshrc" > "$temp_file" && mv "$temp_file" "$zshrc"
      info "Updated codemob source path in ~/.zshrc"
    else
      info "Shell integration already configured in ~/.zshrc"
    fi
  else
    echo "" >> "$zshrc"
    echo "# codemob - AI agent workspace manager" >> "$zshrc"
    echo "$source_line" >> "$zshrc"
    info "Added shell integration to ~/.zshrc"
  fi
}

setup_claude_commands() {
  local commands_dir="$HOME/.claude/commands"
  mkdir -p "$commands_dir"

  local source_dir="$SCRIPT_DIR/claude-commands"
  if [[ ! -d "$source_dir" ]]; then
    warn "Claude commands directory not found at $source_dir, skipping"
    return 0
  fi

  local installed=0
  for cmd_file in "$source_dir"/*.md; do
    [[ -f "$cmd_file" ]] || continue
    local basename
    basename="$(basename "$cmd_file")"
    if [[ ! -f "$commands_dir/$basename" ]] || ! diff -q "$cmd_file" "$commands_dir/$basename" > /dev/null 2>&1; then
      cp "$cmd_file" "$commands_dir/$basename"
      installed=$((installed + 1))
    fi
  done

  if [[ $installed -gt 0 ]]; then
    info "Installed $installed Claude slash command(s) to ~/.claude/commands/"
  else
    info "Claude slash commands are up to date"
  fi
}

# ─── Repo Setup ───────────────────────────────────────────────────────────────

setup_repo() {
  local repo_root
  repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"

  if [[ -z "$repo_root" ]]; then
    warn "Not inside a git repository. Skipping repo setup."
    warn "Run 'codemob init' again from inside a git repo to set up a project."
    return 0
  fi

  local codemob_dir="$repo_root/.codemob"
  local config_file="$codemob_dir/config.json"

  # Create directories
  mkdir -p "$codemob_dir/mobs"

  if [[ -f "$config_file" ]]; then
    info "Repo already initialized at $repo_root"
    return 0
  fi

  # Detect base branch
  local default_branch="main"
  local detected
  detected="$(git -C "$repo_root" symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's|refs/remotes/origin/||' || true)"
  if [[ -n "$detected" ]]; then
    default_branch="$detected"
  fi

  # Prompt for base branch
  echo ""
  read -rp "Base branch for new mobs [$default_branch]: " user_branch
  local base_branch="${user_branch:-$default_branch}"

  # Create config
  local config
  config="$(jq -n --arg agent "claude" --arg branch "$base_branch" '{
    default_agent: $agent,
    base_branch: $branch,
    mobs: []
  }')"

  echo "$config" > "$config_file"
  info "Created $config_file (base_branch: $base_branch)"
}

# ─── Main ─────────────────────────────────────────────────────────────────────

main() {
  echo "codemob init"
  echo "────────────"
  echo ""

  echo "Global setup:"
  check_dependencies
  setup_global_gitignore
  setup_shell_integration
  setup_claude_commands

  echo ""
  echo "Repo setup:"
  setup_repo

  echo ""
  info "Done! Open a new terminal or run: source ~/.zshrc"
}

main "$@"
