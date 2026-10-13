package native

import (
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// PlainText converts Markdown to plain text by parsing it as CommonMark and writing the text a
// viewer would show: headings become plain lines, list items keep their "- " and depth, and
// emphasis, strong and code-span markers and link URLs are dropped. Parsing instead of
// stripping characters keeps text CommonMark does not treat as formatting, such as an intraword
// underscore, intact.
func PlainText(markdown string) string {
	src := []byte(markdown)
	doc := goldmark.New().Parser().Parse(text.NewReader(src))
	return strings.TrimSpace(blocksText(doc, src))
}

// blocksText writes the blocks under parent, each followed by a blank line.
func blocksText(parent ast.Node, src []byte) string {
	var sb strings.Builder
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		sb.WriteString(blockText(n, src))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func blockText(n ast.Node, src []byte) string {
	switch n := n.(type) {
	case *ast.Heading, *ast.Paragraph, *ast.TextBlock:
		var sb strings.Builder
		writeInlines(&sb, n, src)
		return sb.String()
	case *ast.List:
		return listText(n, src)
	case *ast.HTMLBlock, *ast.FencedCodeBlock, *ast.CodeBlock:
		return strings.TrimRight(linesText(n, src), "\n")
	default:
		return strings.TrimSpace(blocksText(n, src))
	}
}

func linesText(n ast.Node, src []byte) string {
	var sb strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		sb.Write(seg.Value(src))
	}
	return sb.String()
}

// listText writes a list one item per line group: "- " (or "N. " when ordered) before the first line of an
// item, and the item's other blocks and nested lists indented under it.
func listText(list *ast.List, src []byte) string {
	var out []string
	number := list.Start
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		marker := "- "
		if list.IsOrdered() {
			marker = strconv.Itoa(number) + ". "
			number++
		}
		var parts []string
		for c := item.FirstChild(); c != nil; c = c.NextSibling() {
			parts = append(parts, blockText(c, src))
		}
		for i, line := range strings.Split(strings.Join(parts, "\n"), "\n") {
			switch {
			case i == 0:
				out = append(out, marker+line)
			case line == "":
				out = append(out, "")
			default:
				out = append(out, strings.Repeat(" ", len(marker))+line)
			}
		}
	}
	return strings.Join(out, "\n")
}

func writeInlines(sb *strings.Builder, parent ast.Node, src []byte) {
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *ast.Text:
			sb.WriteString(string(util.ResolveEntityNames(util.ResolveNumericReferences(util.UnescapePunctuations(n.Segment.Value(src))))))
			if n.SoftLineBreak() || n.HardLineBreak() {
				sb.WriteString("\n")
			}
		case *ast.String:
			sb.Write(n.Value)
		case *ast.AutoLink:
			sb.Write(n.URL(src))
		case *ast.CodeSpan:
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				if t, ok := c.(*ast.Text); ok {
					sb.Write(t.Segment.Value(src))
				}
			}
		case *ast.RawHTML:
			for i := 0; i < n.Segments.Len(); i++ {
				seg := n.Segments.At(i)
				sb.Write(seg.Value(src))
			}
		default:
			writeInlines(sb, n, src)
		}
	}
}
