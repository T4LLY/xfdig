package main

import (
	"bytes"
	"errors"
	"strings"
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

func TestParseCLIDefaultLimitIsTwenty(t *testing.T) {
	cfg, err := parseCLI([]string{"go", "deadlock"}, fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Limit != 20 {
		t.Fatalf("limit=%d", cfg.Limit)
	}
}

func TestParseLimitAcceptsOneToOneHundred(t *testing.T) {
	for _, raw := range []string{"1", "100"} {
		if _, err := parseLimit(raw); err != nil {
			t.Fatalf("parseLimit(%q): %v", raw, err)
		}
	}
	for _, raw := range []string{"0", "101"} {
		if _, err := parseLimit(raw); err == nil {
			t.Fatalf("parseLimit(%q) unexpectedly succeeded", raw)
		}
	}
}

func TestParseCLIAcceptsShortEqualsBounds(t *testing.T) {
	cfg, err := parseCLI([]string{"go", "-s=14d", "-u=7d", "deadlock"}, fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Since != "2026-08-03" || cfg.Until != "2026-08-10" {
		t.Fatalf("unexpected bounds: %#v", cfg)
	}
}

func TestParseCLIRejectsFlagWhereOptionValueIsRequired(t *testing.T) {
	_, err := parseCLI([]string{"go", "-n", "-t", "deadlock"}, fixedNow())
	if err == nil || err.Error() != "-n requires a value" {
		t.Fatalf("err=%v", err)
	}
}

func TestParseCLIAcceptsQueryBeginningWithHyphenAfterLanguage(t *testing.T) {
	cfg, err := parseCLI([]string{"go", "-32603 internal error"}, fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Query != "-32603 internal error" {
		t.Fatalf("query=%q", cfg.Query)
	}
}

func TestParseCLIAcceptsEndOfOptionsBeforeHyphenQuery(t *testing.T) {
	cfg, err := parseCLI([]string{"go", "--", "--unexpected flag from upstream"}, fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Query != "--unexpected flag from upstream" {
		t.Fatalf("query=%q", cfg.Query)
	}
}

func TestParseCLIRejectsUnknownOptionBeforeLanguage(t *testing.T) {
	_, err := parseCLI([]string{"--bogus", "go", "deadlock"}, fixedNow())
	if err == nil || !strings.Contains(err.Error(), "unknown option") {
		t.Fatalf("err=%v", err)
	}
}

func TestRunArgsEmitsStructuredFailureWhenGHIsMissing(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runArgs(
		[]string{"go", "deadlock"},
		fixedNow(),
		&stdout,
		&stderr,
		func(string) (string, error) { return "", errors.New("not found") },
		nil,
	)
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	for _, want := range []string{
		`"status":"failure"`,
		`"q":"deadlock"`,
		`"language":"go"`,
		`"fixes":[]`,
		`"error":"GitHub CLI (gh) is required and was not found in PATH"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %s: %s", want, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "GitHub CLI (gh) is required") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}
