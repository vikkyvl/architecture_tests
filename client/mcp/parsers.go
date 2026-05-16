package mcp

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// importExtractor pulls import / use / require strings out of an already-parsed
// syntax tree for one language. Per spec 0002: each supported language gets one
// extractor; the dispatch map below routes by canonical language name. Adding a
// language = one entry in languageImportExtractors + one extractor function.
type importExtractor func(root *sitter.Node, src []byte) []string

var languageImportExtractors = map[string]importExtractor{
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

// walk visits every node in the tree depth-first. The visitor returns true to
// keep descending into children, false to skip the subtree.
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

// nodeText returns the byte slice of source covered by n, as a string.
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

// trimQuotes strips one matching pair of single, double, or back-tick quotes.
func trimQuotes(s string) string {
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '"' || first == '\'' || first == '`') && first == last {
			return s[1 : len(s)-1]
		}
	}
	return s
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

// ----- PHP -----------------------------------------------------------------

// PHP imports: `use Foo\Bar;`, `use Foo\{A, B};`, `use Foo as F;`.
// Tree-sitter PHP nodes:
//   namespace_use_declaration > namespace_use_clause > qualified_name / name
//   namespace_use_declaration > namespace_use_group > namespace_use_clause > name
//
// We deliberately skip `namespace_definition` — that's the file's *own*
// namespace declaration, not an import; including it would polute the deps
// list with self-references and make every file appear to "depend on" its
// own layer, which the LLM had to filter out manually.
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

// ----- JavaScript / TypeScript / TSX --------------------------------------

// import { x } from "y"           -> source string "y"
// import type { x } from "y"      -> source string "y"
// require("y") / require('y')     -> argument string
// dynamic import("y")             -> argument string
// export * from "y" (re-export)   -> source string
func jsImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		switch n.Type() {
		case "import_statement", "export_statement":
			// Pull the string source: either via the explicit "source" field, or by
			// walking for the first string literal under the statement (TypeScript
			// `import type` and re-export variants don't always set the field).
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

func collectFirstStringChild(n *sitter.Node, src []byte, out *[]string) {
	walk(n, func(c *sitter.Node) bool {
		switch c.Type() {
		// Quoted string nodes (most grammars): grab whole text and strip quotes.
		case "string", "string_literal", "string_lit":
			*out = appendUnique(*out, trimQuotes(nodeText(c, src)))
			return false
		// Inner-content nodes (string_fragment in TS/JS, string_content in Lua/Ruby,
		// template_literal in HCL): grab text as-is, no quotes to strip.
		case "string_fragment", "string_content", "template_literal":
			*out = appendUnique(*out, nodeText(c, src))
			return false
		}
		return true
	})
}

// ----- Python --------------------------------------------------------------

// import a.b.c              -> a.b.c
// import a.b as c           -> a.b
// from a.b import c, d      -> a.b.c, a.b.d
// from . import x           -> .x
// from ..pkg import y       -> ..pkg.y
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
				// relative import: child of type `relative_import` carries the dots.
				walk(n, func(c *sitter.Node) bool {
					if c.Type() == "relative_import" {
						module = nodeText(c, src)
						return false
					}
					return true
				})
			}
			// each name child is the imported symbol.
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

// ----- Go ------------------------------------------------------------------

// import "fmt"
// import ( "fmt"; alias "os" )
// import . "math"
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

// ----- Java ----------------------------------------------------------------

// import java.util.List;
// import static java.lang.Math.PI;
// import java.util.*;
func javaImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "import_declaration" {
			// The first scoped_identifier / identifier child after the `import` keyword.
			collectFirstByTypes(n, src, &out, "scoped_identifier", "identifier")
			return false
		}
		return true
	})
	return out
}

// ----- C# ------------------------------------------------------------------

// using System.Collections.Generic;
// using static System.Math;
// using Sys = System;            -> System (not the alias)
func csharpImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "using_directive" {
			// Aliased form: name_equals + identifier/qualified_name
			collectFirstByTypes(n, src, &out, "qualified_name", "identifier")
			return false
		}
		return true
	})
	return out
}

// ----- Ruby ----------------------------------------------------------------

// require "rails"
// require_relative "../foo"
// load "x.rb"
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
			// Some Ruby grammars expose top-level require as a bare command call
			// without a `method` field — walk command nodes too.
			return true
		}
		return true
	})
	// Fallback: command-style `require "x"` may appear as `command_call` or simply
	// `identifier` followed by `argument_list`. Pick up explicit string literals
	// following a require-like identifier at top level.
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

// ----- Kotlin --------------------------------------------------------------

// import com.example.Foo
// import com.example.*
// import com.example.Foo as Bar
func kotlinImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "import_header" {
			// Concatenate identifier / dot_qualified_expression segments.
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

// ----- Swift ---------------------------------------------------------------

// import Foundation
// @testable import App
// import struct Foundation.Date
func swiftImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "import_declaration" {
			text := strings.TrimSpace(nodeText(n, src))
			// strip leading attributes and modifiers; the last whitespace-separated
			// token is the imported module path.
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

// ----- Rust ----------------------------------------------------------------

// use std::collections::HashMap;
// use foo::{a, b};
// use foo::bar::*;
// extern crate serde;
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
	// Argument tree includes scoped_identifier / use_list / use_as_clause / use_wildcard.
	// Pragmatic approach: pull out the textual path, expanding {a, b} groups manually.
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
		// strip ` as Alias`
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

// ----- C / C++ -------------------------------------------------------------

// #include <stdio.h>           -> stdio.h
// #include "config.h"          -> config.h
// using namespace std;         -> std (C++)
// using std::vector;           -> std::vector (C++)
func cImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		switch n.Type() {
		case "preproc_include":
			// path is in `path` field, either `string_literal` or `system_lib_string`.
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

// ----- Scala ---------------------------------------------------------------

// import scala.collection.mutable.Map
// import scala.collection.mutable.{Map, Set}
// import java.util.{HashMap => HM}
func scalaImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "import_declaration" {
			text := strings.TrimSpace(nodeText(n, src))
			text = strings.TrimPrefix(text, "import ")
			// Scala uses '.' separator and '{a, b}' groups, identical structure to Rust;
			// borrow the same expansion helper after normalising '::' to '.'.
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

// ----- Lua -----------------------------------------------------------------

// local foo = require("foo.bar")
// local baz = require "baz"
func luaImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "function_call" || n.Type() == "call" {
			fname := nodeText(n.ChildByFieldName("name"), src)
			if fname == "" {
				// some Lua grammars expose the callee positionally
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

// ----- Elixir --------------------------------------------------------------

// alias Foo.Bar
// import Foo.Bar
// use Foo.Bar
// require Foo
func elixirImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "call" {
			// Tree-sitter Elixir represents `alias Foo.Bar` as call(target=alias, args=Foo.Bar).
			target := ""
			if t := n.ChildByFieldName("target"); t != nil {
				target = nodeText(t, src)
			} else if c := n.NamedChild(0); c != nil {
				target = nodeText(c, src)
			}
			switch target {
			case "alias", "import", "use", "require":
				// pull first identifier-like argument
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

// ----- HCL / Terraform -----------------------------------------------------

// module "x" { source = "./modules/y" }
// provider "aws" { ... }
func hclImports(root *sitter.Node, src []byte) []string {
	var out []string
	walk(root, func(n *sitter.Node) bool {
		if n.Type() == "block" {
			// Block type is the first identifier child.
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
			// attribute = identifier "=" expression
			if name := c.NamedChild(0); name != nil && nodeText(name, src) == "source" {
				collectFirstStringChild(c, src, out)
			}
		}
		return true
	})
}

// ----- helpers -------------------------------------------------------------

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
