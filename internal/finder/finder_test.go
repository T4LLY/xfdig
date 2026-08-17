package finder

import (
	"context"
	"testing"

	gh "github.com/T4LLY/xfdig/internal/github"
)

type fakeGitHub struct{}

func (fakeGitHub) SearchClosedIssues(context.Context, string, int) ([]gh.Issue, string, error) {
	return []gh.Issue{
		{Repo: "acme/tool", Number: 7, Title: "stderr pipe deadlock", URL: "https://github.com/acme/tool/issues/7", Body: "child hangs when stderr fills", Closed: true, Rank: 1},
		{Repo: "other/tool", Number: 3, Title: "unrelated", URL: "https://github.com/other/tool/issues/3", Closed: true, Rank: 2},
	}, "hybrid", nil
}

func (fakeGitHub) ClosingPullRequests(_ context.Context, issue gh.Issue) ([]gh.PullRequest, error) {
	if issue.Number == 7 {
		return []gh.PullRequest{
			{Repo: "acme/tool", Number: 11, Title: "drain stderr pipe concurrently", URL: "https://github.com/acme/tool/pull/11", State: "MERGED", MergedAt: "2026-08-01T00:00:00Z"},
			{Repo: "acme/tool", Number: 12, Title: "abandoned attempt", URL: "https://github.com/acme/tool/pull/12", State: "CLOSED"},
		}, nil
	}
	return nil, nil
}

func TestFindReturnsOnlyMergedLinkedPRs(t *testing.T) {
	result, err := New(fakeGitHub{}).Find(context.Background(), "stderr pipe deadlock", 5)
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
}
