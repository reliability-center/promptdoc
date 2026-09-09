# promptdoc

Renders prompts kept inside markdown documents, so a prompt and the
prose that explains it live in one file.

## The document

A prompt is a markdown code block whose opening line of backticks is
followed by `prompt <name>`. Everything else in the file is prose for
people and is never rendered.

````markdown
# Greetings

The model greets the user by name. Keep it to one line; the chat
window is narrow.

```prompt greeting
Hello, {{.Name}}.
```

A greeting followed by a question, for the start of a session.

```prompt opener
{{template "greeting" .}} What are we working on today?
```
````

The rules:

- The lines between the backtick lines are a Go `text/template`.
- The closing line has at least as many backticks as the opening one
  and nothing else. A block that needs a line of three backticks
  inside it opens with four.
- Every block parsed together shares one namespace, so a block can
  invoke another with `{{template "name" .}}`, across files.
- Invoking a block that does not exist is a parse error, not a render
  error. A bad document fails when it is loaded, not when a request
  reaches it.

## Go

```
go get github.com/reliability-center/promptdoc
```

Embed the documents, parse them once at package level, render a block
per request:

```go
import (
	"embed"

	"github.com/reliability-center/promptdoc"
)

//go:embed *.md
var docs embed.FS

var prompts = promptdoc.Must(promptdoc.ParseFS(docs, "*.md"))

func opener(name string) (string, error) {
	return prompts.Render("opener", struct{ Name string }{name})
}
```

`ParseFS` takes any `fs.FS` and the same glob patterns as `fs.Glob`.
`Must` panics on error, so a malformed document stops the program at
init. `Render` returns the block's text exactly as the template
produced it; nothing is trimmed or reformatted. A `Doc` is safe for
concurrent use.

## Strings

Every value a template reads is trimmed if it is a string or a pointer
to a string, a nil pointer reads as empty, and other values are
untouched. So `{{.Title}}` on a nil `*string` renders nothing, and
`{{if .Title}}` is false when it points at a blank.

This is done by rewriting the parsed template, not by a function the
author calls. Optional fields can be plain `*string` values in Go and
plain `{{.Field}}` reads in the document.

## Errors

Errors from `ParseFS` name the file and the line of the block's opening
backticks:

```
a.md:2: unclosed code block
b.md:2: duplicate block "a" (also at a.md:1)
a.md:1: expected ```prompt <name>
a.md:1: template: bad:1: unexpected EOF
a.md:1: block "a" invokes undefined block "nope"
```

`Render` returns `text/template`'s error unchanged; an error inside a
template points at the author's own action.

## Why a document

The wording of a prompt changes often. The reasons behind it, the
constraints it must satisfy and the cases it exists to handle change
slowly. Keeping the reasons beside the text, in prose that is never
sent to the model, is what lets the next person change the wording
without losing them.

A document that reads top to bottom as "what this feature asks the
model to do, and why" is also the best context for anyone editing the
feature, whether a person or a coding agent.

When the prose lists what a good answer must contain and must not,
that section is the specification. Evaluations, when they come, belong
there too.

## License

Copyright 2026 Reliability Center, Inc.

Licensed under the Apache License, Version 2.0; see [LICENSE](LICENSE).
