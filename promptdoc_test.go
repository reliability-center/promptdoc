package promptdoc

import (
	"strings"
	"testing"
	"testing/fstest"
)

// doc turns ' into ` so test documents can be raw string literals.
func doc(s string) string { return strings.ReplaceAll(s, "'", "`") }

func load(files map[string]string) (*Doc, error) {
	fsys := fstest.MapFS{}
	for name, text := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(doc(text))}
	}
	return ParseFS(fsys, "*.md")
}

func Test_Render(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string
		data   any
		want   map[string]string // block -> rendered text
		absent []string          // names that must not be blocks
	}{
		{
			name: "two blocks with prose between",
			files: map[string]string{"a.md": `
# Title
Prose is never parsed, so {{this}} is fine.

'''prompt one
first body
'''

More prose.

'''prompt two
second body
'''
`},
			want: map[string]string{"one": "first body", "two": "second body"},
		},
		{
			name: "non-prompt code blocks are skipped whole",
			files: map[string]string{"a.md": `
'''go
fmt.Println("not a block")
'''

'''
bare block
'''

''''markdown
an example containing a backtick line:
'''prompt x
which is not a block either
'''
''''

'''prompt real
real body
'''
`},
			want:   map[string]string{"real": "real body"},
			absent: []string{"x", "go", "markdown"},
		},
		{
			name: "longer backtick line keeps a shorter one in the body",
			files: map[string]string{"a.md": `
''''prompt a
line
'''
more
''''
`},
			want: map[string]string{"a": "line\n```\nmore"},
		},
		{
			name: "blocks share a namespace across files",
			files: map[string]string{
				"a.md": "'''prompt shared\nSHARED\n'''",
				"b.md": "'''prompt user\nbefore {{template \"shared\"}} after\n'''",
			},
			want: map[string]string{"user": "before SHARED after"},
		},
		{
			name:  "data",
			files: map[string]string{"a.md": "'''prompt hello\nHello {{.Name}}\n'''"},
			data:  struct{ Name string }{"Bob"},
			want:  map[string]string{"hello": "Hello Bob"},
		},
		{
			name: "interior whitespace is verbatim, backtick lines are not part of the body",
			files: map[string]string{"a.md": `
'''prompt w

  indented

'''
`},
			want: map[string]string{"w": "\n  indented\n"},
		},
		{
			name:  "empty block",
			files: map[string]string{"a.md": "'''prompt e\n'''"},
			want:  map[string]string{"e": ""},
		},
		{
			name:   "unknown block",
			files:  map[string]string{"a.md": "'''prompt a\nbody\n'''"},
			absent: []string{"nope"},
		},
	}

	for _, c := range cases {
		d, err := load(c.files)
		if err != nil {
			t.Errorf("%s: parse: %v", c.name, err)
			continue
		}
		for block, want := range c.want {
			got, err := d.Render(block, c.data)
			if err != nil {
				t.Errorf("%s: Render(%q): %v", c.name, block, err)
			} else if got != want {
				t.Errorf("%s: Render(%q) = %q, want %q", c.name, block, got, want)
			}
		}
		for _, block := range c.absent {
			if _, err := d.Render(block, c.data); err == nil {
				t.Errorf("%s: Render(%q) succeeded, want error", c.name, block)
			}
		}
	}
}

func Test_ParseFS_errors(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  string // substring of the error
	}{
		{
			name:  "unclosed prompt block",
			files: map[string]string{"a.md": "prose\n'''prompt a\nbody\n"},
			want:  "a.md:2: unclosed code block",
		},
		{
			name:  "unclosed prose block",
			files: map[string]string{"a.md": "'''go\ncode\n"},
			want:  "a.md:1: unclosed code block",
		},
		{
			name: "duplicate block across files",
			files: map[string]string{
				"a.md": "'''prompt a\nfirst\n'''",
				"b.md": "\n'''prompt a\nsecond\n'''",
			},
			want: `b.md:2: duplicate block "a" (also at a.md:1)`,
		},
		{
			name:  "prompt block without a name",
			files: map[string]string{"a.md": "'''prompt\nbody\n'''"},
			want:  "a.md:1: expected ```prompt <name>",
		},
		{
			name:  "prompt block with too many words",
			files: map[string]string{"a.md": "\n\n'''prompt a b\nbody\n'''"},
			want:  "a.md:3: expected ```prompt <name>",
		},
		{
			name:  "template syntax error names the file and block",
			files: map[string]string{"a.md": "'''prompt bad\n{{if .X}}\n'''"},
			want:  "a.md:1: template: bad:",
		},
		{
			name:  "template action naming an undefined block",
			files: map[string]string{"a.md": "'''prompt a\n{{template \"nope\"}}\n'''"},
			want:  `a.md:1: block "a" invokes undefined block "nope"`,
		},
		{
			name:  "pattern matches no files",
			files: map[string]string{},
			want:  `promptdoc: pattern matches no files: "*.md"`,
		},
	}

	for _, c := range cases {
		_, err := load(c.files)
		if err == nil {
			t.Errorf("%s: no error, want %q", c.name, c.want)
		} else if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q, want it to contain %q", c.name, err, c.want)
		}
	}
}
