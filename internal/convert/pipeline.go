package convert

import (
	"fmt"
	"path/filepath"

	"github.com/lavallee/local2gd/internal/gdrive"
)

// CreateDocFromMarkdown converts markdown content and creates a Google Doc.
// Frontmatter is stripped before conversion and returned separately.
// Returns the created file info and the stripped frontmatter (if any).
func CreateDocFromMarkdown(client *gdrive.Client, folderID string, filename string, mdContent []byte) (gdrive.FileInfo, []byte, error) {
	body, frontmatter := StripFrontmatter(mdContent)

	requests, title, err := MarkdownToDocs(body)
	if err != nil {
		return gdrive.FileInfo{}, nil, fmt.Errorf("failed to convert markdown: %w", err)
	}

	if title == "" {
		title = TitleFromFilename(filepath.Base(filename))
	}

	info, err := client.CreateDoc(folderID, title, requests)
	if err != nil {
		return gdrive.FileInfo{}, nil, fmt.Errorf("failed to create doc '%s': %w", title, err)
	}

	return info, frontmatter, nil
}

// UpdateDocFromMarkdown converts markdown content and updates an existing Google Doc.
// Frontmatter is stripped before conversion and returned separately.
func UpdateDocFromMarkdown(client *gdrive.Client, fileID string, filename string, mdContent []byte) ([]byte, error) {
	body, frontmatter := StripFrontmatter(mdContent)

	requests, title, err := MarkdownToDocs(body)
	if err != nil {
		return nil, fmt.Errorf("failed to convert markdown: %w", err)
	}

	if err := client.UpdateDoc(fileID, requests); err != nil {
		return nil, fmt.Errorf("failed to update doc: %w", err)
	}

	// Update title if we extracted one
	if title == "" {
		title = TitleFromFilename(filepath.Base(filename))
	}
	if err := client.UpdateDocTitle(fileID, title); err != nil {
		// Non-fatal — content is already updated
		fmt.Printf("Warning: failed to update doc title: %v\n", err)
	}

	return frontmatter, nil
}

// ExportDocAsMarkdown exports a Google Doc and post-processes the markdown.
// If frontmatter is provided, it is prepended to the exported content.
func ExportDocAsMarkdown(client *gdrive.Client, fileID string, frontmatter []byte) ([]byte, error) {
	raw, err := client.ExportMarkdown(fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to export doc: %w", err)
	}

	cleaned := PostProcessExport(raw)

	// Re-attach frontmatter if provided
	if frontmatter != nil {
		cleaned = AttachFrontmatter(cleaned, frontmatter)
	}

	return cleaned, nil
}
