package convert

import (
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"google.golang.org/api/docs/v1"
)

// docBuilder accumulates plain text and style requests for a Google Doc.
//
// Strategy: three phases in a single batchUpdate:
//   1. Insert all text at index 1 (tables get a \n placeholder)
//   2. Apply paragraph and text styles
//   3. For each table (reverse order): delete placeholder, insert native table, fill cells
//
// Phase 3 runs after styles are applied, so table insertion shifting indices
// doesn't affect already-applied styles (styles attach to characters, not indices).
//
// IMPORTANT: Google Docs API uses UTF-16 code unit offsets for indexing.
type docBuilder struct {
	text       []byte           // accumulated plain text (UTF-8)
	charCount  int64            // accumulated UTF-16 code unit count
	styles     []styleRange     // pending style applications
	paraStyles []paraStyleRange // pending paragraph style applications
	tables     []tableInfo      // pending table insertions
	source     []byte           // original markdown source
}

type styleRange struct {
	start  int64
	end    int64
	bold   bool
	italic bool
	link   string
}

type paraStyleRange struct {
	start      int64
	end        int64
	namedStyle string
}

// tableInfo records a table's position and content for deferred insertion.
type tableInfo struct {
	charPos int64      // position of the \n placeholder (UTF-16 offset from start)
	rows    int64
	cols    int64
	cells   [][]string // [row][col] = cell text
}

func newDocBuilder(source []byte) *docBuilder {
	return &docBuilder{source: source}
}

// writeText appends text and returns its start index (in UTF-16 code units).
func (b *docBuilder) writeText(text string) int64 {
	start := b.charCount
	b.text = append(b.text, []byte(text)...)
	b.charCount += utf16Len(text)
	return start
}

func (b *docBuilder) currentIndex() int64 {
	return b.charCount
}

// utf16Len returns the number of UTF-16 code units needed to represent s.
func utf16Len(s string) int64 {
	var count int64
	for _, r := range s {
		if r >= 0x10000 {
			count += 2
		} else {
			count += 1
		}
	}
	return count
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

	// Phase 1: insert all text at index 1 (tables are \n placeholders)
	if len(b.text) > 0 {
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: 1},
				Text:     string(b.text),
			},
		})
	}

	// Phase 2: apply paragraph styles (headings)
	for _, ps := range b.paraStyles {
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

	// Phase 2b: apply text styles (bold, italic, links)
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

	// Phase 3: insert native tables (reverse order to avoid shifting earlier tables)
	for i := len(b.tables) - 1; i >= 0; i-- {
		tbl := b.tables[i]
		// +1 for 1-based doc indexing
		insertIdx := tbl.charPos + 1

		// Delete the \n placeholder
		requests = append(requests, &docs.Request{
			DeleteContentRange: &docs.DeleteContentRangeRequest{
				Range: &docs.Range{
					StartIndex: insertIdx,
					EndIndex:   insertIdx + 1,
				},
			},
		})

		// Insert table
		requests = append(requests, &docs.Request{
			InsertTable: &docs.InsertTableRequest{
				Location: &docs.Location{Index: insertIdx},
				Rows:     tbl.rows,
				Columns:  tbl.cols,
			},
		})

		// Fill cells with content (reverse order within table to avoid shifting)
		for r := tbl.rows - 1; r >= 0; r-- {
			for c := tbl.cols - 1; c >= 0; c-- {
				cellText := ""
				if r < int64(len(tbl.cells)) && c < int64(len(tbl.cells[r])) {
					cellText = tbl.cells[r][c]
				}
				if cellText == "" {
					continue
				}

				// Cell content index in a newly created empty table:
				// Each cell has 1 newline. Structure per row: row_start + C cells * 2 indices each.
				// Cell (r, c) content starts at: tableStart + 2 + r*(2*C+1) + 2*c
				cellIdx := insertIdx + 2 + r*(2*tbl.cols+1) + 2*c

				requests = append(requests, &docs.Request{
					InsertText: &docs.InsertTextRequest{
						Location: &docs.Location{Index: cellIdx},
						Text:     cellText,
					},
				})
			}
		}
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
		if n.Parent() != nil && n.Parent().Kind() == ast.KindListItem {
			b.walkInlineChildren(node, inBold, inItalic, linkURL)
		} else {
			b.walkInlineChildren(node, false, false, "")
			b.writeText("\n")
		}

	case *ast.TextBlock:
		b.walkInlineChildren(node, inBold, inItalic, linkURL)

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

	case *extast.Table:
		b.walkTable(node)

	case *ast.Blockquote:
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			b.walkNode(child, false, false, "")
		}

	case *extast.Strikethrough:
		b.writeText("~")
		b.walkInlineChildren(node, inBold, inItalic, linkURL)
		b.writeText("~")

	default:
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
		if n.HasChildren() {
			b.walkInlineChildren(n, inBold, inItalic, linkURL)
		}
	}
}

func (b *docBuilder) walkTable(table *extast.Table) {
	// Collect all rows and cells as plain text
	var cells [][]string
	var numCols int64

	for child := table.FirstChild(); child != nil; child = child.NextSibling() {
		switch row := child.(type) {
		case *extast.TableHeader:
			var rowCells []string
			for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
				rowCells = append(rowCells, b.cellText(cell))
			}
			if int64(len(rowCells)) > numCols {
				numCols = int64(len(rowCells))
			}
			cells = append(cells, rowCells)
		case *extast.TableRow:
			var rowCells []string
			for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
				rowCells = append(rowCells, b.cellText(cell))
			}
			if int64(len(rowCells)) > numCols {
				numCols = int64(len(rowCells))
			}
			cells = append(cells, rowCells)
		}
	}

	if len(cells) == 0 || numCols == 0 {
		return
	}

	// Record table position and insert a \n placeholder
	pos := b.currentIndex()
	b.writeText("\n")

	b.tables = append(b.tables, tableInfo{
		charPos: pos,
		rows:    int64(len(cells)),
		cols:    numCols,
		cells:   cells,
	})
}

// cellText extracts plain text content from a table cell.
func (b *docBuilder) cellText(cell ast.Node) string {
	var text string
	for child := cell.FirstChild(); child != nil; child = child.NextSibling() {
		if t, ok := child.(*ast.Text); ok {
			text += string(t.Segment.Value(b.source))
		} else {
			text += b.inlineText(child)
		}
	}
	return text
}

// inlineText extracts text from inline nodes recursively.
func (b *docBuilder) inlineText(n ast.Node) string {
	var text string
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if t, ok := child.(*ast.Text); ok {
			text += string(t.Segment.Value(b.source))
		} else {
			text += b.inlineText(child)
		}
	}
	return text
}

func (b *docBuilder) walkListItem(item *ast.ListItem, ordered bool, index int) {
	if ordered {
		prefix := string(rune('0'+index)) + ". "
		if index >= 10 {
			prefix = string([]byte{byte('0' + index/10), byte('0' + index%10)}) + ". "
		}
		b.writeText(prefix)
	} else {
		b.writeText("\u2022 ") // bullet character
	}

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
