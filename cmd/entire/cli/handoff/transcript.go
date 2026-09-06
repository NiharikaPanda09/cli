package handoff

import (
	"bytes"
	"encoding/json"
	"strings"
)

const (
	statusError   = "error"
	statusSuccess = "success"
)

type toolCall struct {
	Line     int
	Name     string
	Input    map[string]any
	Status   string
	Output   string
	Followup string
}

type transcriptLine struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Content json.RawMessage `json:"content"`
}

type contentBlock struct {
	Type   string          `json:"type"`
	Text   string          `json:"text"`
	Name   string          `json:"name"`
	Input  map[string]any  `json:"input"`
	Result json.RawMessage `json:"result"`
}

type toolResult struct {
	Output string `json:"output"`
	Status string `json:"status"`
}

func scanToolCalls(transcript []byte, from int) []toolCall {
	if len(transcript) == 0 {
		return nil
	}
	lines := bytes.Split(transcript, []byte("\n"))
	if from > 0 && from < len(lines) {
		lines = lines[from:]
	}

	var calls []toolCall
	seen := make(map[string]bool)

	for i, raw := range lines {
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		var tl transcriptLine
		if err := json.Unmarshal(raw, &tl); err != nil {
			continue
		}
		if tl.Type != "assistant" || len(tl.Content) == 0 {
			continue
		}
		var blocks []contentBlock
		if err := json.Unmarshal(tl.Content, &blocks); err != nil {
			continue
		}

		lineNo := from + i
		trailing := trailingText(blocks)

		for bi, b := range blocks {
			if b.Type != "tool_use" || len(b.Result) == 0 {
				continue
			}
			var res toolResult
			if err := json.Unmarshal(b.Result, &res); err != nil {
				continue
			}
			if res.Status != statusError {
				continue
			}
			if key := dedupeKey(tl.ID, bi); key != "" {
				if seen[key] {
					continue
				}
				seen[key] = true
			}
			calls = append(calls, toolCall{
				Line:     lineNo,
				Name:     b.Name,
				Input:    b.Input,
				Status:   res.Status,
				Output:   res.Output,
				Followup: trailing,
			})
		}
	}
	return calls
}

func dedupeKey(id string, blockIndex int) string {
	if id == "" {
		return ""
	}
	return id + ":" + itoa(blockIndex)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func trailingText(blocks []contentBlock) string {
	for i := len(blocks) - 1; i >= 0; i-- {
		if blocks[i].Type == "text" {
			if t := strings.TrimSpace(blocks[i].Text); t != "" {
				return t
			}
		}
	}
	return ""
}

func groupKey(c toolCall) string {
	if c.Input != nil {
		for _, field := range []string{"command", "file_path", "path", "pattern"} {
			if v, ok := c.Input[field].(string); ok {
				if s := strings.TrimSpace(v); s != "" {
					return s
				}
			}
		}
	}
	if c.Name != "" {
		return c.Name
	}
	return "unknown tool"
}
