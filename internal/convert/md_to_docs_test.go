package convert

import (
	"testing"

	"github.com/yuin/goldmark/ast"
)

func TestParseMarkdown_Headings(t *testing.T) {
	source := []byte("# Title\n\n## Subtitle\n\n### Third\n")
	doc := ParseMarkdown(source)

	headings := collectNodes(doc, ast.KindHeading)
	if len(headings) != 3 {
		t.Fatalf("expected 3 headings, got %d", len(headings))
	}

	levels := []int{1, 2, 3}
	for i, h := range headings {
		heading := h.(*ast.Heading)
		if heading.Level != levels[i] {
			t.Errorf("heading %d: expected level %d, got %d", i, levels[i], heading.Level)
		}
	}
}

func TestParseMarkdown_Paragraph(t *testing.T) {
	source := []byte("Hello world.\n")
	doc := ParseMarkdown(source)

	paras := collectNodes(doc, ast.KindParagraph)
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paras))
	}
}

func TestParseMarkdown_Bold(t *testing.T) {
	source := []byte("This is **bold** text.\n")
	doc := ParseMarkdown(source)

	emphases := collectNodes(doc, ast.KindEmphasis)
	if len(emphases) != 1 {
		t.Fatalf("expected 1 emphasis node, got %d", len(emphases))
	}
	em := emphases[0].(*ast.Emphasis)
	if em.Level != 2 {
		t.Errorf("expected emphasis level 2 (bold), got %d", em.Level)
	}
}

func TestParseMarkdown_Italic(t *testing.T) {
	source := []byte("This is *italic* text.\n")
	doc := ParseMarkdown(source)

	emphases := collectNodes(doc, ast.KindEmphasis)
	if len(emphases) != 1 {
		t.Fatalf("expected 1 emphasis node, got %d", len(emphases))
	}
	em := emphases[0].(*ast.Emphasis)
	if em.Level != 1 {
		t.Errorf("expected emphasis level 1 (italic), got %d", em.Level)
	}
}

func TestParseMarkdown_Link(t *testing.T) {
	source := []byte("[click here](https://example.com)\n")
	doc := ParseMarkdown(source)

	links := collectNodes(doc, ast.KindLink)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	link := links[0].(*ast.Link)
	if string(link.Destination) != "https://example.com" {
		t.Errorf("expected destination 'https://example.com', got '%s'", link.Destination)
	}
}

func TestParseMarkdown_Lists(t *testing.T) {
	source := []byte("- item 1\n- item 2\n- item 3\n")
	doc := ParseMarkdown(source)

	lists := collectNodes(doc, ast.KindList)
	if len(lists) != 1 {
		t.Fatalf("expected 1 list, got %d", len(lists))
	}

	items := collectNodes(doc, ast.KindListItem)
	if len(items) != 3 {
		t.Errorf("expected 3 list items, got %d", len(items))
	}
}

func TestParseMarkdown_OrderedList(t *testing.T) {
	source := []byte("1. first\n2. second\n3. third\n")
	doc := ParseMarkdown(source)

	lists := collectNodes(doc, ast.KindList)
	if len(lists) != 1 {
		t.Fatalf("expected 1 list, got %d", len(lists))
	}
	list := lists[0].(*ast.List)
	if !list.IsOrdered() {
		t.Error("expected ordered list")
	}
}

func TestParseMarkdown_HorizontalRule(t *testing.T) {
	source := []byte("above\n\n---\n\nbelow\n")
	doc := ParseMarkdown(source)

	rules := collectNodes(doc, ast.KindThematicBreak)
	if len(rules) != 1 {
		t.Fatalf("expected 1 thematic break, got %d", len(rules))
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected string
	}{
		{"simple h1", "# My Title\n\nContent", "My Title"},
		{"h1 with formatting", "# **Bold** Title\n\nContent", "Bold Title"},
		{"no h1", "## Not a title\n\nContent", ""},
		{"h1 after content", "Content\n\n# Title", "Title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := []byte(tt.source)
			doc := ParseMarkdown(source)
			title := ExtractTitle(doc, source)
			if title != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, title)
			}
		})
	}
}

func TestTitleFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"design-notes.md", "Design Notes"},
		{"my_project.md", "My Project"},
		{"README.md", "README"},
		{"simple.md", "Simple"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := TitleFromFilename(tt.filename)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// collectNodes walks the AST and returns all nodes of the given kind.
func collectNodes(doc ast.Node, kind ast.NodeKind) []ast.Node {
	var nodes []ast.Node
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == kind {
			nodes = append(nodes, n)
		}
		return ast.WalkContinue, nil
	})
	return nodes
}
