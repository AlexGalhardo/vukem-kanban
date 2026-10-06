package markdown

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"paragraph with line break", "linha 1\nlinha 2", "<p>linha 1<br>linha 2</p>"},
		{"paragraphs", "a\n\nb", "<p>a</p><p>b</p>"},
		{"headings", "# Título\n### Sub ###", "<h1>Título</h1><h3>Sub</h3>"},
		{"emphasis", "**negrito** e *itálico* e _também_ e ~~riscado~~", "<p><strong>negrito</strong> e <em>itálico</em> e <em>também</em> e <del>riscado</del></p>"},
		{"snake_case is not italic", "use my_var_name aqui", "<p>use my_var_name aqui</p>"},
		{"inline code is literal", "rode `go test **./...**`", "<p>rode <code>go test **./...**</code></p>"},
		{"fenced code", "```go\nfmt.Println(\"<x>\")\n```", `<pre><code class="language-go">fmt.Println(&#34;&lt;x&gt;&#34;)</code></pre>`},
		{"unclosed fence keeps going", "```\na\nb", "<pre><code>a\nb</code></pre>"},
		{"blockquote", "> citação\n> **forte**", "<blockquote><p>citação<br><strong>forte</strong></p></blockquote>"},
		{"unordered list", "- a\n- b", "<ul><li>a</li><li>b</li></ul>"},
		{"ordered list", "1. um\n2. dois", "<ol><li>um</li><li>dois</li></ol>"},
		{"nested list", "- a\n  - a1\n  - a2\n- b", "<ul><li>a<ul><li>a1</li><li>a2</li></ul></li><li>b</li></ul>"},
		{"task list", "- [ ] fazer\n- [x] feito", `<ul><li class="task"><input type="checkbox" disabled> fazer</li><li class="task"><input type="checkbox" disabled checked> feito</li></ul>`},
		{"hr", "a\n\n---\n\nb", "<p>a</p><hr><p>b</p>"},
		{"link", "[docs](https://go.dev/doc)", `<p><a href="https://go.dev/doc" target="_blank" rel="noopener noreferrer">docs</a></p>`},
		{"autolink", "veja https://github.com/x/y.", `<p>veja <a href="https://github.com/x/y" target="_blank" rel="noopener noreferrer">https://github.com/x/y</a>.</p>`},
		{"escaped markers", `\*não itálico\*`, "<p>*não itálico*</p>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(Render(tc.in)); got != tc.want {
				t.Fatalf("\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// TestRenderIsSafe feeds hostile input; nothing may come out as live HTML,
// script URL or event handler.
func TestRenderIsSafe(t *testing.T) {
	hostile := []string{
		`<script>alert(1)</script>`,
		`<img src=x onerror=alert(1)>`,
		`[x](javascript:alert(1))`,
		`[x](JaVaScRiPt:alert(1))`,
		`[x](data:text/html,<script>alert(1)</script>)`,
		"[<b>x</b>](https://ok.dev)",
		`[x](https://ok.dev/"onmouseover="alert(1))`,
		"`</code><script>alert(1)</script>`",
		"```\n</pre><script>alert(1)</script>\n```",
		"> <iframe src=//evil>",
		"- [ ] <svg onload=alert(1)>",
		"# <style>body{display:none}</style>",
		"https://ok.dev/<script>",
		"\x00 placeholder \x000\x00 injection",
	}
	for _, in := range hostile {
		out := strings.ToLower(string(Render(in)))
		for _, bad := range []string{"<script", "<img", "<iframe", "<svg", "<style", "javascript:", "data:text", `" onmouseover`, "onerror=\"", "<b>"} {
			if strings.Contains(out, bad) {
				t.Errorf("input %q produced %q containing %q", in, out, bad)
			}
		}
	}
}

func TestChecklist(t *testing.T) {
	done, total := Checklist("- [x] a\n- [ ] b\n  - [X] c\n1. [ ] d\ntexto - [x] não conta")
	if done != 2 || total != 4 {
		t.Fatalf("checklist = %d/%d", done, total)
	}
}
