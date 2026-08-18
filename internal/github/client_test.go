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

	issues, info, err := client.SearchClosedIssues(context.Background(), SearchOptions{
		Query:    "stderr deadlock",
		Language: "go",
		Since:    "2025-01-01",
		Until:    "2026-01-01",
		Limit:    5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.Type != "hybrid" {
		t.Fatalf("mode=%q", info.Type)
	}
	if len(info.Warnings) != 0 {
		t.Fatalf("warnings=%#v", info.Warnings)
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

	_, info, err := client.SearchClosedIssues(context.Background(), SearchOptions{Query: "bug", Language: "rust", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if info.Type != "lexical_fallback" {
		t.Fatalf("mode=%q", info.Type)
	}
	if len(info.Warnings) != 1 || !strings.Contains(info.Warnings[0], "used lexical fallback") {
		t.Fatalf("warnings=%#v", info.Warnings)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls=%d", len(runner.calls))
	}
}

func TestSearchClosedIssuesWarnsWhenGitHubPerformsLexicalSearch(t *testing.T) {
	runner := &fakeRunner{
		responses: [][]byte{[]byte(`{"search_type":"lexical","items":[]}`)},
		errors:    []error{nil},
	}
	client := NewClient(runner)

	_, info, err := client.SearchClosedIssues(context.Background(), SearchOptions{Query: "bug", Language: "go", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if info.Type != "lexical" {
		t.Fatalf("mode=%q", info.Type)
	}
	if len(info.Warnings) != 1 || !strings.Contains(info.Warnings[0], "instead of requested hybrid") {
		t.Fatalf("warnings=%#v", info.Warnings)
	}
}

func TestSearchClosedIssuesReportsIncompleteResults(t *testing.T) {
	runner := &fakeRunner{
		responses: [][]byte{[]byte(`{"search_type":"hybrid","incomplete_results":true,"items":[]}`)},
		errors:    []error{nil},
	}
	client := NewClient(runner)

	_, info, err := client.SearchClosedIssues(context.Background(), SearchOptions{Query: "bug", Language: "go", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if info.Type != "hybrid" {
		t.Fatalf("mode=%q", info.Type)
	}
	if len(info.Warnings) != 1 || !strings.Contains(info.Warnings[0], "incomplete results") {
		t.Fatalf("warnings=%#v", info.Warnings)
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

func TestClosingPullRequestsUsesRawFieldsForStringVariables(t *testing.T) {
	runner := &fakeRunner{
		responses: [][]byte{[]byte(`{"data":{"repository":{"issue":{"closedByPullRequestsReferences":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}}}`)},
		errors:    []error{nil},
	}
	client := NewClient(runner)

	_, err := client.ClosingPullRequests(context.Background(), Issue{Repo: "123/2024", Number: 7})
	if err != nil {
		t.Fatal(err)
	}
	args := runner.calls[0]
	for _, want := range []string{"owner=123", "name=2024"} {
		found := false
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "-f" && args[i+1] == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing raw string field %q in %#v", want, args)
		}
	}
}

func TestClosingPullRequestsPaginatesAllResults(t *testing.T) {
	runner := &fakeRunner{
		responses: [][]byte{
			[]byte(`{"data":{"repository":{"issue":{"closedByPullRequestsReferences":{"pageInfo":{"hasNextPage":true,"endCursor":"CURSOR1"},"nodes":[{"number":11,"title":"first","url":"https://github.com/acme/tool/pull/11","state":"MERGED","mergedAt":"2026-08-01T00:00:00Z","repository":{"nameWithOwner":"acme/tool"}}]}}}}}`),
			[]byte(`{"data":{"repository":{"issue":{"closedByPullRequestsReferences":{"pageInfo":{"hasNextPage":false,"endCursor":"CURSOR2"},"nodes":[{"number":12,"title":"second","url":"https://github.com/acme/tool/pull/12","state":"MERGED","mergedAt":"2026-08-02T00:00:00Z","repository":{"nameWithOwner":"acme/tool"}}]}}}}}`),
		},
		errors: []error{nil, nil},
	}
	client := NewClient(runner)

	prs, err := client.ClosingPullRequests(context.Background(), Issue{Repo: "acme/tool", Number: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 2 || prs[0].Number != 11 || prs[1].Number != 12 {
		t.Fatalf("unexpected prs: %#v", prs)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls=%d", len(runner.calls))
	}
	joined := strings.Join(runner.calls[1], " ")
	if !strings.Contains(joined, "cursor=CURSOR1") {
		t.Fatalf("second call missing cursor: %s", joined)
	}
}

func TestClosingPullRequestsRejectsInvalidPaginationCursor(t *testing.T) {
	runner := &fakeRunner{
		responses: [][]byte{[]byte(`{"data":{"repository":{"issue":{"closedByPullRequestsReferences":{"pageInfo":{"hasNextPage":true,"endCursor":""},"nodes":[]}}}}}`)},
		errors:    []error{nil},
	}
	client := NewClient(runner)

	_, err := client.ClosingPullRequests(context.Background(), Issue{Repo: "acme/tool", Number: 7})
	if err == nil || !strings.Contains(err.Error(), "invalid cursor") {
		t.Fatalf("err=%v", err)
	}
}
