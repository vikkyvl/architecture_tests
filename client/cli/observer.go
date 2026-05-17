package cli

import (
	"fmt"
	"os"
	"time"

	c "github.com/archguard/project/shared/constants"
)

const (
	formatLimitMaxTools = "Limit reached: max %d tool calls"
	formatLimitTimeout  = "Limit reached: timeout %s"
	formatLimitGeneric  = "Limit reached: %s"
	msgAnalysisComplete = "Analysis complete"
)

type cliReviewObserver struct {
	maxToolCalls int
	timeout      time.Duration
}

func (o cliReviewObserver) LimitReached(reason string) {
	switch reason {
	case c.LimitReasonMaxToolCalls:
		fmt.Fprintln(os.Stderr, warnStyle.Render(fmt.Sprintf(formatLimitMaxTools, o.maxToolCalls)))
	case c.LimitReasonTimeout:
		fmt.Fprintln(os.Stderr, warnStyle.Render(fmt.Sprintf(formatLimitTimeout, o.timeout)))
	default:
		fmt.Fprintln(os.Stderr, warnStyle.Render(fmt.Sprintf(formatLimitGeneric, reason)))
	}
}

func (cliReviewObserver) Retry(provider string, attempt, maxAttempts int, wait time.Duration, err error) {
	fmt.Fprintln(os.Stderr, renderRetry(provider, attempt, maxAttempts, wait, err))
}

func (cliReviewObserver) LLMText(text string) {
	fmt.Fprintln(os.Stderr, renderLLMText(text))
}

func (cliReviewObserver) ToolResult(callNumber int, toolName string, resultSize int, err error) {
	fmt.Fprintln(os.Stderr, renderToolResult(callNumber, toolName, resultSize, err))
}

func (cliReviewObserver) AnalysisComplete() {
	fmt.Fprintln(os.Stderr, renderStep(msgAnalysisComplete))
}
