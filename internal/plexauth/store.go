package plexauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Credentials are the persisted device identity and token.
type Credentials struct {
	ClientID string `json:"client_id"`
	Token    string `json:"token"`
}

// credentialsPath is <user-config-dir>/plexmatch-generator/auth.json
// (e.g. ~/.config/plexmatch-generator/auth.json on Linux).
func credentialsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating config directory: %w", err)
	}
	return filepath.Join(dir, "plexmatch-generator", "auth.json"), nil
}

// LoadCredentials reads the cached credentials. A missing file is not an error;
// it returns a zero Credentials value.
func LoadCredentials() (Credentials, error) {
	path, err := credentialsPath()
	if err != nil {
		return Credentials{}, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Credentials{}, nil
	}
	if err != nil {
		return Credentials{}, fmt.Errorf("reading credentials: %w", err)
	}

	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return Credentials{}, fmt.Errorf("parsing credentials: %w", err)
	}
	return c, nil
}

// SaveCredentials writes the credentials with owner-only permissions.
func SaveCredentials(c Credentials) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding credentials: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}
	return nil
}

// ClearCredentials removes the cached credentials file. A missing file is fine.
func ClearCredentials() error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing credentials: %w", err)
	}
	return nil
}
