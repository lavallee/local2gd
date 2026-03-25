package gdrive

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/api/drive/v3"
)

// ListFolder returns all files in a Drive folder.
func (c *Client) ListFolder(folderID string) ([]FileInfo, error) {
	var files []FileInfo
	query := fmt.Sprintf("'%s' in parents and trashed = false", folderID)

	pageToken := ""
	for {
		result, err := withRetry(func() (*drive.FileList, error) {
			call := c.drive.Files.List().
				Q(query).
				Fields("nextPageToken, files(id, name, mimeType, modifiedTime, md5Checksum, parents)").
				PageSize(100)
			if pageToken != "" {
				call = call.PageToken(pageToken)
			}
			return call.Do()
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list folder %s: %w", folderID, err)
		}

		for _, f := range result.Files {
			modTime, _ := time.Parse(time.RFC3339, f.ModifiedTime)
			files = append(files, FileInfo{
				ID:           f.Id,
				Name:         f.Name,
				MimeType:     f.MimeType,
				ModifiedTime: modTime,
				MD5Checksum:  f.Md5Checksum,
				Parents:      f.Parents,
			})
		}

		pageToken = result.NextPageToken
		if pageToken == "" {
			break
		}
	}

	slog.Debug("Listed folder", "folderID", folderID, "fileCount", len(files))
	return files, nil
}

// ListFolderRecursive returns all files in a Drive folder and its subfolders.
// Files are returned with relative paths from the root folder.
type RemoteEntry struct {
	FileInfo
	RelPath string
}

func (c *Client) ListFolderRecursive(folderID string) ([]RemoteEntry, error) {
	return c.listRecursive(folderID, "")
}

func (c *Client) listRecursive(folderID, prefix string) ([]RemoteEntry, error) {
	files, err := c.ListFolder(folderID)
	if err != nil {
		return nil, err
	}

	var entries []RemoteEntry
	for _, f := range files {
		relPath := f.Name
		if prefix != "" {
			relPath = prefix + "/" + f.Name
		}

		if f.MimeType == googleFolderMimeType {
			// Recurse into subfolder
			subEntries, err := c.listRecursive(f.ID, relPath)
			if err != nil {
				return nil, err
			}
			entries = append(entries, subEntries...)
		} else {
			entries = append(entries, RemoteEntry{
				FileInfo: f,
				RelPath:  relPath,
			})
		}
	}

	return entries, nil
}

// ResolvePath resolves a human-readable Drive path to a folder ID.
// E.g., "Notes/Projects" → folder ID.
// An empty path or "My Drive" resolves to the root.
func (c *Client) ResolvePath(path string) (string, error) {
	if path == "" || path == "My Drive" || path == "/" {
		return "root", nil
	}

	segments := strings.Split(strings.Trim(path, "/"), "/")
	currentID := "root"

	for _, segment := range segments {
		query := fmt.Sprintf("'%s' in parents and name = '%s' and mimeType = '%s' and trashed = false",
			currentID, escapeQuery(segment), googleFolderMimeType)

		result, err := withRetry(func() (*drive.FileList, error) {
			return c.drive.Files.List().
				Q(query).
				Fields("files(id, name)").
				PageSize(1).
				Do()
		})
		if err != nil {
			return "", fmt.Errorf("failed to resolve path segment '%s': %w", segment, err)
		}

		if len(result.Files) == 0 {
			return "", fmt.Errorf("folder not found: '%s' (full path: '%s')", segment, path)
		}

		currentID = result.Files[0].Id
	}

	slog.Debug("Resolved path", "path", path, "folderID", currentID)
	return currentID, nil
}

// CreateFolder creates a folder in Drive and returns its ID.
func (c *Client) CreateFolder(parentID, name string) (string, error) {
	f, err := withRetry(func() (*drive.File, error) {
		return c.drive.Files.Create(&drive.File{
			Name:     name,
			MimeType: googleFolderMimeType,
			Parents:  []string{parentID},
		}).Fields("id").Do()
	})
	if err != nil {
		return "", fmt.Errorf("failed to create folder '%s': %w", name, err)
	}
	return f.Id, nil
}

// ResolveOrCreatePath resolves a path, creating missing folders as needed.
func (c *Client) ResolveOrCreatePath(path string) (string, error) {
	if path == "" || path == "My Drive" || path == "/" {
		return "root", nil
	}

	segments := strings.Split(strings.Trim(path, "/"), "/")
	currentID := "root"

	for _, segment := range segments {
		query := fmt.Sprintf("'%s' in parents and name = '%s' and mimeType = '%s' and trashed = false",
			currentID, escapeQuery(segment), googleFolderMimeType)

		result, err := withRetry(func() (*drive.FileList, error) {
			return c.drive.Files.List().
				Q(query).
				Fields("files(id)").
				PageSize(1).
				Do()
		})
		if err != nil {
			return "", fmt.Errorf("failed to resolve path segment '%s': %w", segment, err)
		}

		if len(result.Files) > 0 {
			currentID = result.Files[0].Id
		} else {
			// Create the missing folder
			currentID, err = c.CreateFolder(currentID, segment)
			if err != nil {
				return "", err
			}
			slog.Debug("Created folder", "name", segment, "id", currentID)
		}
	}

	return currentID, nil
}

// GetFileInfo retrieves metadata for a single file.
func (c *Client) GetFileInfo(fileID string) (FileInfo, error) {
	f, err := withRetry(func() (*drive.File, error) {
		return c.drive.Files.Get(fileID).
			Fields("id, name, mimeType, modifiedTime, md5Checksum, webViewLink").
			Do()
	})
	if err != nil {
		return FileInfo{}, fmt.Errorf("failed to get file %s: %w", fileID, err)
	}

	modTime, _ := time.Parse(time.RFC3339, f.ModifiedTime)
	return FileInfo{
		ID:           f.Id,
		Name:         f.Name,
		MimeType:     f.MimeType,
		ModifiedTime: modTime,
		MD5Checksum:  f.Md5Checksum,
	}, nil
}

// escapeQuery escapes single quotes in Drive API query strings.
func escapeQuery(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}
