package output

import (
	"fmt"
	"io"
	"time"
)

const rateLimitFrames = "|/-\\"

func AnimateRateLimitWait(out io.Writer, wait time.Duration) {
	animateWait(out, wait, renderRateLimitWaitFrame, renderRateLimitResume)
}

func AnimateProactivePauseWait(out io.Writer, wait time.Duration) {
	animateWait(out, wait, renderProactivePauseWaitFrame, renderProactivePauseResume)
}

func animateWait(out io.Writer, wait time.Duration, frameFn func(time.Duration, string) string, doneFn func(time.Duration) string) {
	if wait <= 0 {
		fmt.Fprintln(out, doneFn(wait))
		return
	}

	deadline := time.Now().Add(wait)
	timer := time.NewTimer(wait)
	defer timer.Stop()
	ticker := time.NewTicker(rateLimitTick)
	defer ticker.Stop()

	for frame := 0; ; frame++ {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		fmt.Fprint(out, terminalClearLine)
		fmt.Fprint(out, frameFn(remaining, string(rateLimitFrames[frame%len(rateLimitFrames)])))
		select {
		case <-timer.C:
			fmt.Fprint(out, terminalClearLine)
			fmt.Fprintln(out, doneFn(wait))
			return
		case <-ticker.C:
		}
	}

	fmt.Fprint(out, terminalClearLine)
	fmt.Fprintln(out, doneFn(wait))
}
