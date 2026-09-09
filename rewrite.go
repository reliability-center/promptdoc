package promptdoc

import (
	"strings"
	"text/template"
	"text/template/parse"
)

// strFunc is the template function every value reference is routed
// through. Authors never write it; rewrite inserts it after parsing.
const strFunc = "_str"

// str trims strings and pointers to strings, reads a nil pointer as
// empty, and returns any other value unchanged.
func str(v any) any {
	switch s := v.(type) {
	case nil:
		return ""
	case *string:
		if s == nil {
			return ""
		}
		return strings.TrimSpace(*s)
	case string:
		return strings.TrimSpace(s)
	}
	return v
}

// ref is a {{template "name"}} action: the block it appears in and the
// name it invokes.
type ref struct {
	block, name string
}

// rewrite routes every value the templates read through strFunc: a
// reference standing alone becomes "_str ref", a reference given as a
// function argument becomes "(_str ref)", and every pipeline ends in
// "| _str". Injected nodes take the position of the node they wrap, so
// errors still point at the author's text. It returns every template
// action it passed on the way, for ParseFS to resolve.
func rewrite(set *template.Template) (refs []ref) {
	for _, t := range set.Templates() {
		if t.Tree != nil {
			walk(t.Root, func(name string) { refs = append(refs, ref{t.Name(), name}) })
		}
	}
	return refs
}

func walk(n parse.Node, seen func(name string)) {
	switch n := n.(type) {
	case *parse.ListNode:
		for _, c := range n.Nodes {
			walk(c, seen)
		}
	case *parse.ActionNode:
		pipe(n.Pipe)
	case *parse.IfNode:
		branch(&n.BranchNode, seen)
	case *parse.RangeNode:
		branch(&n.BranchNode, seen)
	case *parse.WithNode:
		branch(&n.BranchNode, seen)
	case *parse.TemplateNode:
		seen(n.Name)
		pipe(n.Pipe)
	}
}

func branch(n *parse.BranchNode, seen func(name string)) {
	pipe(n.Pipe)
	walk(n.List, seen)
	if n.ElseList != nil {
		walk(n.ElseList, seen)
	}
}

// pipe rewrites one pipeline in place.
func pipe(p *parse.PipeNode) {
	if p == nil {
		return
	}
	for _, c := range p.Cmds {
		for i, a := range c.Args {
			switch {
			case i == 0 && len(c.Args) == 1 && isRef(a): // {{.X}} -> {{_str .X}}
				c.Args = []parse.Node{ident(c.Pos), a}
			case i > 0 && isRef(a): // f .X -> f (_str .X)
				c.Args[i] = &parse.PipeNode{NodeType: parse.NodePipe, Pos: a.Position(), Cmds: []*parse.CommandNode{cmd(a.Position(), a)}}
			default:
				if sub, ok := a.(*parse.PipeNode); ok { // f (g .X) -> recurse
					pipe(sub)
				}
			}
		}
	}
	if last := p.Cmds[len(p.Cmds)-1]; !isStr(last) {
		p.Cmds = append(p.Cmds, cmd(p.Pos))
	}
}

// isRef reports whether n reads a value: a field, variable, dot or
// chain. Literals and function names are not references.
func isRef(n parse.Node) bool {
	switch n.(type) {
	case *parse.FieldNode, *parse.VariableNode, *parse.DotNode, *parse.ChainNode:
		return true
	}
	return false
}

func isStr(c *parse.CommandNode) bool {
	id, ok := c.Args[0].(*parse.IdentifierNode)
	return ok && id.Ident == strFunc
}

// cmd builds the command "_str args..." at pos.
func cmd(pos parse.Pos, args ...parse.Node) *parse.CommandNode {
	return &parse.CommandNode{NodeType: parse.NodeCommand, Pos: pos, Args: append([]parse.Node{ident(pos)}, args...)}
}

func ident(pos parse.Pos) *parse.IdentifierNode {
	return parse.NewIdentifier(strFunc).SetPos(pos)
}
