package constants

import (
	"path/filepath"
	"strings"
	"time"
)

const (
	AppName    = "archguard"
	AppVersion = "0.1.0"
)

const (
	DefaultMaxTokens    = 4096
	DefaultMaxToolCalls = 100
	DefaultTimeout      = 10 * time.Minute
	MaxPromptLength     = 15000
	TruncateLength      = 200
	FilePermission      = 0644
	FileCountBudget     = 999
)

const (
	ProviderAnthropic = "anthropic"
	ProviderGemini    = "gemini"
	ProviderOpenAI    = "openai"
)

const (
	AnthropicBaseURL    = "https://api.anthropic.com/v1/messages"
	AnthropicVersion    = "2023-06-01"
	AnthropicModel      = "claude-opus-4-1-20250805"
	AnthropicTimeout    = 120 * time.Second
	AnthropicMaxRetries = 5
	AnthropicRetryWait  = 60 * time.Second
)

const (
	GeminiBaseURL    = "https://generativelanguage.googleapis.com/v1beta/models"
	GeminiModel      = "gemini-2.5-flash"
	GeminiTimeout    = 120 * time.Second
	GeminiMaxRetries = 5
	GeminiRetryWait  = 15 * time.Second
)

const (
	OpenAIBaseURL    = "https://api.openai.com/v1/chat/completions"
	OpenAIModel      = "gpt-4o"
	OpenAITimeout    = 120 * time.Second
	OpenAIMaxRetries = 5
	OpenAIRetryWait  = 15 * time.Second
)

const (
	NotionBaseURL       = "https://api.notion.com/v1"
	NotionVersion       = "2022-06-28"
	NotionVersionHeader = "Notion-Version"
	NotionTimeout       = 30 * time.Second
	NotionMaxResults    = 3
	NotionMaxContentLen = 3000
)

const (
	ToolCallBegin = "TOOL_CALL_BEGIN"
	ToolCallEnd   = "TOOL_CALL_END"
)

const (
	EnvAnthropicKey = "ANTHROPIC_API_KEY"
	EnvGeminiKey    = "GEMINI_API_KEY"
	EnvOpenAIKey    = "OPENAI_API_KEY"
)

const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
)

const (
	CategoryLayerDep   = "layer_dependency"
	CategoryDomainRule = "domain_rule"
	CategorySOLID      = "solid"
	CategoryGRASP      = "grasp"
)

const (
	DecisionAutoApproved = "auto_approved"
	DecisionUserAllowed  = "user_allowed"
	DecisionSessionAllow = "session_allowed"
	DecisionProjectAllow = "project_allowed"
	DecisionGlobalAllow  = "global_allowed"
	DecisionUserDenied   = "user_denied"
	DecisionBlocked      = "blocked"
)

const (
	StopReasonEndTurn = "end_turn"
	StopReasonToolUse = "tool_use"
)

const (
	ContentTypeText       = "text"
	ContentTypeToolUse    = "tool_use"
	ContentTypeToolResult = "tool_result"
)

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleModel     = "model"
)

const (
	DefaultSrcRoot   = "src"
	DefaultEnvFile   = ".env"
	DefaultPHPFile   = ".php"
	UnknownValue     = "unknown"
	JSONIndent       = "  "
	ReportTimeLayout = "2006-01-02 15:04:05"
	TruncationSuffix = "..."
)

const (
	HTTPMethodPost              = "POST"
	HTTPHeaderContentType       = "Content-Type"
	HTTPContentTypeJSON         = "application/json"
	HTTPHeaderAnthropicAPIKey   = "x-api-key"
	HTTPHeaderAnthropicVersion  = "anthropic-version"
	HTTPHeaderCFAccessClientID  = "CF-Access-Client-Id"
	HTTPHeaderCFAccessClientSec = "CF-Access-Client-Secret"
	HTTPHeaderAuthorization     = "Authorization"
	HTTPAuthBearer              = "Bearer "
)

const (
	ToolGetProjectStructure  = "get_project_structure"
	ToolReadFile             = "read_file"
	ToolGetArchitectureRules = "get_architecture_rules"
	ToolGetClassDependencies = "get_class_dependencies"
	ToolGetDocumentation     = "get_documentation"
	ToolGetExternalContext   = "get_external_context"
	ToolReportViolation      = "report_violation"
)

const (
	ArgPath        = "path"
	ArgClass       = "class"
	ArgQuery       = "query"
	ArgSystem      = "system"
	ArgFile        = "file"
	ArgLine        = "line"
	ArgSeverity    = "severity"
	ArgCategory    = "category"
	ArgRule        = "rule"
	ArgDescription = "description"
	ArgSuggestion  = "suggestion"
)

const (
	JSONKeyRoot         = "root"
	JSONKeyFiles        = "files"
	JSONKeyTotal        = "total"
	JSONKeyPath         = "path"
	JSONKeyName         = "name"
	JSONKeyExtension    = "extension"
	JSONKeySize         = "size"
	JSONKeyLayers       = "layers"
	JSONKeyRules        = "rules"
	JSONKeyDomain       = "domain"
	JSONKeyDomainDesc   = "domain_desc"
	JSONKeyDomainRules  = "domain_rules"
	JSONKeyClass        = "class"
	JSONKeyFile         = "file"
	JSONKeyLayer        = "layer"
	JSONKeyDependencies = "dependencies"
	JSONKeyType         = "type"
	JSONKeyResult       = "result"
	JSONValueImport     = "import"
)

var SkipDirs = []string{
	"node_modules", "vendor", ".git", "__pycache__",
	"build", "dist", "var", "cache", ".idea", ".vscode",
}

var BlacklistPaths = []string{
	".env", "*.pem", "*.key", "secrets/**",
}

var AutoApprovedTools = map[string]bool{
	ToolGetProjectStructure:  true,
	ToolGetArchitectureRules: true,
	ToolGetClassDependencies: true,
	ToolReportViolation:      true,
}

var SeverityOrder = map[string]int{
	SeverityCritical: 0,
	SeverityHigh:     1,
	SeverityMedium:   2,
	SeverityLow:      3,
}

var SeverityList = []string{
	SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow,
}

var LangExtensions = map[string][]string{
	"php":        {"php"},
	"typescript": {"ts", "tsx"},
	"java":       {"java"},
	"csharp":     {"cs"},
	"python":     {"py"},
	"go":         {"go"},
	"javascript": {"js", "jsx"},
}

var ExtToLang = map[string]string{
	"php": "php", "ts": "typescript", "tsx": "typescript",
	"java": "java", "cs": "csharp", "go": "go",
	"py": "python", "rb": "ruby", "js": "javascript",
	"kt": "kotlin", "swift": "swift", "rs": "rust",
}

func IsSensitivePath(path string) bool {
	normalized := filepath.ToSlash(filepath.Clean(path))
	if strings.HasPrefix(filepath.Base(normalized), ".") {
		return true
	}
	for _, pattern := range BlacklistPaths {
		trimmed := strings.TrimSuffix(filepath.ToSlash(pattern), "/**")
		if strings.Contains(normalized, trimmed) {
			return true
		}
		if strings.HasPrefix(pattern, "*.") {
			if strings.HasSuffix(normalized, strings.TrimPrefix(pattern, "*")) {
				return true
			}
		}
	}
	return false
}

var LangFileExtensions = map[string]string{
	"php": ".php", "typescript": ".ts", "python": ".py",
	"java": ".java", "csharp": ".cs",
}
