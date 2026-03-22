package mob

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const nextFile = ".codemob/next.json"

// NextAction represents a pending action to execute after an agent exits.
type NextAction struct {
	Action string `json:"action"` // "switch", "new", etc.
	Target string `json:"target"` // mob name for switch
}

// WriteNextAction writes an action for the trampoline to pick up.
func WriteNextAction(repoRoot string, action NextAction) error {
	data, err := json.MarshalIndent(action, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(repoRoot, nextFile), append(data, '\n'), 0644)
}

// ReadNextAction reads and returns the pending action, if any.
func ReadNextAction(repoRoot string) (*NextAction, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, nextFile))
	if err != nil {
		return nil, nil // no file = no action
	}
	var action NextAction
	if err := json.Unmarshal(data, &action); err != nil {
		return nil, nil // corrupt file = no action
	}
	return &action, nil
}

// ClearNextAction removes the next action file.
func ClearNextAction(repoRoot string) {
	os.Remove(filepath.Join(repoRoot, nextFile))
}
