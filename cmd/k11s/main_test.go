package main

import (
	"strings"
	"testing"

	"github.com/dzhanguzin/k11s/internal/protocol"
)

func TestFormatStatusBarLive(t *testing.T) {
	meta := protocol.FreshnessMeta{
		State:              protocol.FreshnessStateLive,
		SnapshotTimeUnixMs: 1_700_000_000_000,
		AgeMs:              250,
		WatchHealthy:       true,
		Source:             "cache",
	}

	got := formatStatusBar(meta, false)
	if !strings.Contains(got, "[LIVE]") {
		t.Fatalf("expected live badge, got %q", got)
	}
	if !strings.Contains(got, "age=250ms") {
		t.Fatalf("expected age, got %q", got)
	}
	if !strings.Contains(got, "watch=healthy") {
		t.Fatalf("expected watch health, got %q", got)
	}
}

func TestFormatStatusBarStaleDistinct(t *testing.T) {
	meta := protocol.FreshnessMeta{
		State:              protocol.FreshnessStateStale,
		SnapshotTimeUnixMs: 1_700_000_000_000,
		AgeMs:              181000,
		WatchHealthy:       false,
		Source:             "cache-stale",
	}

	got := formatStatusBar(meta, false)
	if !strings.Contains(got, "!!! STALE !!!") {
		t.Fatalf("expected stale marker, got %q", got)
	}
	if !strings.Contains(got, "source=cache-stale") {
		t.Fatalf("expected stale source, got %q", got)
	}
	if !strings.Contains(got, "watch=degraded") {
		t.Fatalf("expected degraded watch health, got %q", got)
	}
}
