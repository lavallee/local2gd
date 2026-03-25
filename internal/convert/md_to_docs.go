package convert

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// md is the configured goldmark parser with GFM extensions.
var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM, // tables, strikethrough, task lists, autolinks
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
)

// ParseMarkdown parses markdown content into a goldmark AST.
func ParseMarkdown(source []byte) ast.Node {
	reader := text.NewReader(source)
	return md.Parser().Parse(reader)
}

// ExtractTitle returns the text content of the first H1 heading in the markdown,
// or an empty string if no H1 is found.
func ExtractTitle(doc ast.Node, source []byte) string {
	var title string
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if heading, ok := n.(*ast.Heading); ok && heading.Level == 1 {
			title = extractTextContent(heading, source)
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return title
}

// extractTextContent returns the concatenated text content of a node and its children.
func extractTextContent(n ast.Node, source []byte) string {
	var buf bytes.Buffer
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if t, ok := child.(*ast.Text); ok {
			buf.Write(t.Segment.Value(source))
			if t.HardLineBreak() || t.SoftLineBreak() {
				buf.WriteByte('\n')
			}
		} else {
			// Recurse into inline elements (bold, italic, etc.)
			buf.WriteString(extractTextContent(child, source))
		}
	}
	return buf.String()
}

// TitleFromFilename derives a human-readable title from a markdown filename.
// "design-notes.md" → "Design Notes"
func TitleFromFilename(filename string) string {
	// Strip .md extension
	name := filename
	if len(name) > 3 && name[len(name)-3:] == ".md" {
		name = name[:len(name)-3]
	}

	// Replace hyphens and underscores with spaces, capitalize words
	var result bytes.Buffer
	capitalizeNext := true
	for _, ch := range name {
		if ch == '-' || ch == '_' {
			result.WriteByte(' ')
			capitalizeNext = true
		} else if capitalizeNext && ch >= 'a' && ch <= 'z' {
			result.WriteRune(ch - 32) // to uppercase
			capitalizeNext = false
		} else {
			result.WriteRune(ch)
			if ch == ' ' {
				capitalizeNext = true
			} else {
				capitalizeNext = false
			}
		}
	}
	return result.String()
}
