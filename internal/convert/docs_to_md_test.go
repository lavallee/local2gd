package convert

import (
	"testing"
)

func TestPostProcessExport_NormalizesLineEndings(t *testing.T) {
	input := []byte("Hello\r\nWorld\r\n")
	result := PostProcessExport(input)
	if string(result) != "Hello\nWorld\n" {
		t.Errorf("expected normalized line endings, got %q", string(result))
	}
}

func TestPostProcessExport_StripsBase64Images(t *testing.T) {
	input := []byte("Text before\n\n![alt text](data:image/png;base64,iVBORw0KGgoAAAANSUhEU...)\n\nText after")
	result := PostProcessExport(input)
	expected := "Text before\n\n<!-- image: alt text (stripped from Google Docs export) -->\n\nText after\n"
	if string(result) != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, string(result))
	}
}

func TestPostProcessExport_RemovesTrailingWhitespace(t *testing.T) {
	input := []byte("Hello   \nWorld\t\n")
	result := PostProcessExport(input)
	if string(result) != "Hello\nWorld\n" {
		t.Errorf("expected no trailing whitespace, got %q", string(result))
	}
}

func TestPostProcessExport_CollapsesExcessiveBlanks(t *testing.T) {
	input := []byte("Hello\n\n\n\n\n\nWorld")
	result := PostProcessExport(input)
	if string(result) != "Hello\n\n\nWorld\n" {
		t.Errorf("expected max 2 blank lines, got %q", string(result))
	}
}

func TestPostProcessExport_EnsuresHeadingSpacing(t *testing.T) {
	input := []byte("Some text\n## Heading\nMore text")
	result := PostProcessExport(input)
	expected := "Some text\n\n## Heading\n\nMore text\n"
	if string(result) != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, string(result))
	}
}

func TestPostProcessExport_EndsWithNewline(t *testing.T) {
	input := []byte("Hello World")
	result := PostProcessExport(input)
	if result[len(result)-1] != '\n' {
		t.Error("expected file to end with newline")
	}
}

func TestPostProcessExport_EmptyInput(t *testing.T) {
	result := PostProcessExport([]byte(""))
	if string(result) != "\n" {
		t.Errorf("expected single newline for empty input, got %q", string(result))
	}
}

func TestPostProcessExport_AlreadyClean(t *testing.T) {
	input := []byte("# Title\n\nClean content.\n")
	result := PostProcessExport(input)
	if string(result) != "# Title\n\nClean content.\n" {
		t.Errorf("clean content should pass through, got %q", string(result))
	}
}

func TestNormalizeListMarkers(t *testing.T) {
	input := "* item one\n+ item two\n- item three\n"
	result := NormalizeListMarkers(input)
	expected := "- item one\n- item two\n- item three\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}
