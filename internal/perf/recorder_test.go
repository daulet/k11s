package perf

import (
	"testing"
	"time"
)

func TestRecorderCapturesSpans(t *testing.T) {
	t.Parallel()

	start := time.Now().Add(-100 * time.Millisecond)
	recorder := NewRecorder(start)
	recorder.AddDuration("process_start", 0)
	recorder.AddDuration("daemon_connect", 12*time.Millisecond)
	recorder.AddDuration("handshake", 3*time.Millisecond)

	report := recorder.Snapshot()
	if len(report.Spans) != 3 {
		t.Fatalf("expected 3 spans, got %d", len(report.Spans))
	}
	if report.Spans[1].Name != "daemon_connect" {
		t.Fatalf("unexpected span name: %q", report.Spans[1].Name)
	}
	if report.Spans[1].DurationMs <= 0 {
		t.Fatalf("expected daemon_connect duration > 0")
	}
	if report.TotalStartupMs <= 0 {
		t.Fatalf("expected total startup duration > 0")
	}
}
