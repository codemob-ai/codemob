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
	"cd":           true,
}

const queuesDir = ".codemob/queues"

// QueuedAction represents a pending action to execute after an agent exits.
type QueuedAction struct {
	Action string `json:"action"`           // "switch", "new", "remove", "change-agent"
	Target string `json:"target"`           // mob name, agent name, etc.
	Mob    string `json:"mob,omitempty"`    // current mob name (for change-agent)
	Agent  string `json:"agent,omitempty"`  // agent to use (for new)
}

// WriteQueuedAction writes an action for the trampoline to pick up.
func WriteQueuedAction(repoRoot, mobName string, action QueuedAction) error {
	data, err := json.MarshalIndent(action, "", "  ")
	if err != nil {
		return err
	}
	path := QueueFilePath(repoRoot, mobName)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// ReadQueuedAction reads and returns the pending action, if any.
func ReadQueuedAction(repoRoot, mobName string) (*QueuedAction, error) {
	data, err := os.ReadFile(QueueFilePath(repoRoot, mobName))
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

// QueueFilePath returns the absolute path to the queue file for a given mob.
func QueueFilePath(repoRoot, mobName string) string {
	return filepath.Join(repoRoot, queuesDir, mobName+".json")
}

// ClearQueue removes the queued action file for a given mob.
func ClearQueue(repoRoot, mobName string) {
	os.Remove(QueueFilePath(repoRoot, mobName))
}

// ClearAllQueues removes all queued action files.
func ClearAllQueues(repoRoot string) {
	os.RemoveAll(filepath.Join(repoRoot, queuesDir))
}
