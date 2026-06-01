package output

import (
	"fmt"
	"io"
	"os"
)

type OutputTransport struct {
	Progress io.Writer
	Result   io.Writer
}

func NewOutputTransport() *OutputTransport {
	return &OutputTransport{Progress: os.Stderr, Result: os.Stdout}
}

func (t *OutputTransport) WriteProgress(s string) { _, _ = fmt.Fprintln(t.Progress, s) }
func (t *OutputTransport) WriteResult(s string)   { _, _ = fmt.Fprintln(t.Result, s) }
func (t *OutputTransport) WriteProgressBlank()    { _, _ = fmt.Fprintln(t.Progress) }
