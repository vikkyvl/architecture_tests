package constants

import (
	"os"
	"strings"
	"time"
)

const (
	envBoolTrue = "true"
	envBoolOne  = "1"
)

func IsEnvTrue(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case envBoolOne, envBoolTrue:
		return true
	default:
		return false
	}
}

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
	ToolCallBegin = "TOOL_CALL_BEGIN"
	ToolCallEnd   = "TOOL_CALL_END"
)

const (
	LimitReasonMaxToolCalls = "max tool calls"
	LimitReasonTimeout      = "timeout"
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
	DefaultSrcRoot   = "src"
	DefaultEnvFile   = ".env"
	DefaultPHPFile   = ".php"
	EmptyJSONObject  = "{" + "}"
	UnknownValue     = "unknown"
	JSONIndent       = "  "
	ReportTimeLayout = "2006-01-02 15:04:05"
	TruncationSuffix = "..."
	FormatInt        = "%d"
	FormatChars      = "%d chars"
)

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
