package perf

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type Span struct {
	Name       string `json:"name"`
	DurationMs int64  `json:"durationMs"`
}

type StartupReport struct {
	ProcessStartUnixMs int64  `json:"processStartUnixMs"`
	GeneratedAtUnixMs  int64  `json:"generatedAtUnixMs"`
	TotalStartupMs     int64  `json:"totalStartupMs"`
	Spans              []Span `json:"spans"`
}

type Recorder struct {
	processStart time.Time

	mu    sync.Mutex
	spans []Span
}

func NewRecorder(processStart time.Time) *Recorder {
	return &Recorder{
		processStart: processStart,
		spans:        make([]Span, 0, 8),
	}
}

func (r *Recorder) AddDuration(name string, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if duration < 0 {
		duration = 0
	}

	ms := duration.Milliseconds()
	if ms == 0 && duration > 0 {
		ms = 1
	}

	r.spans = append(r.spans, Span{
		Name:       name,
		DurationMs: ms,
	})
}

func (r *Recorder) Snapshot() StartupReport {
	r.mu.Lock()
	defer r.mu.Unlock()

	spansCopy := make([]Span, len(r.spans))
	copy(spansCopy, r.spans)

	return StartupReport{
		ProcessStartUnixMs: r.processStart.UnixMilli(),
		GeneratedAtUnixMs:  time.Now().UnixMilli(),
		TotalStartupMs:     time.Since(r.processStart).Milliseconds(),
		Spans:              spansCopy,
	}
}

func FormatReport(report StartupReport) string {
	var b strings.Builder
	b.WriteString("perf report\n")

	for _, span := range report.Spans {
		b.WriteString(fmt.Sprintf("  %-16s %dms\n", span.Name, span.DurationMs))
	}

	b.WriteString(fmt.Sprintf("  %-16s %dms\n", "total_startup", report.TotalStartupMs))
	return b.String()
}
