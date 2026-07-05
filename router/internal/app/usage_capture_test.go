package app

import "testing"

func TestUsageCaptureParsesNonStreamingJSON(t *testing.T) {
	body := []byte(`{
		"id": "cmpl-1",
		"object": "chat.completion",
		"choices": [],
		"usage": {
			"prompt_tokens": 1000,
			"completion_tokens": 250,
			"total_tokens": 1250,
			"prompt_tokens_details": {"cached_tokens": 400}
		}
	}`)

	capture := newUsageCapture()
	if _, err := capture.Write(body); err != nil {
		t.Fatalf("write capture: %v", err)
	}
	usage := capture.usage("application/json")
	if !usage.HasUsage {
		t.Fatal("expected usage to be detected")
	}
	if usage.InputUnits != 1000 || usage.OutputUnits != 250 || usage.CachedInputUnits != 400 || usage.TotalUnits != 1250 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}

func TestUsageCaptureParsesStreamingUsageFrame(t *testing.T) {
	stream := "" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":8,\"total_tokens\":20}}\n\n" +
		"data: [DONE]\n\n"

	capture := newUsageCapture()
	if _, err := capture.Write([]byte(stream)); err != nil {
		t.Fatalf("write capture: %v", err)
	}
	usage := capture.usage("text/event-stream; charset=utf-8")
	if !usage.HasUsage {
		t.Fatal("expected streaming usage to be detected")
	}
	if usage.InputUnits != 12 || usage.OutputUnits != 8 || usage.TotalUnits != 20 {
		t.Fatalf("unexpected streaming usage: %+v", usage)
	}
}

func TestUsageCaptureHandlesResponsesAPITokens(t *testing.T) {
	body := []byte(`{"usage":{"input_tokens":7,"output_tokens":3,"input_tokens_details":{"cached_tokens":2}}}`)
	capture := newUsageCapture()
	if _, err := capture.Write(body); err != nil {
		t.Fatalf("write capture: %v", err)
	}
	usage := capture.usage("application/json")
	if usage.InputUnits != 7 || usage.OutputUnits != 3 || usage.CachedInputUnits != 2 || usage.TotalUnits != 10 {
		t.Fatalf("unexpected responses-api usage: %+v", usage)
	}
}

func TestUsageCaptureNoUsageReturnsEmpty(t *testing.T) {
	capture := newUsageCapture()
	if _, err := capture.Write([]byte(`{"id":"cmpl-1","choices":[]}`)); err != nil {
		t.Fatalf("write capture: %v", err)
	}
	usage := capture.usage("application/json")
	if usage.HasUsage {
		t.Fatalf("expected no usage, got %+v", usage)
	}
}

func TestUsageCaptureTruncatedJSONDoesNotPanic(t *testing.T) {
	capture := newUsageCapture()
	capture.headLimit = 8
	capture.tailLimit = 8
	if _, err := capture.Write([]byte(`{"usage":{"prompt_tokens":5}}`)); err != nil {
		t.Fatalf("write capture: %v", err)
	}
	// With both windows truncated the parser must fail gracefully.
	if usage := capture.usage("application/json"); usage.HasUsage {
		t.Fatalf("expected truncated capture to report no usage, got %+v", usage)
	}
}
