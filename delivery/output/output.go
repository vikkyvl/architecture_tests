package output

import (
	"fmt"
	"io"
	"os"
)

// OutputTransport carries the two output channels of the delivery layer:
// Progress for human-facing diagnostics (→ stderr) and Result for
// machine-readable summaries (→ stdout).
type OutputTransport struct {
	Progress io.Writer
	Result   io.Writer
}

func NewOutputTransport() *OutputTransport {
	return &OutputTransport{Progress: os.Stderr, Result: os.Stdout}
}

func (t *OutputTransport) WriteProgress(s string) { fmt.Fprintln(t.Progress, s) }
func (t *OutputTransport) WriteResult(s string)   { fmt.Fprintln(t.Result, s) }
func (t *OutputTransport) WriteProgressBlank()    { fmt.Fprintln(t.Progress) }
