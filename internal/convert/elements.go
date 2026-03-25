package convert

import (
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"google.golang.org/api/docs/v1"
)

// docBuilder accumulates plain text and style requests for a Google Doc.
// Strategy: first pass builds the full text content; second pass applies styles.
// This avoids the offset-shifting problem with interleaved inserts.
type docBuilder struct {
	text     []byte          // accumulated plain text
	styles   []styleRange    // pending style applications
	paraStyles []paraStyleRange // pending paragraph style applications
	source   []byte          // original markdown source
}

type styleRange struct {
	start  int64
	end    int64
	bold   bool
	italic bool
	link   string
}

type paraStyleRange struct {
	start     int64
	end       int64
	namedStyle string
}

func newDocBuilder(source []byte) *docBuilder {
	return &docBuilder{source: source}
}

// writeText appends text and returns its start index in the document.
// Note: Google Docs index starts at 1 (index 0 is the document body start).
func (b *docBuilder) writeText(text string) int64 {
	start := int64(len(b.text))
	b.text = append(b.text, []byte(text)...)
	return start
}

func (b *docBuilder) currentIndex() int64 {
	return int64(len(b.text))
}

func (b *docBuilder) addStyle(start, end int64, bold, italic bool, link string) {
	if start >= end {
		return
	}
	b.styles = append(b.styles, styleRange{
		start: start, end: end,
		bold: bold, italic: italic, link: link,
	})
}

func (b *docBuilder) addParaStyle(start, end int64, namedStyle string) {
	if start >= end {
		return
	}
	b.paraStyles = append(b.paraStyles, paraStyleRange{
		start: start, end: end, namedStyle: namedStyle,
	})
}

// build generates the Docs API batchUpdate requests.
func (b *docBuilder) build() []*docs.Request {
	var requests []*docs.Request

	// First request: insert all text at index 1
	if len(b.text) > 0 {
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: 1},
				Text:     string(b.text),
			},
		})
	}

	// Apply paragraph styles (headings)
	for _, ps := range b.paraStyles {
		// +1 because doc indices are 1-based after insertion at index 1
		requests = append(requests, &docs.Request{
			UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
				Range: &docs.Range{
					StartIndex: ps.start + 1,
					EndIndex:   ps.end + 1,
				},
				ParagraphStyle: &docs.ParagraphStyle{
					NamedStyleType: ps.namedStyle,
				},
				Fields: "namedStyleType",
			},
		})
	}

	// Apply text styles (bold, italic, links)
	for _, s := range b.styles {
		style := &docs.TextStyle{}
		fields := ""

		if s.bold {
			style.Bold = true
			fields = appendField(fields, "bold")
		}
		if s.italic {
			style.Italic = true
			fields = appendField(fields, "italic")
		}
		if s.link != "" {
			style.Link = &docs.Link{Url: s.link}
			fields = appendField(fields, "link")
		}

		if fields == "" {
			continue
		}

		requests = append(requests, &docs.Request{
			UpdateTextStyle: &docs.UpdateTextStyleRequest{
				Range: &docs.Range{
					StartIndex: s.start + 1,
					EndIndex:   s.end + 1,
				},
				TextStyle: style,
				Fields:    fields,
			},
		})
	}

	return requests
}

func appendField(existing, field string) string {
	if existing == "" {
		return field
	}
	return existing + "," + field
}

// walkNode recursively walks the AST and populates the docBuilder.
func (b *docBuilder) walkNode(n ast.Node, inBold, inItalic bool, linkURL string) {
	switch node := n.(type) {
	case *ast.Document:
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			b.walkNode(child, false, false, "")
		}

	case *ast.Heading:
		start := b.currentIndex()
		b.walkInlineChildren(node, false, false, "")
		b.writeText("\n")
		end := b.currentIndex()

		style := headingStyle(node.Level)
		b.addParaStyle(start, end, style)

	case *ast.Paragraph:
		// Check if parent is a list item — don't add extra newline
		if n.Parent() != nil && n.Parent().Kind() == ast.KindListItem {
			b.walkInlineChildren(node, inBold, inItalic, linkURL)
		} else {
			b.walkInlineChildren(node, false, false, "")
			b.writeText("\n")
		}

	case *ast.List:
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			if item, ok := child.(*ast.ListItem); ok {
				b.walkListItem(item, node.IsOrdered(), listItemIndex(node, item))
			}
		}

	case *ast.ThematicBreak:
		b.writeText("───────────────────────────────\n")

	case *ast.CodeBlock:
		lines := node.Lines()
		for i := 0; i < lines.Len(); i++ {
			line := lines.At(i)
			b.writeText(string(line.Value(b.source)))
		}
		b.writeText("\n")

	case *ast.FencedCodeBlock:
		lines := node.Lines()
		for i := 0; i < lines.Len(); i++ {
			line := lines.At(i)
			b.writeText(string(line.Value(b.source)))
		}
		b.writeText("\n")

	case *ast.Blockquote:
		// Walk children with quote prefix handling
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			b.walkNode(child, false, false, "")
		}

	case *extast.Strikethrough:
		// Strikethrough — render as plain text with tildes for now
		b.writeText("~")
		b.walkInlineChildren(node, inBold, inItalic, linkURL)
		b.writeText("~")

	default:
		// For any other block node, walk its children
		if n.HasChildren() {
			for child := n.FirstChild(); child != nil; child = child.NextSibling() {
				b.walkNode(child, inBold, inItalic, linkURL)
			}
		}
	}
}

// walkInlineChildren walks the inline children of a block node.
func (b *docBuilder) walkInlineChildren(n ast.Node, inBold, inItalic bool, linkURL string) {
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		b.walkInline(child, inBold, inItalic, linkURL)
	}
}

// walkInline handles inline elements (text, bold, italic, links).
func (b *docBuilder) walkInline(n ast.Node, inBold, inItalic bool, linkURL string) {
	switch node := n.(type) {
	case *ast.Text:
		text := string(node.Segment.Value(b.source))
		start := b.currentIndex()
		b.writeText(text)
		end := b.currentIndex()

		if inBold || inItalic || linkURL != "" {
			b.addStyle(start, end, inBold, inItalic, linkURL)
		}

		if node.SoftLineBreak() {
			b.writeText(" ")
		}
		if node.HardLineBreak() {
			b.writeText("\n")
		}

	case *ast.Emphasis:
		bold := inBold || node.Level == 2
		italic := inItalic || node.Level == 1
		b.walkInlineChildren(node, bold, italic, linkURL)

	case *ast.Link:
		url := string(node.Destination)
		b.walkInlineChildren(node, inBold, inItalic, url)

	case *ast.CodeSpan:
		// Inline code — just render as plain text
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			if t, ok := child.(*ast.Text); ok {
				b.writeText(string(t.Segment.Value(b.source)))
			}
		}

	case *ast.AutoLink:
		url := string(node.URL(b.source))
		start := b.currentIndex()
		b.writeText(url)
		end := b.currentIndex()
		b.addStyle(start, end, inBold, inItalic, url)

	default:
		// For any other inline, try walking children
		if n.HasChildren() {
			b.walkInlineChildren(n, inBold, inItalic, linkURL)
		}
	}
}

func (b *docBuilder) walkListItem(item *ast.ListItem, ordered bool, index int) {
	// Add bullet/number prefix
	if ordered {
		prefix := string(rune('0'+index)) + ". "
		if index >= 10 {
			prefix = string([]byte{byte('0' + index/10), byte('0' + index%10)}) + ". "
		}
		b.writeText(prefix)
	} else {
		b.writeText("• ")
	}

	// Walk item content
	for child := item.FirstChild(); child != nil; child = child.NextSibling() {
		b.walkNode(child, false, false, "")
	}
	b.writeText("\n")
}

func listItemIndex(list *ast.List, target *ast.ListItem) int {
	idx := int(list.Start)
	if idx == 0 {
		idx = 1
	}
	for child := list.FirstChild(); child != nil; child = child.NextSibling() {
		if child == target {
			return idx
		}
		idx++
	}
	return idx
}

func headingStyle(level int) string {
	switch level {
	case 1:
		return "HEADING_1"
	case 2:
		return "HEADING_2"
	case 3:
		return "HEADING_3"
	case 4:
		return "HEADING_4"
	case 5:
		return "HEADING_5"
	case 6:
		return "HEADING_6"
	default:
		return "NORMAL_TEXT"
	}
}

// MarkdownToDocs converts markdown content to Google Docs API batchUpdate requests.
// Returns the requests, the extracted title (from first H1), and any error.
func MarkdownToDocs(mdContent []byte) ([]*docs.Request, string, error) {
	doc := ParseMarkdown(mdContent)
	title := ExtractTitle(doc, mdContent)

	builder := newDocBuilder(mdContent)
	builder.walkNode(doc, false, false, "")

	requests := builder.build()
	return requests, title, nil
}
