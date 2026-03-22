package mob

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const queueFile = ".codemob/queue.json"

// QueuedAction represents a pending action to execute after an agent exits.
type QueuedAction struct {
	Action string `json:"action"`          // "switch", "new", "remove", "change-agent"
	Target string `json:"target"`          // mob name, agent name, etc.
	Mob    string `json:"mob,omitempty"`   // current mob name (for change-agent)
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
		return nil, nil // no file = no action
	}
	var action QueuedAction
	if err := json.Unmarshal(data, &action); err != nil {
		return nil, nil // corrupt file = no action
	}
	return &action, nil
}

// ClearQueue removes the queued action file.
func ClearQueue(repoRoot string) {
	os.Remove(filepath.Join(repoRoot, queueFile))
}
