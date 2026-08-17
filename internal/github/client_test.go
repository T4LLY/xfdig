package github

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls     [][]string
	responses [][]byte
	errors    []error
}

func (f *fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	i := len(f.calls) - 1
	return f.responses[i], f.errors[i]
}

func TestSearchClosedIssuesHybridWithFilters(t *testing.T) {
	runner := &fakeRunner{
		responses: [][]byte{[]byte(`{"search_type":"hybrid","items":[{"number":7,"title":"pipe deadlock","html_url":"https://github.com/acme/tool/issues/7","state":"closed","body":"stderr fills","repository_url":"https://api.github.com/repos/acme/tool"}]}`)},
		errors:    []error{nil},
	}
	client := NewClient(runner)

	issues, mode, err := client.SearchClosedIssues(context.Background(), SearchOptions{
		Query:    "stderr deadlock",
		Language: "go",
		Since:    "2025-01-01",
		Until:    "2026-01-01",
		Limit:    5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if mode != "hybrid" {
		t.Fatalf("mode=%q", mode)
	}
	if len(issues) != 1 || issues[0].Repo != "acme/tool" || issues[0].Rank != 1 {
		t.Fatalf("unexpected issues: %#v", issues)
	}
	joined := strings.Join(runner.calls[0], " ")
	for _, want := range []string{
		"search_type=hybrid",
		"is:issue is:closed",
		"language:go",
		"closed:>=2025-01-01",
		"closed:<=2026-01-01",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
}

func TestSearchClosedIssuesAnyLanguageOmitsLanguageQualifier(t *testing.T) {
	runner := &fakeRunner{
		responses: [][]byte{[]byte(`{"items":[]}`)},
		errors:    []error{nil},
	}
	client := NewClient(runner)

	_, _, err := client.SearchClosedIssues(context.Background(), SearchOptions{Query: "bug", Language: "any", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(runner.calls[0], " ")
	if strings.Contains(joined, "language:") {
		t.Fatalf("unexpected language qualifier: %s", joined)
	}
}

func TestSearchClosedIssuesQuotesLanguageWithSpaces(t *testing.T) {
	q := buildIssueQuery(SearchOptions{Query: "bug", Language: "Visual Basic"})
	if !strings.Contains(q, `language:"Visual Basic"`) {
		t.Fatalf("query=%q", q)
	}
}

func TestSearchClosedIssuesFallsBackToLexical(t *testing.T) {
	runner := &fakeRunner{
		responses: [][]byte{nil, []byte(`{"items":[]}`)},
		errors:    []error{errors.New("unsupported"), nil},
	}
	client := NewClient(runner)

	_, mode, err := client.SearchClosedIssues(context.Background(), SearchOptions{Query: "bug", Language: "rust", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if mode != "lexical_fallback" {
		t.Fatalf("mode=%q", mode)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls=%d", len(runner.calls))
	}
}

func TestClosingPullRequests(t *testing.T) {
	runner := &fakeRunner{
		responses: [][]byte{[]byte(`{"data":{"repository":{"issue":{"closed":true,"closedByPullRequestsReferences":{"nodes":[{"number":11,"title":"drain both pipes","url":"https://github.com/acme/tool/pull/11","state":"MERGED","mergedAt":"2026-08-01T00:00:00Z","repository":{"nameWithOwner":"acme/tool"}}]}}}}}`)},
		errors:    []error{nil},
	}
	client := NewClient(runner)

	prs, err := client.ClosingPullRequests(context.Background(), Issue{Repo: "acme/tool", Number: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 1 || prs[0].Number != 11 || prs[0].MergedAt == "" {
		t.Fatalf("unexpected prs: %#v", prs)
	}
}
