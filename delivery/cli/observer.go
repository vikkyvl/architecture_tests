package cli

import (
	"fmt"
	"time"

	"github.com/archguard/project/delivery/output"
	c "github.com/archguard/project/shared/constants"
)

const (
	formatLimitMaxTools = "Limit reached: max %d tool calls"
	formatLimitTimeout  = "Limit reached: timeout %s"
	formatLimitGeneric  = "Limit reached: %s"
	msgAnalysisComplete = "Analysis complete"
)

type cliReviewObserver struct {
	out          *output.OutputTransport
	maxToolCalls int
	timeout      time.Duration
}

func (o cliReviewObserver) LimitReached(reason string) {
	switch reason {
	case c.LimitReasonMaxToolCalls:
		o.out.WriteProgress(output.WarnStyle.Render(fmt.Sprintf(formatLimitMaxTools, o.maxToolCalls)))
	case c.LimitReasonTimeout:
		o.out.WriteProgress(output.WarnStyle.Render(fmt.Sprintf(formatLimitTimeout, o.timeout)))
	default:
		o.out.WriteProgress(output.WarnStyle.Render(fmt.Sprintf(formatLimitGeneric, reason)))
	}
}

func (o cliReviewObserver) Retry(provider string, attempt, maxAttempts int, wait time.Duration, err error) {
	o.out.WriteProgress(output.RenderRetry(provider, attempt, maxAttempts, wait, err))
}

func (o cliReviewObserver) LLMText(text string) {
	o.out.WriteProgress(output.RenderLLMText(text))
}

func (o cliReviewObserver) ToolResult(callNumber int, toolName string, resultSize int, err error) {
	o.out.WriteProgress(output.RenderToolResult(callNumber, toolName, resultSize, err))
}

func (o cliReviewObserver) AnalysisComplete() {
	o.out.WriteProgress(output.RenderStep(msgAnalysisComplete))
}
