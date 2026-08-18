package finder

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	gh "github.com/T4LLY/xfdig/internal/github"
)

type fakeGitHub struct {
	search gh.SearchOptions
}

func (f *fakeGitHub) SearchClosedIssues(_ context.Context, options gh.SearchOptions) ([]gh.Issue, gh.SearchInfo, error) {
	f.search = options
	return []gh.Issue{
		{Repo: "acme/tool", Number: 7, Title: "stderr pipe deadlock", URL: "https://github.com/acme/tool/issues/7", Body: "child hangs when stderr fills", Closed: true, Rank: 1},
		{Repo: "other/tool", Number: 3, Title: "unrelated", URL: "https://github.com/other/tool/issues/3", Closed: true, Rank: 2},
	}, gh.SearchInfo{Type: "hybrid"}, nil
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
	if result.Status != StatusSuccess {
		t.Fatalf("status=%q", result.Status)
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

func TestFindCapsIssueSearchAt100(t *testing.T) {
	github := &fakeGitHub{}
	_, err := New(github).Find(context.Background(), "deadlock", Options{Language: "go", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if github.search.Limit != 100 {
		t.Fatalf("search limit=%d, want 100", github.search.Limit)
	}
}

type manyPRGitHub struct{}

func (manyPRGitHub) SearchClosedIssues(_ context.Context, _ gh.SearchOptions) ([]gh.Issue, gh.SearchInfo, error) {
	issues := make([]gh.Issue, 10)
	for i := range issues {
		issues[i] = gh.Issue{
			Repo:   "acme/tool",
			Number: i + 1,
			Title:  "deadlock",
			URL:    "https://github.com/acme/tool/issues/1",
			Closed: true,
			Rank:   i + 1,
		}
	}
	return issues, gh.SearchInfo{Type: "hybrid"}, nil
}

func (manyPRGitHub) ClosingPullRequests(_ context.Context, issue gh.Issue) ([]gh.PullRequest, error) {
	prs := make([]gh.PullRequest, 3)
	for i := range prs {
		number := issue.Number*10 + i
		prs[i] = gh.PullRequest{
			Repo:     "acme/tool",
			Number:   number,
			Title:    "fix deadlock",
			URL:      "https://github.com/acme/tool/pull/" + fmt.Sprint(number),
			State:    "MERGED",
			MergedAt: "2026-08-01T00:00:00Z",
		}
	}
	return prs, nil
}

func TestFindDoesNotDeadlockWhenMatchesExceedIssueBuffer(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		result, err := New(manyPRGitHub{}).Find(context.Background(), "deadlock", Options{Language: "go", Limit: 100})
		if err == nil && len(result.Fixes) != 30 {
			err = fmt.Errorf("fixes=%d, want 30", len(result.Fixes))
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Find deadlocked while collecting merged PRs")
	}
}

type cancelGitHub struct{}

func (cancelGitHub) SearchClosedIssues(_ context.Context, _ gh.SearchOptions) ([]gh.Issue, gh.SearchInfo, error) {
	issues := make([]gh.Issue, 8)
	for i := range issues {
		issues[i] = gh.Issue{Repo: "acme/tool", Number: i + 1, Rank: i + 1}
	}
	return issues, gh.SearchInfo{Type: "hybrid"}, nil
}

func (cancelGitHub) ClosingPullRequests(ctx context.Context, _ gh.Issue) ([]gh.PullRequest, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestFindReturnsContextDeadlineError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := New(cancelGitHub{}).Find(ctx, "deadlock", Options{Language: "go", Limit: 5})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v, want context deadline exceeded", err)
	}
}

func TestFindRejectsInvalidLibraryDateBounds(t *testing.T) {
	_, err := New(&fakeGitHub{}).Find(context.Background(), "deadlock", Options{
		Language: "go",
		Since:    "2026-01-01 is:open",
		Limit:    5,
	})
	if err == nil || !strings.Contains(err.Error(), "since must be YYYY-MM-DD") {
		t.Fatalf("err=%v", err)
	}
}

func TestMatchedTermsKeepsTwoRuneNonASCIITerms(t *testing.T) {
	terms := matchedTerms("排他", "排他制御の不具合を修正")
	if len(terms) != 1 || terms[0] != "排他" {
		t.Fatalf("terms=%#v", terms)
	}
}

type searchWarningGitHub struct{}

func (searchWarningGitHub) SearchClosedIssues(_ context.Context, _ gh.SearchOptions) ([]gh.Issue, gh.SearchInfo, error) {
	return nil, gh.SearchInfo{
		Type:     "hybrid",
		Warnings: []string{"GitHub issue search returned incomplete results"},
	}, nil
}

func (searchWarningGitHub) ClosingPullRequests(_ context.Context, _ gh.Issue) ([]gh.PullRequest, error) {
	return nil, nil
}

func TestFindStatusWarningOnSearchWarning(t *testing.T) {
	result, err := New(searchWarningGitHub{}).Find(context.Background(), "deadlock", Options{Language: "go", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusWarning {
		t.Fatalf("status=%q", result.Status)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "incomplete results") {
		t.Fatalf("warnings=%#v", result.Warnings)
	}
}

type lookupStatusGitHub struct {
	failAll bool
}

func (lookupStatusGitHub) SearchClosedIssues(_ context.Context, _ gh.SearchOptions) ([]gh.Issue, gh.SearchInfo, error) {
	return []gh.Issue{
		{Repo: "acme/tool", Number: 1, Title: "deadlock", Closed: true, Rank: 1},
		{Repo: "acme/tool", Number: 2, Title: "deadlock", Closed: true, Rank: 2},
	}, gh.SearchInfo{Type: "hybrid"}, nil
}

func (f lookupStatusGitHub) ClosingPullRequests(_ context.Context, issue gh.Issue) ([]gh.PullRequest, error) {
	if f.failAll || issue.Number == 2 {
		return nil, errors.New("lookup failed")
	}
	return nil, nil
}

func TestFindStatusWarningOnPartialLookupFailure(t *testing.T) {
	result, err := New(lookupStatusGitHub{}).Find(context.Background(), "deadlock", Options{Language: "go", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusWarning {
		t.Fatalf("status=%q", result.Status)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("warnings=%#v", result.Warnings)
	}
}

func TestFindStatusFailureWhenAllLookupsFail(t *testing.T) {
	result, err := New(lookupStatusGitHub{failAll: true}).Find(context.Background(), "deadlock", Options{Language: "go", Limit: 5})
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Status != StatusFailure {
		t.Fatalf("status=%q", result.Status)
	}
	if result.Error != "all closing PR lookups failed" {
		t.Fatalf("error=%q", result.Error)
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("warnings=%#v", result.Warnings)
	}
}

type searchFailureGitHub struct{}

func (searchFailureGitHub) SearchClosedIssues(_ context.Context, _ gh.SearchOptions) ([]gh.Issue, gh.SearchInfo, error) {
	return nil, gh.SearchInfo{}, errors.New("search failed")
}

func (searchFailureGitHub) ClosingPullRequests(_ context.Context, _ gh.Issue) ([]gh.PullRequest, error) {
	return nil, nil
}

func TestFindStatusFailureOnSearchError(t *testing.T) {
	result, err := New(searchFailureGitHub{}).Find(context.Background(), "deadlock", Options{Language: "go", Limit: 5})
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Status != StatusFailure || result.Error != "search failed" {
		t.Fatalf("result=%#v", result)
	}
}
