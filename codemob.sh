#!/usr/bin/env bash
# codemob.sh — sourced into shell via .zshrc
# Defines codemob(), mob(), and claude() wrapper functions.
# Thin layer: parses args, delegates to codemob-core (Go binary), handles cd + agent launch.

# Resolve where codemob lives (set at source time)
CODEMOB_INSTALL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
CODEMOB_CORE="$CODEMOB_INSTALL_DIR/codemob-core"

_codemob_core() {
  "$CODEMOB_CORE" "$@"
}

_codemob_launch_agent() {
  local agent="$1"
  local mode="${2:-new}" # "new" or "resume"
  case "$agent" in
    claude)
      if [[ "$mode" == "resume" ]]; then
        command claude --continue
      else
        command claude
      fi
      ;;
    codex)
      if [[ "$mode" == "resume" ]]; then
        command codex resume --last
      else
        command codex
      fi
      ;;
    *)
      echo "codemob: unknown agent '$agent'. Launching shell instead." >&2
      ;;
  esac
}

# Parse CODEMOB_KEY=value lines from core output
_codemob_parse() {
  local line
  while IFS= read -r line; do
    case "$line" in
      CODEMOB_*=*) eval "$line" ;;
      *) echo "$line" ;;
    esac
  done
}

codemob() {
  # Reset result vars
  local CODEMOB_PATH="" CODEMOB_AGENT="" CODEMOB_NAME="" CODEMOB_BRANCH="" CODEMOB_NO_LAUNCH=""

  case "${1:-}" in
    --new)
      shift
      local result
      result="$(_codemob_core new "$@" 2>&1)" || { echo "$result" >&2; return 1; }
      _codemob_parse <<< "$result"

      echo "Created mob '$CODEMOB_NAME' on branch $CODEMOB_BRANCH"

      if [[ -n "$CODEMOB_PATH" ]]; then
        cd "$CODEMOB_PATH" || return 1
        if [[ "$CODEMOB_NO_LAUNCH" != "true" && -n "$CODEMOB_AGENT" ]]; then
          _codemob_launch_agent "$CODEMOB_AGENT" new
        fi
      fi
      ;;

    --list|--ls)
      shift
      _codemob_core list "$@"
      ;;

    --resume|--switch)
      shift
      # Filter out --no-launch before passing to core
      local no_launch=false
      local core_args=()
      for arg in "$@"; do
        if [[ "$arg" == "--no-launch" ]]; then
          no_launch=true
        else
          core_args+=("$arg")
        fi
      done

      local result
      result="$(_codemob_core resolve "${core_args[@]}" 2>&1)" || { echo "$result" >&2; return 1; }
      _codemob_parse <<< "$result"

      if [[ -n "$CODEMOB_PATH" ]]; then
        echo "Switching to mob '$CODEMOB_NAME'"
        cd "$CODEMOB_PATH" || return 1
        if [[ "$no_launch" == false && -n "$CODEMOB_AGENT" ]]; then
          _codemob_launch_agent "$CODEMOB_AGENT" resume
        fi
      fi
      ;;

    init|reinit)
      shift
      _codemob_core init "$@"
      ;;

    remove)
      shift
      _codemob_core remove "$@"
      ;;

    --version|-v)
      echo "codemob v0.1.0"
      ;;

    --help|-h|"")
      echo "Usage: codemob <command>"
      echo ""
      echo "Flags:"
      echo "  --new [name]       Create a new mob and launch agent"
      echo "  --list             List all mobs"
      echo "  --resume <name>    Resume a mob (cd + launch agent)"
      echo "  --switch <name>    Alias for --resume"
      echo ""
      echo "Commands:"
      echo "  init               Initialize codemob (global + repo setup)"
      echo "  reinit             Re-run initialization (idempotent)"
      echo "  remove <name>      Remove a mob"
      echo ""
      echo "Options:"
      echo "  --no-launch        Skip launching the agent"
      echo "  --agent <name>     Override agent (default: from config)"
      echo "  --force            Force remove (for remove command)"
      echo "  --help             Show this help"
      echo "  --version          Show version"
      ;;

    *)
      echo "codemob: unknown command '$1'. Run 'codemob --help' for usage." >&2
      return 1
      ;;
  esac
}

# Alias
mob() {
  codemob "$@"
}

# Claude wrapper — intercepts --new-mob / --resume-mob, passthrough otherwise
claude() {
  case "${1:-}" in
    --new-mob)
      shift
      codemob --new "$@"
      ;;
    --resume-mob)
      shift
      codemob --resume "$@"
      ;;
    *)
      command claude "$@"
      ;;
  esac
}
