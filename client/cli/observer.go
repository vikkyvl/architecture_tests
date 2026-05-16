package cli

import (
	"fmt"
	"os"
	"time"
)

type cliReviewObserver struct {
	maxToolCalls int
	timeout      time.Duration
}

func (o cliReviewObserver) LimitReached(reason string) {
	switch reason {
	case "max tool calls":
		fmt.Fprintln(os.Stderr, warnStyle.Render(fmt.Sprintf("Limit reached: max %d tool calls", o.maxToolCalls)))
	case "timeout":
		fmt.Fprintln(os.Stderr, warnStyle.Render(fmt.Sprintf("Limit reached: timeout %s", o.timeout)))
	default:
		fmt.Fprintln(os.Stderr, warnStyle.Render("Limit reached: "+reason))
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
	fmt.Fprintln(os.Stderr, renderStep("Analysis complete"))
}
