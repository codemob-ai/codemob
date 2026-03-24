package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type WorktreeInfo struct {
	Path   string
	Branch string
	HEAD   string
	Bare   bool
}

func RepoRoot() (string, error) {
	out, err := runGit("", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return strings.TrimSpace(out), nil
}

func WorktreeList(repoRoot string) ([]WorktreeInfo, error) {
	out, err := runGit(repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}
	return parseWorktreeList(out), nil
}

func WorktreeAdd(repoRoot, path, branch, base string) error {
	_, err := runGit(repoRoot, "worktree", "add", "-b", branch, path, base)
	if err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}
	return nil
}

func WorktreeRemove(repoRoot, path string, force bool) error {
	args := []string{"worktree", "remove", path}
	if force {
		args = []string{"worktree", "remove", "-f", path}
	}
	_, err := runGit(repoRoot, args...)
	if err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}
	return nil
}

func BranchDelete(repoRoot, branch string) {
	_, _ = runGit(repoRoot, "branch", "-D", branch) // best-effort
}

func DetectDefaultBranch(repoRoot string) string {
	out, err := runGit(repoRoot, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		return "main"
	}
	ref := strings.TrimSpace(out)
	// refs/remotes/origin/main -> main
	parts := strings.Split(ref, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "main"
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stdout.String(), fmt.Errorf("%s: %w", strings.TrimSpace(stderr.String()), err)
	}
	return stdout.String(), nil
}

func parseWorktreeList(raw string) []WorktreeInfo {
	var worktrees []WorktreeInfo
	var current *WorktreeInfo

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				worktrees = append(worktrees, *current)
				current = nil
			}
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			current = &WorktreeInfo{Path: strings.TrimPrefix(line, "worktree ")}
		} else if current != nil {
			if strings.HasPrefix(line, "HEAD ") {
				current.HEAD = strings.TrimPrefix(line, "HEAD ")
			} else if strings.HasPrefix(line, "branch ") {
				current.Branch = strings.TrimPrefix(line, "branch ")
			} else if line == "bare" {
				current.Bare = true
			}
		}
	}
	if current != nil {
		worktrees = append(worktrees, *current)
	}
	return worktrees
}
