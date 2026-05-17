package imports

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func rustImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		switch n.Type() {
		case "use_declaration":
			collectRustUsePaths(n, src, &out)
			return false
		case "extern_crate_declaration":
			collectFirstByTypes(n, src, &out, "identifier")
			return false
		}
		return true
	})
	return out
}

func collectRustUsePaths(n *sitter.Node, src []byte, out *[]string) {
	text := strings.TrimSpace(nodeText(n, src))
	text = strings.TrimPrefix(text, "use ")
	text = strings.TrimSuffix(text, ";")
	expandRustUseGroup(text, "", out)
}

func expandRustUseGroup(text, prefix string, out *[]string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	openIdx := strings.Index(text, "{")
	if openIdx < 0 {
		full := prefix + text
		full = strings.TrimSuffix(full, "::*")
		if idx := strings.Index(full, " as "); idx >= 0 {
			full = full[:idx]
		}
		*out = appendUnique(*out, strings.TrimSpace(full))
		return
	}
	base := prefix + text[:openIdx]
	closeIdx := strings.LastIndex(text, "}")
	if closeIdx < openIdx {
		return
	}
	inner := text[openIdx+1 : closeIdx]
	for _, part := range splitTopLevelCommas(inner) {
		expandRustUseGroup(part, base, out)
	}
}

func cImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		switch n.Type() {
		case "preproc_include":
			if path := n.ChildByFieldName("path"); path != nil {
				txt := strings.TrimSpace(nodeText(path, src))
				txt = strings.TrimPrefix(strings.TrimSuffix(txt, ">"), "<")
				out = appendUnique(out, trimQuotes(txt))
			}
			return false
		case "using_declaration":
			collectFirstByTypes(n, src, &out, "qualified_identifier", "identifier")
			return false
		}
		return true
	})
	return out
}

func scalaImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "import_declaration" {
			text := strings.TrimSpace(nodeText(n, src))
			text = strings.TrimPrefix(text, "import ")
			expandScalaImport(text, "", &out)
			return false
		}
		return true
	})
	return out
}

func expandScalaImport(text, prefix string, out *[]string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	openIdx := strings.Index(text, "{")
	if openIdx < 0 {
		full := prefix + text
		if idx := strings.Index(full, " => "); idx >= 0 {
			full = full[:idx]
		}
		*out = appendUnique(*out, strings.TrimSpace(full))
		return
	}
	base := prefix + text[:openIdx]
	closeIdx := strings.LastIndex(text, "}")
	if closeIdx < openIdx {
		return
	}
	inner := text[openIdx+1 : closeIdx]
	for _, part := range splitTopLevelCommas(inner) {
		expandScalaImport(part, base, out)
	}
}

func hclImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "block" {
			if c := n.NamedChild(0); c != nil {
				if nodeText(c, src) == "module" {
					collectHCLBlockSource(n, src, &out)
				}
			}
		}
		return true
	})
	return out
}

func collectHCLBlockSource(block *sitter.Node, src []byte, out *[]string) {
	walk(block, func(c *sitter.Node) bool {
		if c.Type() == "attribute" {
			if name := c.NamedChild(0); name != nil && nodeText(name, src) == "source" {
				collectFirstStringChild(c, src, out)
			}
		}
		return true
	})
}
