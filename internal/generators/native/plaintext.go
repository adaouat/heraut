package native

import (
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
	var sb strings.Builder
	writeBlocks(&sb, doc, src, 0)
	return strings.TrimSpace(sb.String())
}

func writeBlocks(sb *strings.Builder, parent ast.Node, src []byte, depth int) {
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *ast.Heading, *ast.Paragraph, *ast.TextBlock:
			sb.WriteString(strings.Repeat("  ", depth))
			writeInlines(sb, n, src)
			sb.WriteString("\n\n")
		case *ast.List:
			writeList(sb, n, src, depth)
			if depth == 0 {
				sb.WriteString("\n")
			}
		case *ast.HTMLBlock:
			for i := 0; i < n.Lines().Len(); i++ {
				seg := n.Lines().At(i)
				sb.Write(seg.Value(src))
			}
			sb.WriteString("\n")
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			for i := 0; i < n.Lines().Len(); i++ {
				seg := n.Lines().At(i)
				sb.Write(seg.Value(src))
			}
			sb.WriteString("\n")
		default:
			writeBlocks(sb, n, src, depth)
		}
	}
}

func writeList(sb *strings.Builder, list *ast.List, src []byte, depth int) {
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		sb.WriteString(strings.Repeat("  ", depth) + "- ")
		first := true
		for c := item.FirstChild(); c != nil; c = c.NextSibling() {
			if sub, ok := c.(*ast.List); ok {
				if first {
					sb.WriteString("\n")
				}
				writeList(sb, sub, src, depth+1)
				first = false
				continue
			}
			writeInlines(sb, c, src)
			sb.WriteString("\n")
			first = false
		}
	}
}

func writeInlines(sb *strings.Builder, parent ast.Node, src []byte) {
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *ast.Text:
			sb.WriteString(string(util.UnescapePunctuations(n.Segment.Value(src))))
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
