package mob

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ValidQueueActions = map[string]bool{
	"switch":       true,
	"new":          true,
	"remove":       true,
	"change-agent": true,
}

const queuesDir = ".codemob/queues"

// QueueSessionID returns the current codemob session id from the environment.
func QueueSessionID() (string, error) {
	sessionID := strings.TrimSpace(os.Getenv("CODEMOB_SESSION"))
	if sessionID == "" {
		return "", fmt.Errorf("queue commands require CODEMOB_SESSION")
	}
	return sessionID, nil
}

// QueuedAction represents a pending action to execute after an agent exits.
type QueuedAction struct {
	Action string `json:"action"`          // "switch", "new", "remove", "change-agent"
	Target string `json:"target"`          // mob name, agent name, etc.
	Mob    string `json:"mob,omitempty"`   // current mob name (for change-agent)
	Agent  string `json:"agent,omitempty"` // agent to use (for new)
}

// WriteQueuedAction writes an action for the trampoline to pick up.
func WriteQueuedAction(repoRoot, sessionID string, action QueuedAction) error {
	data, err := json.MarshalIndent(action, "", "  ")
	if err != nil {
		return err
	}
	path := QueueFilePath(repoRoot, sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// ReadQueuedAction reads and returns the pending action, if any.
func ReadQueuedAction(repoRoot, sessionID string) (*QueuedAction, error) {
	data, err := os.ReadFile(QueueFilePath(repoRoot, sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no file = no action
		}
		return nil, fmt.Errorf("could not read queue file: %w", err)
	}
	var action QueuedAction
	if err := json.Unmarshal(data, &action); err != nil {
		return nil, fmt.Errorf("corrupt queue file: %w", err)
	}
	if !ValidQueueActions[action.Action] {
		return nil, fmt.Errorf("unknown queued action: %s", action.Action)
	}
	return &action, nil
}

// QueueFilePath returns the absolute path to the queue file for a given session.
func QueueFilePath(repoRoot, sessionID string) string {
	return filepath.Join(repoRoot, queuesDir, sessionID+".json")
}

// ClearQueue removes the queued action file for a given session.
func ClearQueue(repoRoot, sessionID string) {
	os.Remove(QueueFilePath(repoRoot, sessionID))
}
