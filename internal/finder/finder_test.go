package finder

import (
	"context"
	"testing"

	gh "github.com/T4LLY/xfdig/internal/github"
)

type fakeGitHub struct {
	search gh.SearchOptions
}

func (f *fakeGitHub) SearchClosedIssues(_ context.Context, options gh.SearchOptions) ([]gh.Issue, string, error) {
	f.search = options
	return []gh.Issue{
		{Repo: "acme/tool", Number: 7, Title: "stderr pipe deadlock", URL: "https://github.com/acme/tool/issues/7", Body: "child hangs when stderr fills", Closed: true, Rank: 1},
		{Repo: "other/tool", Number: 3, Title: "unrelated", URL: "https://github.com/other/tool/issues/3", Closed: true, Rank: 2},
	}, "hybrid", nil
}

func (f *fakeGitHub) ClosingPullRequests(_ context.Context, issue gh.Issue) ([]gh.PullRequest, error) {
	if issue.Number == 7 {
		return []gh.PullRequest{
			{Repo: "acme/tool", Number: 11, Title: "drain stderr pipe concurrently", URL: "https://github.com/acme/tool/pull/11", State: "MERGED", MergedAt: "2026-08-01T00:00:00Z"},
			{Repo: "acme/tool", Number: 12, Title: "abandoned attempt", URL: "https://github.com/acme/tool/pull/12", State: "CLOSED"},
		}, nil
	}
	return nil, nil
}

func TestFindReturnsOnlyMergedLinkedPRs(t *testing.T) {
	github := &fakeGitHub{}
	result, err := New(github).Find(context.Background(), "stderr pipe deadlock", Options{
		Language: "go",
		Since:    "2025-08-17",
		Until:    "2026-08-17",
		Limit:    5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fixes) != 1 {
		t.Fatalf("fixes=%d", len(result.Fixes))
	}
	fix := result.Fixes[0]
	if fix.PR != 11 || !fix.Evidence.PRMerged || !fix.Evidence.PRLinked || !fix.Evidence.IssueClosed {
		t.Fatalf("unexpected fix: %#v", fix)
	}
	if len(fix.Evidence.MatchedTerms) == 0 {
		t.Fatalf("expected matched terms")
	}
	if result.Language != "go" || result.Since != "2025-08-17" || result.Until != "2026-08-17" {
		t.Fatalf("unexpected result filters: %#v", result)
	}
	if github.search.Language != "go" || github.search.Since != "2025-08-17" || github.search.Until != "2026-08-17" {
		t.Fatalf("unexpected search options: %#v", github.search)
	}
	if github.search.Limit != 20 {
		t.Fatalf("search limit=%d", github.search.Limit)
	}
}
