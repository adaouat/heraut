package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Every row was observed on github.com on 2026-10-28 by uploading the name to a draft release and
// reading the published name back (T438). The one observed name left out is "x.", published as
// "default.x"; the model gives "x".
func TestPublishedName(t *testing.T) {
	tests := []struct{ in, want string }{
		// kept as is
		{"plus+x.txt", "plus+x.txt"},
		{"at@x.txt", "at@x.txt"},
		{"under_x.txt", "under_x.txt"},
		{"dash-x.txt", "dash-x.txt"},
		{"Upper.TXT", "Upper.TXT"},
		{"MiXeD-Case.Zip", "MiXeD-Case.Zip"},
		{"a.b.c.TXT", "a.b.c.TXT"},
		{"-lead.txt", "-lead.txt"},
		{"+lead.txt", "+lead.txt"},
		{"@lead.txt", "@lead.txt"},
		{"_lead.txt", "_lead.txt"},
		{"dash--x.txt", "dash--x.txt"},
		{"und__x.txt", "und__x.txt"},
		{"plus++x.txt", "plus++x.txt"},
		{"noext", "noext"},
		// any other ASCII character becomes ".", and runs of "." collapse
		{"space x.txt", "space.x.txt"},
		{"two  spaces.txt", "two.spaces.txt"},
		{"tilde~x.txt", "tilde.x.txt"},
		{"hash#x.txt", "hash.x.txt"},
		{"pct%x.txt", "pct.x.txt"},
		{"comma,x.txt", "comma.x.txt"},
		{"eq=x.txt", "eq.x.txt"},
		{"paren(x).txt", "paren.x.txt"},
		{"brack[x].txt", "brack.x.txt"},
		{"quote'x.txt", "quote.x.txt"},
		{"bang!x.txt", "bang.x.txt"},
		{"amp&x.txt", "amp.x.txt"},
		{"colon:x.txt", "colon.x.txt"},
		{"semi;x.txt", "semi.x.txt"},
		{"dollar$x.txt", "dollar.x.txt"},
		{"caret^x.txt", "caret.x.txt"},
		{"brace{x}.txt", "brace.x.txt"},
		{"pipe|x.txt", "pipe.x.txt"},
		{"back`x.txt", "back.x.txt"},
		{`dq"x.txt`, "dq.x.txt"},
		{"star*x.txt", "star.x.txt"},
		{"lt<x>.txt", "lt.x.txt"},
		{"q?x.txt", "q.x.txt"},
		{`bs\x.txt`, "bs.x.txt"},
		{"dd..x.txt", "dd.x.txt"},
		{"multi #+~x.txt", "multi.+.x.txt"},
		// a trailing "." or space is dropped, a leading "." gets "default" in front
		{"trail.txt.", "trail.txt"},
		{"end-space.txt ", "end-space.txt"},
		{".lead.txt", "default.lead.txt"},
		{".hidden", "default.hidden"},
		// letters are transliterated, anything else non-ASCII becomes "."
		{"acc-é.txt", "acc-e.txt"},
		{"Ecole-É.txt", "Ecole-E.txt"},
		{"n-ñ.txt", "n-n.txt"},
		{"u-ü.txt", "u-u.txt"},
		{"ss-ß.txt", "ss-ss.txt"},
		{"Straße.txt", "Strasse.txt"},
		{"o-ø.txt", "o-o.txt"},
		{"Smørrebrød.txt", "Smorrebrod.txt"},
		{"ae-æ.txt", "ae-ae.txt"},
		{"l-Ł.txt", "l-L.txt"},
		{"euro€x.txt", "euro.x.txt"},
		{"emoji😀x.txt", "emoji.x.txt"},
		{"日本.txt", "default.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, publishedName(tc.in))
		})
	}
}
