#!/usr/bin/env bash
# codemob-shell.sh — optional terminal enhancement for codemob
# Source this in .zshrc/.bashrc to get:
#   - mob() alias for codemob
#   - claude/codex wrappers with --new-mob / --resume-mob flags

# Unique per-terminal session ID — used by codemob to track the last active mob
# per terminal so `resume` can default to it without collisions.
[ -z "$CODEMOB_SESSION" ] && export CODEMOB_SESSION=$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid 2>/dev/null || echo $$)

codemob() {
  case "${1:-}" in
    cd) shift
      local dir
      dir="$(command codemob path "$@")" || return $?
      if [ "$dir" = "$PWD" ]; then
        echo "Already here."
        return 0
      fi
      cd "$dir"
      ;;
    *)  command codemob "$@" ;;
  esac
}

mob() {
  codemob "$@"
}

claude() {
  case "${1:-}" in
    --mob|--codemob|--new-mob|--new-codemob) shift; codemob new --agent claude "$@"; return $? ;;
    --resume-mob|--resume-codemob)       shift; codemob resume "$@"; return $? ;;
    --open-mob|--open-codemob)           shift; codemob open --agent claude "$@"; return $? ;;
    --switch-mob|--switch-codemob)       shift; codemob switch "$@"; return $? ;;
    --list-mob|--list-mobs|--list-codemob|--list-codemobs) shift; codemob list "$@"; return $? ;;
    *)
      local extra_args=() codemob_mob=""
      while IFS= read -r line; do
        case "$line" in
          CODEMOB_MOB=*) codemob_mob="${line#CODEMOB_MOB=}" ;;
          *) extra_args+=("$line") ;;
        esac
      done < <(command codemob inject-args claude 2>/dev/null)
      CODEMOB_MOB="$codemob_mob" command claude "${extra_args[@]}" "$@"
      local ec=$?
      CODEMOB_MOB="$codemob_mob" codemob check-queue 2>/dev/null
      return $ec
      ;;
  esac
}

codex() {
  case "${1:-}" in
    --mob|--codemob|--new-mob|--new-codemob) shift; codemob new --agent codex "$@"; return $? ;;
    --resume-mob|--resume-codemob)       shift; codemob resume "$@"; return $? ;;
    --open-mob|--open-codemob)           shift; codemob open --agent codex "$@"; return $? ;;
    --switch-mob|--switch-codemob)       shift; codemob switch "$@"; return $? ;;
    --list-mob|--list-mobs|--list-codemob|--list-codemobs) shift; codemob list "$@"; return $? ;;
    *)
      local extra_args=() codemob_mob=""
      while IFS= read -r line; do
        case "$line" in
          CODEMOB_MOB=*) codemob_mob="${line#CODEMOB_MOB=}" ;;
          *) extra_args+=("$line") ;;
        esac
      done < <(command codemob inject-args codex 2>/dev/null)
      CODEMOB_MOB="$codemob_mob" command codex "${extra_args[@]}" "$@"
      local ec=$?
      CODEMOB_MOB="$codemob_mob" codemob check-queue 2>/dev/null
      return $ec
      ;;
  esac
}
