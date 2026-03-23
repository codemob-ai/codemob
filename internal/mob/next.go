package mob

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var ValidQueueActions = map[string]bool{
	"switch":       true,
	"new":          true,
	"remove":       true,
	"change-agent": true,
}

const queueFile = ".codemob/queue.json"

// QueuedAction represents a pending action to execute after an agent exits.
type QueuedAction struct {
	Action string `json:"action"`           // "switch", "new", "remove", "change-agent"
	Target string `json:"target"`           // mob name, agent name, etc.
	Mob    string `json:"mob,omitempty"`    // current mob name (for change-agent)
	Agent  string `json:"agent,omitempty"`  // agent to use (for new)
}

// WriteQueuedAction writes an action for the trampoline to pick up.
func WriteQueuedAction(repoRoot string, action QueuedAction) error {
	data, err := json.MarshalIndent(action, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(repoRoot, queueFile), append(data, '\n'), 0644)
}

// ReadQueuedAction reads and returns the pending action, if any.
func ReadQueuedAction(repoRoot string) (*QueuedAction, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, queueFile))
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

// ClearQueue removes the queued action file.
func ClearQueue(repoRoot string) {
	os.Remove(filepath.Join(repoRoot, queueFile))
}
