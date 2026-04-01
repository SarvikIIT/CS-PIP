package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// runsDir returns (and creates if necessary) the directory used to store
// per-container run records: ~/.cspip/runs/
func runsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("store: cannot determine home dir: %w", err)
	}
	dir := filepath.Join(home, ".cspip", "runs")
	return dir, os.MkdirAll(dir, 0750)
}

// Save persists rec to ~/.cspip/runs/<containerID>.json.
// Permissions are set to 0600 (owner read/write only) because the file may
// contain resource-usage data worth keeping private.
func Save(rec RunRecord) error {
	dir, err := runsDir()
	if err != nil {
		return fmt.Errorf("store: cannot create runs dir: %w", err)
	}

	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("store: cannot marshal record: %w", err)
	}

	path := filepath.Join(dir, rec.ContainerID+".json")
	return os.WriteFile(path, data, 0600)
}

// Load reads the RunRecord for containerID from disk.
// Returns a descriptive error when the run does not exist.
func Load(containerID string) (RunRecord, error) {
	dir, err := runsDir()
	if err != nil {
		return RunRecord{}, fmt.Errorf("store: cannot access runs dir: %w", err)
	}

	path := filepath.Join(dir, containerID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RunRecord{}, fmt.Errorf("store: no run record found for %q (was 'cspip run' used?)", containerID)
		}
		return RunRecord{}, fmt.Errorf("store: cannot read record: %w", err)
	}

	var rec RunRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return RunRecord{}, fmt.Errorf("store: cannot parse record for %q: %w", containerID, err)
	}
	return rec, nil
}

// List returns the container IDs of all stored run records.
func List() ([]string, error) {
	dir, err := runsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("store: cannot read runs dir: %w", err)
	}

	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && len(name) > 5 && filepath.Ext(name) == ".json" {
			ids = append(ids, name[:len(name)-5])
		}
	}
	return ids, nil
}
