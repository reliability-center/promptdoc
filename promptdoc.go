// Package promptdoc renders prompts kept inside markdown documents.
//
// A prompt is a markdown code block whose opening line of backticks is
// followed by "prompt <name>":
//
//	```prompt greeting
//	Hello, {{.Name}}.
//	```
//
// The lines between the backtick lines are a text/template. The closing
// line must have at least as many backticks as the opening one and
// nothing else. Everything outside these blocks is prose for people and
// is never rendered.
//
// Every value a template reads is trimmed if it is a string or a
// pointer to a string, a nil pointer reads as empty, and other values
// are untouched. So {{.Title}} on a nil *string renders nothing, and
// {{if .Title}} is false when it points at a blank.
//
// All blocks parsed together share one namespace, so a block can invoke
// another with {{template "name"}}, across files. Invoking a block that
// does not exist is a parse error, not a render error.
package promptdoc

import (
	"fmt"
	"io/fs"
	"strings"
	"text/template"
)

// Doc is a set of parsed blocks. It is safe for concurrent use.
type Doc struct {
	set *template.Template
}

type block struct {
	name, body, file string
	line             int // of the opening backtick line, 1-based
}

// ParseFS parses the blocks of every file in fsys matching patterns.
func ParseFS(fsys fs.FS, patterns ...string) (*Doc, error) {
	set := template.New("").Funcs(template.FuncMap{strFunc: str})
	seen := map[string]block{}
	for _, pattern := range patterns {
		names, err := fs.Glob(fsys, pattern)
		if err != nil {
			return nil, err
		}
		if len(names) == 0 {
			return nil, fmt.Errorf("promptdoc: pattern matches no files: %q", pattern)
		}
		for _, name := range names {
			text, err := fs.ReadFile(fsys, name)
			if err != nil {
				return nil, err
			}
			bs, err := blocks(name, string(text))
			if err != nil {
				return nil, err
			}
			for _, b := range bs {
				if first, ok := seen[b.name]; ok {
					return nil, fmt.Errorf("%s:%d: duplicate block %q (also at %s:%d)", b.file, b.line, b.name, first.file, first.line)
				}
				seen[b.name] = b
				if _, err := set.New(b.name).Parse(b.body); err != nil {
					return nil, fmt.Errorf("%s:%d: %v", b.file, b.line, err)
				}
			}
		}
	}
	for _, r := range rewrite(set) {
		if set.Lookup(r.name) == nil {
			b := seen[r.block]
			return nil, fmt.Errorf("%s:%d: block %q invokes undefined block %q", b.file, b.line, b.name, r.name)
		}
	}
	return &Doc{set}, nil
}

// Must panics if err is not nil. It is intended for package-level
// variables: var doc = promptdoc.Must(promptdoc.ParseFS(fsys, "*.md")).
func Must(d *Doc, err error) *Doc {
	if err != nil {
		panic(err)
	}
	return d
}

// Render executes the named block with data.
func (d *Doc) Render(name string, data any) (string, error) {
	var b strings.Builder
	if err := d.set.ExecuteTemplate(&b, name, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

// blocks extracts the prompt blocks of one file. Every code block,
// prompt or not, must be closed, so an unclosed prose block cannot
// swallow a prompt.
func blocks(file, text string) ([]block, error) {
	var out []block
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		n := backticks(lines[i])
		if n == 0 {
			continue
		}
		open := i
		for i++; i < len(lines); i++ {
			if closes(lines[i], n) {
				break
			}
		}
		if i == len(lines) {
			return nil, fmt.Errorf("%s:%d: unclosed code block", file, open+1)
		}
		info := strings.Fields(lines[open][n:])
		if len(info) == 0 || info[0] != "prompt" {
			continue
		}
		if len(info) != 2 {
			return nil, fmt.Errorf("%s:%d: expected ```prompt <name>", file, open+1)
		}
		out = append(out, block{info[1], strings.Join(lines[open+1:i], "\n"), file, open + 1})
	}
	return out, nil
}

// backticks returns the length of the run of backticks that starts
// line, if it is three or more, else 0.
func backticks(line string) int {
	n := len(line) - len(strings.TrimLeft(line, "`"))
	if n < 3 {
		return 0
	}
	return n
}

// closes reports whether line closes a code block opened with n
// backticks.
func closes(line string, n int) bool {
	return backticks(line) >= n && strings.TrimSpace(strings.TrimLeft(line, "`")) == ""
}
