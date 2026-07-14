package app

import (
	"bytes"
	"encoding/json"
	"strings"
)

const (
	// usageCaptureHeadLimit bounds how many leading bytes of a non-streaming
	// response are retained so the top-level `usage` object can be parsed
	// without buffering an unbounded body.
	usageCaptureHeadLimit = 1 << 20 // 1 MiB
	// usageCaptureTailLimit bounds how many trailing bytes are retained for
	// streaming (SSE) responses, where the usage frame is emitted last.
	usageCaptureTailLimit = 64 << 10 // 64 KiB
)

// relayUsage is the token accounting extracted from an upstream response. The
// values feed dynamic, request-cost based billing.
type relayUsage struct {
	InputUnits       int64
	CachedInputUnits int64
	OutputUnits      int64
	TotalUnits       int64
	HasUsage         bool
}

// usageCapture tees upstream response bytes so token usage can be extracted
// while the response is streamed to the client. It keeps a bounded prefix for
// non-streaming JSON responses and a bounded suffix for streaming SSE frames,
// so memory stays bounded regardless of response size.
type usageCapture struct {
	head      bytes.Buffer
	headFull  bool // true once the prefix limit was reached (body is truncated)
	tail      []byte
	headLimit int
	tailLimit int
}

func newUsageCapture() *usageCapture {
	return &usageCapture{headLimit: usageCaptureHeadLimit, tailLimit: usageCaptureTailLimit}
}

// Write implements io.Writer. It never returns an error so that teeing usage
// capture cannot interfere with delivering the response to the client.
func (c *usageCapture) Write(p []byte) (int, error) {
	if c == nil {
		return len(p), nil
	}
	if c.head.Len() < c.headLimit {
		remaining := c.headLimit - c.head.Len()
		if len(p) <= remaining {
			c.head.Write(p)
		} else {
			c.head.Write(p[:remaining])
			c.headFull = true
		}
	} else {
		c.headFull = true
	}
	c.appendTail(p)
	return len(p), nil
}

func (c *usageCapture) appendTail(p []byte) {
	if c.tailLimit <= 0 {
		return
	}
	if len(p) >= c.tailLimit {
		c.tail = append(c.tail[:0], p[len(p)-c.tailLimit:]...)
		return
	}
	combined := append(c.tail, p...)
	if len(combined) > c.tailLimit {
		combined = combined[len(combined)-c.tailLimit:]
	}
	c.tail = combined
}

// usage extracts token usage from the captured bytes. The content type selects
// the parsing strategy: SSE responses carry usage in the final data frame,
// while JSON responses expose a single top-level usage object.
func (c *usageCapture) usage(contentType string) relayUsage {
	if c == nil {
		return relayUsage{}
	}
	if isEventStream(contentType) {
		if u, ok := parseStreamingUsage(c.tail); ok {
			return u
		}
		return relayUsage{}
	}
	if !c.headFull {
		if u, ok := parseJSONUsage(c.head.Bytes()); ok {
			return u
		}
	}
	// Fallback: the response may be JSON without a declared content type, or an
	// SSE stream misreported as JSON. Scanning the retained tail recovers usage
	// emitted near the end of the body.
	if u, ok := parseStreamingUsage(c.tail); ok {
		return u
	}
	return relayUsage{}
}

func isEventStream(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "text/event-stream")
}

// upstreamUsage models the usage object emitted by OpenAI-compatible chat and
// completion responses as well as the Responses API (input/output tokens).
type upstreamUsage struct {
	PromptTokens        int64 `json:"prompt_tokens"`
	CompletionTokens    int64 `json:"completion_tokens"`
	TotalTokens         int64 `json:"total_tokens"`
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	PromptTokensDetails *struct {
		CachedTokens int64 `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	InputTokensDetails *struct {
		CachedTokens int64 `json:"cached_tokens"`
	} `json:"input_tokens_details"`
}

func (u upstreamUsage) toRelayUsage() (relayUsage, bool) {
	input := u.PromptTokens
	if input == 0 {
		input = u.InputTokens
	}
	output := u.CompletionTokens
	if output == 0 {
		output = u.OutputTokens
	}
	total := u.TotalTokens

	var cached int64
	if u.PromptTokensDetails != nil {
		cached = u.PromptTokensDetails.CachedTokens
	}
	if cached == 0 && u.InputTokensDetails != nil {
		cached = u.InputTokensDetails.CachedTokens
	}

	if input == 0 && output == 0 && total == 0 {
		return relayUsage{}, false
	}
	if total == 0 {
		total = input + output
	}
	if cached > input {
		cached = input
	}
	return relayUsage{
		InputUnits:       input,
		CachedInputUnits: cached,
		OutputUnits:      output,
		TotalUnits:       total,
		HasUsage:         true,
	}, true
}

// parseJSONUsage parses a single JSON object with a top-level usage field.
func parseJSONUsage(body []byte) (relayUsage, bool) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return relayUsage{}, false
	}
	var envelope struct {
		Usage *upstreamUsage `json:"usage"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return relayUsage{}, false
	}
	if envelope.Usage == nil {
		return relayUsage{}, false
	}
	return envelope.Usage.toRelayUsage()
}

// parseStreamingUsage scans SSE `data:` frames and returns the usage from the
// last frame that carries a usable usage object. The retained tail may begin
// mid-frame; incomplete leading frames simply fail to parse and are skipped.
func parseStreamingUsage(buf []byte) (relayUsage, bool) {
	if len(buf) == 0 {
		return relayUsage{}, false
	}
	var (
		last  relayUsage
		found bool
	)
	for _, line := range bytes.Split(buf, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(line[len("data:"):])
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		if u, ok := parseJSONUsage(payload); ok {
			last = u
			found = true
		}
	}
	return last, found
}
