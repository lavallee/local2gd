package gdrive

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// Client wraps Google Drive and Docs API services.
type Client struct {
	drive *drive.Service
	docs  *docs.Service
	ctx   context.Context
}

// NewClient creates a new Google Drive/Docs client from an authorized HTTP client.
func NewClient(ctx context.Context, httpClient *http.Client) (*Client, error) {
	driveSvc, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create Drive service: %w", err)
	}

	docsSvc, err := docs.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create Docs service: %w", err)
	}

	return &Client{
		drive: driveSvc,
		docs:  docsSvc,
		ctx:   ctx,
	}, nil
}

// FileInfo represents a file in Google Drive.
type FileInfo struct {
	ID           string
	Name         string
	MimeType     string
	ModifiedTime time.Time
	MD5Checksum  string
	Parents      []string
}

const googleDocMimeType = "application/vnd.google-apps.document"
const googleFolderMimeType = "application/vnd.google-apps.folder"

// withRetry executes fn with exponential backoff on transient errors.
func withRetry[T any](fn func() (T, error)) (T, error) {
	var result T
	var err error

	for attempt := 0; attempt < 5; attempt++ {
		result, err = fn()
		if err == nil {
			return result, nil
		}

		// Check if retryable (rate limit or server error)
		if !isRetryable(err) {
			return result, err
		}

		delay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
		slog.Debug("Retrying after error", "attempt", attempt+1, "delay", delay, "error", err)
		time.Sleep(delay)
	}

	return result, fmt.Errorf("failed after 5 retries: %w", err)
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Google API rate limit and server errors
	for _, substr := range []string{"429", "500", "502", "503", "504", "rate limit"} {
		if contains(errStr, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
