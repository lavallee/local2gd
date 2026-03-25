package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"golang.org/x/oauth2"
)

// tokenPath returns the path to the stored OAuth token.
func tokenPath() (string, error) {
	return xdg.DataFile("local2gd/token.json")
}

// SaveToken writes the OAuth token to the XDG data directory with 0600 permissions.
func SaveToken(token *oauth2.Token) error {
	path, err := tokenPath()
	if err != nil {
		return fmt.Errorf("failed to determine token path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write token: %w", err)
	}

	slog.Debug("Token saved", "path", path)
	return nil
}

// LoadToken reads the stored OAuth token from the XDG data directory.
// Returns an error if no token is stored.
func LoadToken() (*oauth2.Token, error) {
	path, err := tokenPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine token path: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not authenticated — run `local2gd auth` first")
		}
		return nil, fmt.Errorf("failed to read token: %w", err)
	}

	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token (may be corrupted — run `local2gd auth` to re-authenticate): %w", err)
	}

	return &token, nil
}

// HasToken returns true if a stored token exists.
func HasToken() bool {
	path, err := tokenPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Client returns an authorized HTTP client using the stored token.
// The token is automatically refreshed when expired, and the new token is persisted.
func Client(ctx context.Context) (*http.Client, error) {
	token, err := LoadToken()
	if err != nil {
		return nil, err
	}

	client, err := TokenClient(ctx, token, func(newToken *oauth2.Token) {
		slog.Debug("Token refreshed, saving...")
		if err := SaveToken(newToken); err != nil {
			slog.Error("Failed to save refreshed token", "error", err)
		}
	})
	if err != nil {
		return nil, err
	}

	return client, nil
}
