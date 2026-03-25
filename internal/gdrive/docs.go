package gdrive

import (
	"fmt"
	"io"
	"log/slog"
	"time"

	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
)

// ExportMarkdown exports a Google Doc as markdown using the Drive API.
func (c *Client) ExportMarkdown(fileID string) ([]byte, error) {
	resp, err := withRetry(func() (*io.ReadCloser, error) {
		res, err := c.drive.Files.Export(fileID, "text/markdown").Download()
		if err != nil {
			return nil, err
		}
		return &res.Body, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to export doc %s as markdown: %w", fileID, err)
	}
	defer (*resp).Close()

	data, err := io.ReadAll(*resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read export response: %w", err)
	}

	slog.Debug("Exported markdown", "fileID", fileID, "bytes", len(data))
	return data, nil
}

// CreateDoc creates a new Google Doc in the specified folder with the given title,
// then applies batchUpdate requests to populate its content.
func (c *Client) CreateDoc(folderID, title string, requests []*docs.Request) (FileInfo, error) {
	// Create blank doc via Drive API (to set parent folder)
	driveFile, err := withRetry(func() (*drive.File, error) {
		return c.drive.Files.Create(&drive.File{
			Name:     title,
			MimeType: googleDocMimeType,
			Parents:  []string{folderID},
		}).Fields("id, name, mimeType, modifiedTime, parents").Do()
	})
	if err != nil {
		return FileInfo{}, fmt.Errorf("failed to create doc '%s': %w", title, err)
	}

	// Apply content via batchUpdate if we have requests
	if len(requests) > 0 {
		if err := c.batchUpdate(driveFile.Id, requests); err != nil {
			// Clean up the empty doc on failure
			_ = c.DeleteFile(driveFile.Id)
			return FileInfo{}, fmt.Errorf("failed to populate doc '%s': %w", title, err)
		}
	}

	modTime, _ := time.Parse(time.RFC3339, driveFile.ModifiedTime)
	info := FileInfo{
		ID:           driveFile.Id,
		Name:         driveFile.Name,
		MimeType:     driveFile.MimeType,
		ModifiedTime: modTime,
		Parents:      driveFile.Parents,
	}

	slog.Debug("Created doc", "title", title, "id", info.ID)
	return info, nil
}

// UpdateDoc replaces the content of an existing Google Doc.
// It first clears all existing content, then applies the new requests.
func (c *Client) UpdateDoc(fileID string, requests []*docs.Request) error {
	// Get current doc to find content length
	doc, err := withRetry(func() (*docs.Document, error) {
		return c.docs.Documents.Get(fileID).Do()
	})
	if err != nil {
		return fmt.Errorf("failed to get doc %s: %w", fileID, err)
	}

	// Build delete request if doc has content beyond the trailing newline
	var allRequests []*docs.Request
	endIndex := doc.Body.Content[len(doc.Body.Content)-1].EndIndex
	if endIndex > 1 {
		allRequests = append(allRequests, &docs.Request{
			DeleteContentRange: &docs.DeleteContentRangeRequest{
				Range: &docs.Range{
					StartIndex: 1,
					EndIndex:   endIndex - 1,
				},
			},
		})
	}

	// Append new content requests
	allRequests = append(allRequests, requests...)

	if len(allRequests) > 0 {
		if err := c.batchUpdate(fileID, allRequests); err != nil {
			return fmt.Errorf("failed to update doc %s: %w", fileID, err)
		}
	}

	slog.Debug("Updated doc", "fileID", fileID)
	return nil
}

// UpdateDocTitle updates the title of a Google Doc via the Drive API.
func (c *Client) UpdateDocTitle(fileID, title string) error {
	_, err := withRetry(func() (*drive.File, error) {
		return c.drive.Files.Update(fileID, &drive.File{
			Name: title,
		}).Fields("id").Do()
	})
	if err != nil {
		return fmt.Errorf("failed to update doc title: %w", err)
	}
	return nil
}

// DeleteFile moves a file to the Drive trash.
func (c *Client) DeleteFile(fileID string) error {
	_, err := withRetry(func() (*drive.File, error) {
		return c.drive.Files.Update(fileID, &drive.File{
			Trashed: true,
		}).Fields("id").Do()
	})
	if err != nil {
		return fmt.Errorf("failed to trash file %s: %w", fileID, err)
	}

	slog.Debug("Trashed file", "fileID", fileID)
	return nil
}

func (c *Client) batchUpdate(docID string, requests []*docs.Request) error {
	_, err := withRetry(func() (*docs.BatchUpdateDocumentResponse, error) {
		return c.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}).Do()
	})
	return err
}
