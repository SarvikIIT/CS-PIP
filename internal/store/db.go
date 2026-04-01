package store

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

// effectiveHomeDir returns the home directory of the user who invoked the
// process.  When the binary is run via sudo, os.UserHomeDir() returns
// /root, so the run record would be saved there and become invisible to
// the unprivileged user who later runs `cspip report`.  Checking
// SUDO_USER lets us save (and find) records in the invoking user's home.
func effectiveHomeDir() (string, error) {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		u, err := user.Lookup(sudoUser)
		if err == nil {
			return u.HomeDir, nil
		}
	}
	return os.UserHomeDir()
}

// runsDir returns (and creates if necessary) the directory used to store
// per-container run records: ~/.cspip/runs/
func runsDir() (string, error) {
	home, err := effectiveHomeDir()
	if err != nil {
		return "", fmt.Errorf("store: cannot determine home dir: %w", err)
	}
	dir := filepath.Join(home, ".cspip", "runs")
	return dir, os.MkdirAll(dir, 0755)
}

// Save persists rec to ~/.cspip/runs/<containerID>.json (using the real
// invoking user's home when run via sudo).
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
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	// When invoked via sudo, chown the file (and its parent dir) back to
	// the real user so that `cspip report` (run without sudo) can read it.
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		if u, err := user.Lookup(sudoUser); err == nil {
			uid, gid := 0, 0
			fmt.Sscanf(u.Uid, "%d", &uid)
			fmt.Sscanf(u.Gid, "%d", &gid)
			_ = os.Chown(dir, uid, gid)
			_ = os.Chown(path, uid, gid)
		}
	}
	return nil
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
