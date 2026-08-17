package main

import (
	"testing"
	"time"
)

func fixedNow() time.Time {
	return time.Date(2026, time.August, 17, 21, 22, 0, 0, time.FixedZone("JST", 9*60*60))
}

func TestParseCLIWithLanguageAndRelativeBounds(t *testing.T) {
	cfg, err := parseCLI([]string{"go", "--since", "2y", "--until", "6m", "-n", "8", "stderr pipe deadlock"}, fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Language != "go" || cfg.Query != "stderr pipe deadlock" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.Since != "2024-08-17" || cfg.Until != "2026-02-17" {
		t.Fatalf("unexpected bounds: %#v", cfg)
	}
	if cfg.Limit != 8 {
		t.Fatalf("limit=%d", cfg.Limit)
	}
}

func TestParseCLIAcceptsAbsoluteBoundsAndFlagsBeforeLanguage(t *testing.T) {
	cfg, err := parseCLI([]string{"--since=2025-01-02", "-t", "csharp", "--until=2026-03-04", "texture update failure"}, fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Language != "csharp" || cfg.Since != "2025-01-02" || cfg.Until != "2026-03-04" || !cfg.Text {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestParseCLIRejectsInvertedBounds(t *testing.T) {
	_, err := parseCLI([]string{"rust", "--since", "3m", "--until", "1y", "shutdown hangs"}, fixedNow())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveTimeUsesCalendarMonths(t *testing.T) {
	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	got, err := resolveTime("1m", now)
	if err != nil {
		t.Fatal(err)
	}
	if got != "2026-02-28" {
		t.Fatalf("got=%s", got)
	}
}

func TestParseCLIRequiresLanguageAndQuery(t *testing.T) {
	if _, err := parseCLI([]string{"go"}, fixedNow()); err == nil {
		t.Fatal("expected error")
	}
}
