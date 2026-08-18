package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/T4LLY/xfdig/internal/finder"
)

func TestJSONIncludesFiltersAndPRURLForCLIChaining(t *testing.T) {
	result := finder.Result{
		Status:     finder.StatusSuccess,
		Query:      "deadlock",
		Language:   "go",
		Since:      "2025-01-01",
		Until:      "2026-01-01",
		SearchType: "hybrid",
		Fixes: []finder.Fix{{
			Repo: "acme/tool", PR: 11, URL: "https://github.com/acme/tool/pull/11", Title: "fix",
		}},
	}
	var buf bytes.Buffer
	if err := JSON(&buf, result); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"status":"success"`,
		`"language":"go"`,
		`"since":"2025-01-01"`,
		`"until":"2026-01-01"`,
		`"url":"https://github.com/acme/tool/pull/11"`,
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("json missing %s: %s", want, buf.String())
		}
	}
}

func TestJSONIncludesFailureStatusAndError(t *testing.T) {
	result := finder.Result{
		Status:   finder.StatusFailure,
		Query:    "deadlock",
		Language: "go",
		Fixes:    []finder.Fix{},
		Error:    "search failed",
	}
	var buf bytes.Buffer
	if err := JSON(&buf, result); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"status":"failure"`, `"fixes":[]`, `"error":"search failed"`} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("json missing %s: %s", want, buf.String())
		}
	}
}

func TestTextIncludesStatus(t *testing.T) {
	result := finder.Result{Status: finder.StatusWarning, Query: "deadlock", Language: "go", SearchType: "hybrid", Fixes: []finder.Fix{}, Warnings: []string{"partial lookup"}}
	var buf bytes.Buffer
	if err := Text(&buf, result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "status: warning\n") {
		t.Fatalf("text missing status: %s", buf.String())
	}
}
