package constants

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
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleModel     = "model"
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
	HTTPHeaderRetryAfter        = "Retry-After"
)

const (
	HTTPStatusOverloaded = 529
)

const (
	EventLLMTurn            = "<llm_turn>"
	EventRateLimit          = "<rate_limit>"
	EventPreemptiveSleep    = "<preemptive_sleep>"
	RateLimitKind429        = "rate_limited"
	RateLimitKindOverloaded = "overloaded"
	RateLimitKindPreemptive = "preemptive"
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
