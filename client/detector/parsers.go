package detector

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/elixir"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/hcl"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/kotlin"
	"github.com/smacker/go-tree-sitter/lua"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/scala"
	"github.com/smacker/go-tree-sitter/sql"
	"github.com/smacker/go-tree-sitter/swift"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// LanguageGrammars maps the canonical language name used in shared/constants.LangExtensions
// to its tree-sitter grammar. Adding a language: import the grammar package and add one
// entry here; no other detector code needs changing. Spec 0002 expanded this from 7 to 19.
var LanguageGrammars = map[string]*sitter.Language{
	"php":        php.GetLanguage(),
	"typescript": typescript.GetLanguage(),
	"java":       java.GetLanguage(),
	"csharp":     csharp.GetLanguage(),
	"python":     python.GetLanguage(),
	"go":         golang.GetLanguage(),
	"javascript": javascript.GetLanguage(),
	"ruby":       ruby.GetLanguage(),
	"kotlin":     kotlin.GetLanguage(),
	"swift":      swift.GetLanguage(),
	"rust":       rust.GetLanguage(),
	"c":          c.GetLanguage(),
	"cpp":        cpp.GetLanguage(),
	"scala":      scala.GetLanguage(),
	"lua":        lua.GetLanguage(),
	"bash":       bash.GetLanguage(),
	"elixir":     elixir.GetLanguage(),
	"hcl":        hcl.GetLanguage(),
	"sql":        sql.GetLanguage(),
}

// ExtraExtensionGrammars maps secondary extensions (where one language has a dedicated
// grammar variant — e.g. TSX vs TS) to their grammar. Used by the detector when the
// extension is more specific than the canonical language entry.
var ExtraExtensionGrammars = map[string]*sitter.Language{
	"tsx": tsx.GetLanguage(),
}
