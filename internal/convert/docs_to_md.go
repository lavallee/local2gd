package convert

import (
	"bytes"
	"regexp"
	"strings"
)

var (
	// Match base64 data URI image references
	base64ImageRe = regexp.MustCompile(`!\[([^\]]*)\]\(data:[^)]+\)`)
	// Match excessive blank lines (3+ consecutive)
	excessiveBlanksRe = regexp.MustCompile(`\n{4,}`)
	// Match trailing whitespace on lines
	trailingSpaceRe = regexp.MustCompile(`[ \t]+\n`)
	// Match headings without blank line before them (except at start of file)
	headingNeedsPrecedingBlank = regexp.MustCompile(`([^\n])\n(#{1,6} )`)
	// Match headings without blank line after them
	headingNeedsFollowingBlank = regexp.MustCompile(`(#{1,6} [^\n]+)\n([^\n#])`)
)

// PostProcessExport cleans up Google's native markdown export.
// Handles known issues: whitespace normalization, image cleanup, heading spacing.
func PostProcessExport(rawMD []byte) []byte {
	result := rawMD

	// Normalize line endings to LF
	result = bytes.ReplaceAll(result, []byte("\r\n"), []byte("\n"))
	result = bytes.ReplaceAll(result, []byte("\r"), []byte("\n"))

	// Replace base64 data URI images with placeholder comments
	result = base64ImageRe.ReplaceAll(result, []byte("<!-- image: $1 (stripped from Google Docs export) -->"))

	// Remove trailing whitespace on each line
	result = trailingSpaceRe.ReplaceAll(result, []byte("\n"))

	// Ensure blank line before headings
	result = headingNeedsPrecedingBlank.ReplaceAll(result, []byte("$1\n\n$2"))

	// Ensure blank line after headings
	result = headingNeedsFollowingBlank.ReplaceAll(result, []byte("$1\n\n$2"))

	// Collapse excessive blank lines to max 2
	result = excessiveBlanksRe.ReplaceAll(result, []byte("\n\n\n"))

	// Trim leading/trailing whitespace from the document
	result = bytes.TrimSpace(result)

	// Ensure file ends with a single newline
	result = append(result, '\n')

	return result
}

// StripGoogleComments removes Google Docs comment markers from exported markdown.
// Google Docs comments don't have a standard markdown representation,
// but sometimes appear as bracketed annotations.
func StripGoogleComments(md []byte) []byte {
	// Google sometimes exports comments as [a], [b], etc. with footnote-style references
	commentRefRe := regexp.MustCompile(`\[([a-z])\]`)
	return commentRefRe.ReplaceAll(md, []byte(""))
}

// NormalizeListMarkers standardizes list bullet characters to dashes.
func NormalizeListMarkers(md string) string {
	lines := strings.Split(md, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		indent := line[:len(line)-len(trimmed)]
		if strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
			lines[i] = indent + "- " + trimmed[2:]
		}
	}
	return strings.Join(lines, "\n")
}
