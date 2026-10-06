// Package markdown renders the GitHub-flavoured Markdown subset used in card
// descriptions: headings, emphasis, strikethrough, inline code, fenced code
// blocks, blockquotes, nested lists, task lists, horizontal rules, links and
// bare URLs.
//
// It is written with the standard library only. Safety comes from escaping
// every piece of user text before any HTML is produced and from allowing
// only http, https and mailto links, so the output can be embedded as
// template.HTML without a sanitizer.
package markdown

import (
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strings"
)

// Render converts Markdown source into safe HTML.
func Render(src string) template.HTML {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var b strings.Builder
	renderBlocks(&b, lines)
	return template.HTML(b.String()) //nolint:gosec // every user string is escaped in inline(); see package doc
}

// Checklist counts task list items ("- [ ]" and "- [x]") in src.
func Checklist(src string) (done, total int) {
	for _, l := range strings.Split(src, "\n") {
		if m := taskRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			total++
			if m[1] != " " {
				done++
			}
		}
	}
	return done, total
}

var (
	headingRe = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	hrRe      = regexp.MustCompile(`^(\*\s*){3,}$|^(-\s*){3,}$|^(_\s*){3,}$`)
	ulRe      = regexp.MustCompile(`^([ \t]*)[-*+]\s+(.*)$`)
	olRe      = regexp.MustCompile(`^([ \t]*)(\d{1,9})[.)]\s+(.*)$`)
	taskRe    = regexp.MustCompile(`^(?:[-*+]|\d{1,9}[.)])\s+\[([ xX])\]\s+`)
	fenceRe   = regexp.MustCompile("^\\s*(```|~~~)\\s*([\\w+-]*)")
)

func renderBlocks(b *strings.Builder, lines []string) {
	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			i++
		case fenceRe.MatchString(line):
			i = renderFence(b, lines, i)
		case headingRe.MatchString(trimmed):
			m := headingRe.FindStringSubmatch(trimmed)
			fmt.Fprintf(b, "<h%d>%s</h%d>", len(m[1]), inline(m[2]), len(m[1]))
			i++
		case hrRe.MatchString(trimmed):
			b.WriteString("<hr>")
			i++
		case strings.HasPrefix(trimmed, ">"):
			var inner []string
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">") {
				t := strings.TrimPrefix(strings.TrimSpace(lines[i]), ">")
				inner = append(inner, strings.TrimPrefix(t, " "))
				i++
			}
			b.WriteString("<blockquote>")
			renderBlocks(b, inner)
			b.WriteString("</blockquote>")
		case ulRe.MatchString(line) || olRe.MatchString(line):
			i = renderList(b, lines, i)
		default:
			i = renderParagraph(b, lines, i)
		}
	}
}

func renderFence(b *strings.Builder, lines []string, i int) int {
	m := fenceRe.FindStringSubmatch(lines[i])
	marker, lang := m[1], m[2]
	i++
	var code []string
	for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), marker) {
		code = append(code, lines[i])
		i++
	}
	if i < len(lines) {
		i++ // closing fence
	}
	if lang != "" {
		fmt.Fprintf(b, `<pre><code class="language-%s">`, html.EscapeString(lang))
	} else {
		b.WriteString("<pre><code>")
	}
	b.WriteString(html.EscapeString(strings.Join(code, "\n")))
	b.WriteString("</code></pre>")
	return i
}

func indentOf(s string) int {
	n := 0
	for _, r := range s {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

// listItem parses a list marker line, returning the item text and whether
// the list is ordered.
func listItem(line string) (indent int, ordered bool, text string, ok bool) {
	if m := ulRe.FindStringSubmatch(line); m != nil {
		return indentOf(m[1]), false, m[2], true
	}
	if m := olRe.FindStringSubmatch(line); m != nil {
		return indentOf(m[1]), true, m[3], true
	}
	return 0, false, "", false
}

// renderList renders consecutive items at the same indentation; deeper
// indented lines become a nested block inside the current item.
func renderList(b *strings.Builder, lines []string, i int) int {
	base, ordered, _, _ := listItem(lines[i])
	tag := "ul"
	if ordered {
		tag = "ol"
	}
	b.WriteString("<" + tag + ">")
	for i < len(lines) {
		indent, ord, text, ok := listItem(lines[i])
		if !ok || indent < base || (indent == base && ord != ordered) {
			break
		}
		i++
		var nested []string
		for i < len(lines) {
			l := lines[i]
			if strings.TrimSpace(l) == "" {
				// A blank line ends the list unless the next line keeps it going.
				if i+1 < len(lines) && indentOf(lines[i+1]) > base {
					nested = append(nested, "")
					i++
					continue
				}
				break
			}
			if indentOf(l) <= base {
				break
			}
			nested = append(nested, l)
			i++
		}
		if m := taskRe.FindStringSubmatch("- " + text); m != nil {
			checked := ""
			if m[1] != " " {
				checked = " checked"
			}
			rest := strings.TrimSpace(text[strings.Index(text, "]")+1:])
			fmt.Fprintf(b, `<li class="task"><input type="checkbox" disabled%s> %s`, checked, inline(rest))
		} else {
			fmt.Fprintf(b, "<li>%s", inline(text))
		}
		if len(nested) > 0 {
			renderBlocks(b, dedent(nested))
		}
		b.WriteString("</li>")
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" && i+1 < len(lines) {
			if _, _, _, next := listItem(lines[i+1]); !next || indentOf(lines[i+1]) != base {
				break
			}
			i++
		}
	}
	b.WriteString("</" + tag + ">")
	return i
}

func dedent(lines []string) []string {
	minIndent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if n := indentOf(l); minIndent < 0 || n < minIndent {
			minIndent = n
		}
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		cut := 0
		for cut < len(l) && indentOf(l[:cut+1]) <= minIndent && (l[cut] == ' ' || l[cut] == '\t') {
			cut++
		}
		out[i] = l[cut:]
	}
	return out
}

func renderParagraph(b *strings.Builder, lines []string, i int) int {
	var parts []string
	for i < len(lines) {
		l := lines[i]
		t := strings.TrimSpace(l)
		if t == "" || headingRe.MatchString(t) || hrRe.MatchString(t) || strings.HasPrefix(t, ">") ||
			fenceRe.MatchString(l) || ulRe.MatchString(l) || olRe.MatchString(l) {
			break
		}
		parts = append(parts, inline(t))
		i++
	}
	// Single newlines are kept as line breaks: card notes are written like
	// chat messages, not like prose documents.
	b.WriteString("<p>" + strings.Join(parts, "<br>") + "</p>")
	return i
}

var (
	codeSpanRe = regexp.MustCompile("`([^`]+)`")
	linkRe     = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	autoLinkRe = regexp.MustCompile(`https?://[^\s<>"]+[^\s<>".,;:!?)\]]`)
	boldRe     = regexp.MustCompile(`\*\*(\S(?:.*?\S)?)\*\*|__(\S(?:.*?\S)?)__`)
	italicRe   = regexp.MustCompile(`\*(\S(?:[^*]*?\S)?)\*|(^|[^\w])_(\S(?:[^_]*?\S)?)_([^\w]|$)`)
	strikeRe   = regexp.MustCompile(`~~(\S(?:.*?\S)?)~~`)
	escapeRe   = regexp.MustCompile("\\\\([\\\\`*_{}\\[\\]()#+\\-.!~>])")
	slotRe     = regexp.MustCompile("\x00(\\d+)\x00")
)

// inline renders one line of inline Markdown. Pieces that must not be
// re-processed (code spans, links, escaped characters) are swapped for
// numbered placeholders until the end.
func inline(s string) string {
	var slots []string
	hold := func(htmlPiece string) string {
		slots = append(slots, htmlPiece)
		return fmt.Sprintf("\x00%d\x00", len(slots)-1)
	}
	s = strings.ReplaceAll(s, "\x00", "")
	s = escapeRe.ReplaceAllStringFunc(s, func(m string) string { return hold(html.EscapeString(m[1:])) })
	s = codeSpanRe.ReplaceAllStringFunc(s, func(m string) string {
		return hold("<code>" + html.EscapeString(m[1:len(m)-1]) + "</code>")
	})
	s = linkRe.ReplaceAllStringFunc(s, func(m string) string {
		p := linkRe.FindStringSubmatch(m)
		if !safeURL(p[2]) {
			return hold(html.EscapeString(p[1]))
		}
		return hold(anchor(p[2], inlineText(p[1])))
	})
	s = autoLinkRe.ReplaceAllStringFunc(s, func(u string) string { return hold(anchor(u, html.EscapeString(u))) })
	s = inlineText(s)
	return slotRe.ReplaceAllStringFunc(s, func(m string) string {
		var n int
		_, _ = fmt.Sscanf(strings.Trim(m, "\x00"), "%d", &n)
		if n < len(slots) {
			return slots[n]
		}
		return ""
	})
}

// inlineText escapes text and applies emphasis; placeholders pass through.
func inlineText(s string) string {
	s = html.EscapeString(s)
	s = boldRe.ReplaceAllStringFunc(s, func(m string) string {
		p := boldRe.FindStringSubmatch(m)
		return "<strong>" + p[1] + p[2] + "</strong>"
	})
	s = italicRe.ReplaceAllStringFunc(s, func(m string) string {
		p := italicRe.FindStringSubmatch(m)
		if p[1] != "" {
			return "<em>" + p[1] + "</em>"
		}
		return p[2] + "<em>" + p[3] + "</em>" + p[4]
	})
	return strikeRe.ReplaceAllString(s, "<del>$1</del>")
}

func anchor(href, text string) string {
	return `<a href="` + html.EscapeString(href) + `" target="_blank" rel="noopener noreferrer">` + text + "</a>"
}

func safeURL(u string) bool {
	l := strings.ToLower(u)
	return strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "mailto:")
}
