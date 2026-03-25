package convert

import (
	"bytes"
)

var frontmatterDelimiter = []byte("---")

// StripFrontmatter separates YAML frontmatter from the markdown body.
// Returns the body (without frontmatter) and the raw frontmatter bytes (including delimiters).
// If no frontmatter is found, returns the original content and nil.
func StripFrontmatter(md []byte) (body []byte, frontmatter []byte) {
	trimmed := bytes.TrimLeft(md, " \t\n\r")
	if !bytes.HasPrefix(trimmed, frontmatterDelimiter) {
		return md, nil
	}

	// Find the closing delimiter
	rest := trimmed[len(frontmatterDelimiter):]
	// Must have a newline after the opening delimiter
	nlIdx := bytes.IndexByte(rest, '\n')
	if nlIdx < 0 {
		return md, nil
	}

	// Check that what's between --- and \n is only whitespace
	between := rest[:nlIdx]
	if len(bytes.TrimSpace(between)) > 0 {
		return md, nil
	}

	rest = rest[nlIdx+1:]

	// Find closing ---
	closeIdx := bytes.Index(rest, frontmatterDelimiter)
	if closeIdx < 0 {
		return md, nil
	}

	// Verify closing delimiter is at start of a line
	if closeIdx > 0 && rest[closeIdx-1] != '\n' {
		return md, nil
	}

	// Everything up to and including the closing delimiter line
	afterClose := rest[closeIdx+len(frontmatterDelimiter):]
	closeLineEnd := bytes.IndexByte(afterClose, '\n')
	if closeLineEnd < 0 {
		// Frontmatter is the entire file
		frontmatter = trimmed
		return nil, frontmatter
	}

	fmEnd := closeIdx + len(frontmatterDelimiter) + closeLineEnd + 1
	frontmatter = trimmed[:len(frontmatterDelimiter)+nlIdx+1+fmEnd]
	body = rest[fmEnd:]

	// Trim leading whitespace from body
	body = bytes.TrimLeft(body, "\n")

	return body, frontmatter
}

// AttachFrontmatter prepends frontmatter to a markdown body.
// If frontmatter is nil, returns the body as-is.
func AttachFrontmatter(body []byte, frontmatter []byte) []byte {
	if frontmatter == nil || len(frontmatter) == 0 {
		return body
	}

	var result bytes.Buffer
	result.Write(frontmatter)
	if !bytes.HasSuffix(frontmatter, []byte("\n")) {
		result.WriteByte('\n')
	}
	result.WriteByte('\n')
	result.Write(body)
	return result.Bytes()
}
