package imports

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

type importExtractor func(root *sitter.Node, src []byte) []string

var Extractors = map[string]importExtractor{
	"php":        phpImports,
	"typescript": jsImports,
	"javascript": jsImports,
	"python":     pythonImports,
	"go":         goImports,
	"java":       javaImports,
	"csharp":     csharpImports,
	"ruby":       rubyImports,
	"kotlin":     kotlinImports,
	"swift":      swiftImports,
	"rust":       rustImports,
	"c":          cImports,
	"cpp":        cImports,
	"scala":      scalaImports,
	"lua":        luaImports,
	"elixir":     elixirImports,
	"hcl":        hclImports,
}

func walk(n *sitter.Node, visit func(*sitter.Node) bool) {
	if n == nil {
		return
	}
	if !visit(n) {
		return
	}
	count := int(n.ChildCount())
	for i := 0; i < count; i++ {
		walk(n.Child(i), visit)
	}
}

func nodeText(n *sitter.Node, src []byte) string {
	if n == nil {
		return ""
	}
	start, end := int(n.StartByte()), int(n.EndByte())
	if start < 0 || end > len(src) || start > end {
		return ""
	}
	return string(src[start:end])
}

func trimQuotes(s string) string {
	result := s
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '"' || first == '\'' || first == '`') && first == last {
			result = s[1 : len(s)-1]
		}
	}
	return result
}

func appendUnique(dst []string, s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return dst
	}
	for _, existing := range dst {
		if existing == s {
			return dst
		}
	}
	return append(dst, s)
}

func collectFirstStringChild(n *sitter.Node, src []byte, out *[]string) {
	walk(n, func(c *sitter.Node) bool {
		switch c.Type() {
		case "string", "string_literal", "string_lit":
			*out = appendUnique(*out, trimQuotes(nodeText(c, src)))
			return false
		case "string_fragment", "string_content", "template_literal":
			*out = appendUnique(*out, nodeText(c, src))
			return false
		}
		return true
	})
}

func collectFirstByTypes(n *sitter.Node, src []byte, out *[]string, types ...string) {
	walk(n, func(c *sitter.Node) bool {
		for _, t := range types {
			if c.Type() == t {
				*out = appendUnique(*out, nodeText(c, src))
				return false
			}
		}
		return true
	})
}

func splitTopLevelCommas(s string) []string {
	var parts []string
	depth := 0
	last := 0
	for i, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[last:i])
				last = i + 1
			}
		}
	}
	if last < len(s) {
		parts = append(parts, s[last:])
	}
	return parts
}
