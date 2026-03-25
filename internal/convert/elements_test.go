package convert

import (
	"testing"

	"google.golang.org/api/docs/v1"
)

func TestMarkdownToDocs_SimpleHeading(t *testing.T) {
	requests, title, err := MarkdownToDocs([]byte("# Hello World\n"))
	if err != nil {
		t.Fatal(err)
	}
	if title != "Hello World" {
		t.Errorf("expected title 'Hello World', got %q", title)
	}
	if len(requests) < 2 {
		t.Fatalf("expected at least 2 requests (insert + heading style), got %d", len(requests))
	}

	// First request should be InsertText
	if requests[0].InsertText == nil {
		t.Fatal("first request should be InsertText")
	}

	// Should have a heading style request
	hasHeadingStyle := false
	for _, r := range requests {
		if r.UpdateParagraphStyle != nil && r.UpdateParagraphStyle.ParagraphStyle.NamedStyleType == "HEADING_1" {
			hasHeadingStyle = true
		}
	}
	if !hasHeadingStyle {
		t.Error("expected HEADING_1 paragraph style")
	}
}

func TestMarkdownToDocs_BoldText(t *testing.T) {
	requests, _, err := MarkdownToDocs([]byte("This is **bold** text.\n"))
	if err != nil {
		t.Fatal(err)
	}

	hasBold := false
	for _, r := range requests {
		if r.UpdateTextStyle != nil && r.UpdateTextStyle.TextStyle.Bold {
			hasBold = true
		}
	}
	if !hasBold {
		t.Error("expected bold text style")
	}
}

func TestMarkdownToDocs_ItalicText(t *testing.T) {
	requests, _, err := MarkdownToDocs([]byte("This is *italic* text.\n"))
	if err != nil {
		t.Fatal(err)
	}

	hasItalic := false
	for _, r := range requests {
		if r.UpdateTextStyle != nil && r.UpdateTextStyle.TextStyle.Italic {
			hasItalic = true
		}
	}
	if !hasItalic {
		t.Error("expected italic text style")
	}
}

func TestMarkdownToDocs_BoldItalic(t *testing.T) {
	requests, _, err := MarkdownToDocs([]byte("This is ***bold italic*** text.\n"))
	if err != nil {
		t.Fatal(err)
	}

	hasBoldItalic := false
	for _, r := range requests {
		if r.UpdateTextStyle != nil && r.UpdateTextStyle.TextStyle.Bold && r.UpdateTextStyle.TextStyle.Italic {
			hasBoldItalic = true
		}
	}
	if !hasBoldItalic {
		t.Error("expected bold+italic text style")
	}
}

func TestMarkdownToDocs_Link(t *testing.T) {
	requests, _, err := MarkdownToDocs([]byte("[click](https://example.com)\n"))
	if err != nil {
		t.Fatal(err)
	}

	hasLink := false
	for _, r := range requests {
		if r.UpdateTextStyle != nil && r.UpdateTextStyle.TextStyle.Link != nil {
			if r.UpdateTextStyle.TextStyle.Link.Url == "https://example.com" {
				hasLink = true
			}
		}
	}
	if !hasLink {
		t.Error("expected link style with URL https://example.com")
	}
}

func TestMarkdownToDocs_MultipleHeadings(t *testing.T) {
	md := "# Title\n\n## Section\n\n### Subsection\n"
	requests, _, err := MarkdownToDocs([]byte(md))
	if err != nil {
		t.Fatal(err)
	}

	styles := map[string]bool{}
	for _, r := range requests {
		if r.UpdateParagraphStyle != nil {
			styles[r.UpdateParagraphStyle.ParagraphStyle.NamedStyleType] = true
		}
	}

	for _, expected := range []string{"HEADING_1", "HEADING_2", "HEADING_3"} {
		if !styles[expected] {
			t.Errorf("expected %s style", expected)
		}
	}
}

func TestMarkdownToDocs_UnorderedList(t *testing.T) {
	md := "- item one\n- item two\n"
	requests, _, err := MarkdownToDocs([]byte(md))
	if err != nil {
		t.Fatal(err)
	}

	// Should have InsertText with bullet characters
	if requests[0].InsertText == nil {
		t.Fatal("expected InsertText request")
	}
	text := requests[0].InsertText.Text
	if !containsString(text, "•") {
		t.Errorf("expected bullet character in text, got: %q", text)
	}
}

func TestMarkdownToDocs_OrderedList(t *testing.T) {
	md := "1. first\n2. second\n"
	requests, _, err := MarkdownToDocs([]byte(md))
	if err != nil {
		t.Fatal(err)
	}

	if requests[0].InsertText == nil {
		t.Fatal("expected InsertText request")
	}
	text := requests[0].InsertText.Text
	if !containsString(text, "1.") {
		t.Errorf("expected '1.' in text, got: %q", text)
	}
}

func TestMarkdownToDocs_EmptyContent(t *testing.T) {
	requests, title, err := MarkdownToDocs([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if title != "" {
		t.Errorf("expected empty title, got %q", title)
	}
	if len(requests) != 0 {
		t.Errorf("expected 0 requests for empty content, got %d", len(requests))
	}
}

func TestMarkdownToDocs_ComplexDocument(t *testing.T) {
	md := `# Project Notes

## Overview

This is a **complex** document with *various* formatting.

### Features

- Feature one with **bold**
- Feature two with [a link](https://example.com)
- Feature three

### Steps

1. First step
2. Second step
3. Third step

---

## Conclusion

That's ***all*** folks.
`
	requests, title, err := MarkdownToDocs([]byte(md))
	if err != nil {
		t.Fatal(err)
	}

	if title != "Project Notes" {
		t.Errorf("expected title 'Project Notes', got %q", title)
	}

	// Verify we have a variety of request types
	var insertCount, paraStyleCount, textStyleCount int
	for _, r := range requests {
		if r.InsertText != nil {
			insertCount++
		}
		if r.UpdateParagraphStyle != nil {
			paraStyleCount++
		}
		if r.UpdateTextStyle != nil {
			textStyleCount++
		}
	}

	if insertCount < 1 {
		t.Error("expected at least 1 InsertText request")
	}
	if paraStyleCount < 3 {
		t.Errorf("expected at least 3 paragraph style requests, got %d", paraStyleCount)
	}
	if textStyleCount < 3 {
		t.Errorf("expected at least 3 text style requests, got %d", textStyleCount)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Verify all requests have valid index ranges (using UTF-16 length).
func TestMarkdownToDocs_ValidIndices(t *testing.T) {
	md := "# Hello\n\nSome **bold** and *italic* [link](http://x.com).\n\n- item\n"
	requests, _, err := MarkdownToDocs([]byte(md))
	if err != nil {
		t.Fatal(err)
	}

	var totalTextLen int64
	for _, r := range requests {
		if r.InsertText != nil {
			totalTextLen += utf16Len(r.InsertText.Text)
		}
	}

	for i, r := range requests {
		checkRange := func(rng *docs.Range, kind string) {
			if rng.StartIndex < 1 {
				t.Errorf("request %d (%s): start index %d < 1", i, kind, rng.StartIndex)
			}
			if rng.EndIndex <= rng.StartIndex {
				t.Errorf("request %d (%s): end index %d <= start %d", i, kind, rng.EndIndex, rng.StartIndex)
			}
			if rng.EndIndex > totalTextLen+1 {
				t.Errorf("request %d (%s): end index %d exceeds text length %d", i, kind, rng.EndIndex, totalTextLen)
			}
		}

		if r.UpdateParagraphStyle != nil {
			checkRange(r.UpdateParagraphStyle.Range, "paragraph style")
		}
		if r.UpdateTextStyle != nil {
			checkRange(r.UpdateTextStyle.Range, "text style")
		}
	}
}

// Test with multi-byte unicode characters (em dashes, smart quotes, etc.)
func TestMarkdownToDocs_UnicodeCharacters(t *testing.T) {
	md := "# Healthcare \u2014 Provider Framework\n\nThis is a \u201cquoted\u201d section with an em dash \u2014 and more.\n\n**H\u00e9llo** w\u00f6rld with \u00e0ccents.\n"
	requests, title, err := MarkdownToDocs([]byte(md))
	if err != nil {
		t.Fatal(err)
	}

	if title != "Healthcare \u2014 Provider Framework" {
		t.Errorf("unexpected title: %q", title)
	}

	// Verify indices are valid (UTF-16 based)
	var totalUTF16Len int64
	for _, r := range requests {
		if r.InsertText != nil {
			totalUTF16Len += utf16Len(r.InsertText.Text)
		}
	}

	for i, r := range requests {
		check := func(rng *docs.Range, kind string) {
			if rng.EndIndex > totalUTF16Len+1 {
				t.Errorf("request %d (%s): end index %d exceeds UTF-16 text length %d",
					i, kind, rng.EndIndex, totalUTF16Len)
			}
		}
		if r.UpdateParagraphStyle != nil {
			check(r.UpdateParagraphStyle.Range, "paragraph style")
		}
		if r.UpdateTextStyle != nil {
			check(r.UpdateTextStyle.Range, "text style")
		}
	}
}

func TestUTF16Len(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"hello", 5},
		{"\u2014", 1},                    // em dash: U+2014, BMP, 1 UTF-16 unit
		{"\u201cquoted\u201d", 8},        // smart quotes: BMP
		{"H\u00e9llo", 5},                // accented: BMP
		{"\U0001F389", 2},                // emoji: U+1F389, supplementary plane, 2 UTF-16 units
		{"hello \U0001F389 world", 14},   // 6 + 2 + 6
	}

	for _, tt := range tests {
		result := utf16Len(tt.input)
		if result != tt.expected {
			t.Errorf("utf16Len(%q) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}
