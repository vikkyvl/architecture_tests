package constants

var LangExtensions = map[string][]string{
	"php":        {"php"},
	"typescript": {"ts", "tsx"},
	"java":       {"java"},
	"csharp":     {"cs"},
	"python":     {"py"},
	"go":         {"go"},
	"javascript": {"js", "jsx"},
	"ruby":       {"rb"},
	"kotlin":     {"kt"},
	"swift":      {"swift"},
	"rust":       {"rs"},
	"c":          {"c", "h"},
	"cpp":        {"cpp", "cc", "cxx", "hpp"},
	"scala":      {"scala"},
	"lua":        {"lua"},
	"bash":       {"sh", "bash"},
	"elixir":     {"ex", "exs"},
	"hcl":        {"tf", "hcl"},
	"sql":        {"sql"},
}

var ExtToLang = map[string]string{
	"php": "php", "ts": "typescript", "tsx": "typescript",
	"java": "java", "cs": "csharp", "go": "go",
	"py": "python", "rb": "ruby", "js": "javascript",
	"jsx": "javascript",
	"kt":  "kotlin", "swift": "swift", "rs": "rust",
	"c": "c", "h": "c",
	"cpp": "cpp", "cc": "cpp", "cxx": "cpp", "hpp": "cpp",
	"scala": "scala",
	"lua":   "lua",
	"sh":    "bash", "bash": "bash",
	"ex": "elixir", "exs": "elixir",
	"tf": "hcl", "hcl": "hcl",
	"sql": "sql",
}

var LangFileExtensions = map[string]string{
	"php": ".php", "typescript": ".ts", "python": ".py",
	"java": ".java", "csharp": ".cs",
	"go": ".go", "javascript": ".js",
	"ruby": ".rb", "kotlin": ".kt", "swift": ".swift", "rust": ".rs",
	"c": ".c", "cpp": ".cpp", "scala": ".scala",
	"lua": ".lua", "bash": ".sh", "elixir": ".ex",
	"hcl": ".tf", "sql": ".sql",
}
