package llm

import (
	"encoding/json"

	c "github.com/archguard/project/shared/constants"
)

const (
	contentBlockJSONType         = "type"
	contentBlockJSONToolUseID    = "tool_use_id"
	contentBlockJSONContent      = "content"
	contentBlockJSONIsError      = "is_error"
	contentBlockJSONCacheControl = "cache_control"
)

type Request struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    []SystemBlock `json:"system,omitempty"`
	Messages  []Message     `json:"messages"`
	Tools     []Tool        `json:"tools,omitempty"`
}

type SystemBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

type CacheControl struct {
	Type string `json:"type"`
}

type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type         string
	Text         string
	ID           string
	Name         string
	Input        json.RawMessage
	ToolUseID    string
	Content      string
	IsError      bool
	CacheControl *CacheControl
}

type toolUseContentBlockJSON struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type textContentBlockJSON struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (b *ContentBlock) MarshalJSON() ([]byte, error) {
	switch b.Type {
	case c.ContentTypeToolUse:
		return json.Marshal(toolUseContentBlockJSON{
			Type:  b.Type,
			ID:    b.ID,
			Name:  b.Name,
			Input: b.Input,
		})
	case c.ContentTypeToolResult:
		m := map[string]interface{}{
			contentBlockJSONType:      b.Type,
			contentBlockJSONToolUseID: b.ToolUseID,
			contentBlockJSONContent:   b.Content,
		}
		if b.IsError {
			m[contentBlockJSONIsError] = true
		}
		if b.CacheControl != nil {
			m[contentBlockJSONCacheControl] = b.CacheControl
		}
		return json.Marshal(m)
	default:
		return json.Marshal(textContentBlockJSON{
			Type: b.Type,
			Text: b.Text,
		})
	}
}

func (b *ContentBlock) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type         string          `json:"type"`
		Text         string          `json:"text"`
		ID           string          `json:"id"`
		Name         string          `json:"name"`
		Input        json.RawMessage `json:"input"`
		ToolUseID    string          `json:"tool_use_id"`
		Content      string          `json:"content"`
		IsError      bool            `json:"is_error"`
		CacheControl *CacheControl   `json:"cache_control"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	b.Type = raw.Type
	b.Text = raw.Text
	b.ID = raw.ID
	b.Name = raw.Name
	b.Input = raw.Input
	b.ToolUseID = raw.ToolUseID
	b.Content = raw.Content
	b.IsError = raw.IsError
	b.CacheControl = raw.CacheControl
	return nil
}

type Response struct {
	ID         string         `json:"id"`
	Role       string         `json:"role"`
	Content    []ContentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Usage      Usage          `json:"usage"`
}

type Usage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadTokens     int `json:"cache_read_input_tokens,omitempty"`
}

type Tool struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema"`
	CacheControl *CacheControl   `json:"cache_control,omitempty"`
}

func NewSystemBlock(text string, cache bool) SystemBlock {
	sb := SystemBlock{
		Type: c.ContentTypeText,
		Text: text,
	}
	if cache {
		sb.CacheControl = &CacheControl{Type: c.CacheControlEphemeral}
	}
	return sb
}

func SystemText(blocks []SystemBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	if len(blocks) == 1 {
		return blocks[0].Text
	}
	var total int
	for _, b := range blocks {
		total += len(b.Text)
	}
	out := make([]byte, 0, total)
	for _, b := range blocks {
		out = append(out, b.Text...)
	}
	return string(out)
}

func NewTextBlock(text string) ContentBlock {
	return ContentBlock{
		Type: c.ContentTypeText,
		Text: text,
	}
}

func NewToolResult(toolUseID, content string, isError bool) ContentBlock {
	return ContentBlock{
		Type:      c.ContentTypeToolResult,
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
	}
}
