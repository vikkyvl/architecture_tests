package imports

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func rubyImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "call" || n.Type() == "method_call" {
			method := nodeText(n.ChildByFieldName("method"), src)
			switch method {
			case "require", "require_relative", "load":
				if args := n.ChildByFieldName("arguments"); args != nil {
					collectFirstStringChild(args, src, &out)
				}
			}
		} else if n.Type() == "program" {
			return true
		}
		return true
	})
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "command" || n.Type() == "command_call" {
			ident := ""
			if c := n.NamedChild(0); c != nil {
				ident = nodeText(c, src)
			}
			if ident == "require" || ident == "require_relative" || ident == "load" {
				collectFirstStringChild(n, src, &out)
			}
		}
		return true
	})
	return out
}

func kotlinImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "import_header" {
			text := strings.TrimSpace(nodeText(n, src))
			text = strings.TrimPrefix(text, "import ")
			if idx := strings.Index(text, " as "); idx >= 0 {
				text = text[:idx]
			}
			out = appendUnique(out, strings.TrimSpace(text))
			return false
		}
		return true
	})
	return out
}

func swiftImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "import_declaration" {
			text := strings.TrimSpace(nodeText(n, src))
			fields := strings.Fields(text)
			if len(fields) > 0 {
				out = appendUnique(out, fields[len(fields)-1])
			}
			return false
		}
		return true
	})
	return out
}

func luaImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "function_call" || n.Type() == "call" {
			fname := nodeText(n.ChildByFieldName("name"), src)
			if fname == "" {
				if c := n.NamedChild(0); c != nil {
					fname = nodeText(c, src)
				}
			}
			if fname == "require" {
				if args := n.ChildByFieldName("arguments"); args != nil {
					collectFirstStringChild(args, src, &out)
				} else {
					collectFirstStringChild(n, src, &out)
				}
			}
		}
		return true
	})
	return out
}

func elixirImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "call" {
			target := ""
			if t := n.ChildByFieldName("target"); t != nil {
				target = nodeText(t, src)
			} else if c := n.NamedChild(0); c != nil {
				target = nodeText(c, src)
			}
			switch target {
			case "alias", "import", "use", "require":
				walk(n, func(c *sitter.Node) bool {
					switch c.Type() {
					case "alias":
						if t2 := nodeText(c, src); t2 != target {
							out = appendUnique(out, t2)
							return false
						}
					case "dot", "qualified_alias":
						out = appendUnique(out, strings.TrimSpace(nodeText(c, src)))
						return false
					}
					return true
				})
			}
		}
		return true
	})
	return out
}
