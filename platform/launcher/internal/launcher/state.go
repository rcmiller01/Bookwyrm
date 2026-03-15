package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type firstRunState struct {
	Complete  bool      `json:"complete"`
	Completed time.Time `json:"completed_at,omitempty"`
}

func loadFirstRunState(path string) (firstRunState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return firstRunState{}, nil
		}
		return firstRunState{}, err
	}
	var state firstRunState
	if err := json.Unmarshal(raw, &state); err != nil {
		return firstRunState{}, err
	}
	return state, nil
}

func saveFirstRunState(path string, state firstRunState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
