package convert

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lavallee/local2gd/internal/gdrive"
)

// docNameFromFilename returns the Google Doc name derived from a local filename.
// "design-notes.md" → "design-notes"
// "subdir/my-file.md" → "my-file"
// This ensures the Drive filename matches the local filename for round-tripping.
func docNameFromFilename(filename string) string {
	base := filepath.Base(filename)
	return strings.TrimSuffix(base, ".md")
}

// CreateDocFromMarkdown converts markdown content and creates a Google Doc.
// The Doc is named after the local filename (not the H1 heading) to ensure
// round-trip filename consistency.
// Returns the created file info and the stripped frontmatter (if any).
func CreateDocFromMarkdown(client *gdrive.Client, folderID string, filename string, mdContent []byte) (gdrive.FileInfo, []byte, error) {
	body, frontmatter := StripFrontmatter(mdContent)

	requests, _, err := MarkdownToDocs(body)
	if err != nil {
		return gdrive.FileInfo{}, nil, fmt.Errorf("failed to convert markdown: %w", err)
	}

	docName := docNameFromFilename(filename)

	info, err := client.CreateDoc(folderID, docName, requests)
	if err != nil {
		return gdrive.FileInfo{}, nil, fmt.Errorf("failed to create doc '%s': %w", docName, err)
	}

	return info, frontmatter, nil
}

// UpdateDocFromMarkdown converts markdown content and updates an existing Google Doc.
// Frontmatter is stripped before conversion and returned separately.
func UpdateDocFromMarkdown(client *gdrive.Client, fileID string, filename string, mdContent []byte) ([]byte, error) {
	body, frontmatter := StripFrontmatter(mdContent)

	requests, _, err := MarkdownToDocs(body)
	if err != nil {
		return nil, fmt.Errorf("failed to convert markdown: %w", err)
	}

	if err := client.UpdateDoc(fileID, requests); err != nil {
		return nil, fmt.Errorf("failed to update doc: %w", err)
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
