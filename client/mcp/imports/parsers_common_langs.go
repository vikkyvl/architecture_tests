package imports

import (
	sitter "github.com/smacker/go-tree-sitter"
)

func phpImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		switch n.Type() {
		case "namespace_use_declaration", "namespace_use_group":
			collectPHPUseNames(n, src, &out)
			return false
		}
		return true
	})
	return out
}

func collectPHPUseNames(n *sitter.Node, src []byte, out *[]string) {
	walk(n, func(c *sitter.Node) bool {
		switch c.Type() {
		case "qualified_name", "name":
			*out = appendUnique(*out, nodeText(c, src))
			return false
		}
		return true
	})
}

func jsImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		switch n.Type() {
		case "import_statement", "export_statement":
			if srcNode := n.ChildByFieldName("source"); srcNode != nil {
				collectFirstStringChild(srcNode, src, &out)
			} else {
				collectFirstStringChild(n, src, &out)
			}
		case "call_expression":
			fn := n.ChildByFieldName("function")
			if fn == nil {
				return true
			}
			name := nodeText(fn, src)
			if name == "require" || name == "import" {
				if args := n.ChildByFieldName("arguments"); args != nil {
					collectFirstStringChild(args, src, &out)
				}
			}
		}
		return true
	})
	return out
}

func pythonImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		switch n.Type() {
		case "import_statement":
			collectPythonDottedNames(n, src, &out)
			return false
		case "import_from_statement":
			module := nodeText(n.ChildByFieldName("module_name"), src)
			if module == "" {
				walk(n, func(c *sitter.Node) bool {
					if c.Type() == "relative_import" {
						module = nodeText(c, src)
						return false
					}
					return true
				})
			}
			for i := 0; i < int(n.NamedChildCount()); i++ {
				c := n.NamedChild(i)
				if c.Type() == "dotted_name" || c.Type() == "aliased_import" {
					name := nodeText(c, src)
					if c.Type() == "aliased_import" {
						if dn := c.ChildByFieldName("name"); dn != nil {
							name = nodeText(dn, src)
						}
					}
					if module != "" {
						out = appendUnique(out, module+"."+name)
					} else {
						out = appendUnique(out, name)
					}
				}
			}
			return false
		}
		return true
	})
	return out
}

func collectPythonDottedNames(n *sitter.Node, src []byte, out *[]string) {
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		switch c.Type() {
		case "dotted_name":
			*out = appendUnique(*out, nodeText(c, src))
		case "aliased_import":
			if dn := c.ChildByFieldName("name"); dn != nil {
				*out = appendUnique(*out, nodeText(dn, src))
			}
		}
	}
}

func goImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "import_spec" {
			if path := n.ChildByFieldName("path"); path != nil {
				out = appendUnique(out, trimQuotes(nodeText(path, src)))
			}
			return false
		}
		return true
	})
	return out
}

func javaImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "import_declaration" {
			collectFirstByTypes(n, src, &out, "scoped_identifier", "identifier")
			return false
		}
		return true
	})
	return out
}

func csharpImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "using_directive" {
			collectFirstByTypes(n, src, &out, "qualified_name", "identifier")
			return false
		}
		return true
	})
	return out
}
