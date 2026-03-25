package convert

import (
	"testing"
)

func TestStripFrontmatter_WithFrontmatter(t *testing.T) {
	input := []byte("---\ntitle: Hello\ntags: [a, b]\n---\n\n# Content\n\nBody text.\n")
	body, fm := StripFrontmatter(input)

	if fm == nil {
		t.Fatal("expected frontmatter to be extracted")
	}
	if string(body) != "# Content\n\nBody text.\n" {
		t.Errorf("unexpected body: %q", string(body))
	}
	if string(fm) != "---\ntitle: Hello\ntags: [a, b]\n---\n" {
		t.Errorf("unexpected frontmatter: %q", string(fm))
	}
}

func TestStripFrontmatter_NoFrontmatter(t *testing.T) {
	input := []byte("# Just a heading\n\nNo frontmatter here.\n")
	body, fm := StripFrontmatter(input)

	if fm != nil {
		t.Error("expected no frontmatter")
	}
	if string(body) != string(input) {
		t.Error("body should be unchanged")
	}
}

func TestStripFrontmatter_Empty(t *testing.T) {
	body, fm := StripFrontmatter([]byte(""))
	if fm != nil {
		t.Error("expected no frontmatter for empty input")
	}
	if string(body) != "" {
		t.Error("body should be empty")
	}
}

func TestStripFrontmatter_EmptyFrontmatter(t *testing.T) {
	input := []byte("---\n---\n\n# Content\n")
	body, fm := StripFrontmatter(input)

	if fm == nil {
		t.Fatal("expected frontmatter")
	}
	if string(body) != "# Content\n" {
		t.Errorf("unexpected body: %q", string(body))
	}
}

func TestAttachFrontmatter_WithFrontmatter(t *testing.T) {
	body := []byte("# Content\n\nBody.\n")
	fm := []byte("---\ntitle: Hello\n---\n")

	result := AttachFrontmatter(body, fm)
	expected := "---\ntitle: Hello\n---\n\n# Content\n\nBody.\n"
	if string(result) != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, string(result))
	}
}

func TestAttachFrontmatter_NilFrontmatter(t *testing.T) {
	body := []byte("# Content\n")
	result := AttachFrontmatter(body, nil)
	if string(result) != "# Content\n" {
		t.Errorf("expected body unchanged, got %q", string(result))
	}
}

func TestStripAndReattach_RoundTrip(t *testing.T) {
	original := []byte("---\ntitle: Test\ndate: 2026-03-25\n---\n\n# My Document\n\nContent here.\n")
	body, fm := StripFrontmatter(original)

	if fm == nil {
		t.Fatal("expected frontmatter")
	}

	reassembled := AttachFrontmatter(body, fm)
	if string(reassembled) != string(original) {
		t.Errorf("round-trip failed.\noriginal:\n%s\nreassembled:\n%s", string(original), string(reassembled))
	}
}
