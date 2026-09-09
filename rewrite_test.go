package promptdoc

import (
	"strings"
	"testing"
	"text/template"
	"time"
)

func Test_rewrite(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{{.Title}}`, `{{_str .Title}}`},
		{`{{or .Title "d"}}`, `{{or (_str .Title) "d" | _str}}`},
		{`{{if .D}}x{{end}}`, `{{if _str .D}}x{{end}}`},
		{`{{with .X}}{{.}}{{else}}none{{end}}`, `{{with _str .X}}{{_str .}}{{else}}none{{end}}`},
		{`{{printf "%q" .T}}`, `{{printf "%q" (_str .T) | _str}}`},
		{`{{.IsOverdue $.Now}}`, `{{.IsOverdue (_str $.Now) | _str}}`},
		{`{{range .C}}{{template "n" .}}{{end}}`, `{{range _str .C}}{{template "n" _str .}}{{end}}`},
		{`{{.T | printf "%q"}}`, `{{_str .T | printf "%q" | _str}}`},
		{`{{$t := .D}}{{$t}}`, `{{$t := _str .D}}{{_str $t}}`},
		{`{{len (index .M "k")}}`, `{{len (index (_str .M) "k" | _str) | _str}}`},
		{`{{"lit"}}`, `{{"lit" | _str}}`},
	}
	for _, c := range cases {
		set := template.Must(template.New("").Funcs(template.FuncMap{strFunc: str}).Parse(c.in))
		rewrite(set)
		if got := set.Root.String(); got != c.want {
			t.Errorf("rewrite(%s) = %s, want %s", c.in, got, c.want)
		}
	}
}

type node struct {
	Title, Note   *string
	Text          string
	Possible      bool
	Depth         int
	Children      []*node
	Now, Deadline time.Time
}

func (n *node) IsOverdue(now time.Time) bool { return now.After(n.Deadline) }

func p(s string) *string { return &s }

func Test_Render_nilSafe(t *testing.T) {
	d, err := load(map[string]string{"a.md": `
'''prompt node
title=[{{or .Title "untitled"}}] note=[{{.Note}}]{{if .Note}} has-note{{end}} text=[{{.Text}}] q={{printf "%q" .Title}} depth={{.Depth}}{{if .Possible}} possible{{end}} overdue={{.IsOverdue .Now}}
{{range .Children}}{{template "node" .}}{{end}}
'''
`})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	root := &node{Title: nil, Note: p("  "), Text: " root ", Possible: true, Now: now, Deadline: now.Add(-time.Hour),
		Children: []*node{{Title: p(" Kid "), Note: p(" real "), Text: "kid", Depth: 1, Now: now, Deadline: now.Add(time.Hour)}}}

	got, err := d.Render("node", root)
	if err != nil {
		t.Fatal(err)
	}
	want := "title=[untitled] note=[] text=[root] q=\"\" depth=0 possible overdue=true\n" +
		"title=[Kid] note=[real] has-note text=[kid] q=\"Kid\" depth=1 overdue=false\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func Test_Render_errorPointsAtAuthorNode(t *testing.T) {
	d, err := load(map[string]string{"a.md": "'''prompt bad\n{{.Title}} {{.Nope}}\n'''"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.Render("bad", &node{})
	if err == nil || !strings.Contains(err.Error(), "<.Nope>") || strings.Contains(err.Error(), strFunc) {
		t.Errorf("error %v, want it to name <.Nope> and not %s", err, strFunc)
	}
}
