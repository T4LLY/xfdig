package finder

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	gh "github.com/T4LLY/xfdig/internal/github"
)

type GitHub interface {
	SearchClosedIssues(ctx context.Context, options gh.SearchOptions) ([]gh.Issue, gh.SearchInfo, error)
	ClosingPullRequests(ctx context.Context, issue gh.Issue) ([]gh.PullRequest, error)
}

type Finder struct {
	github GitHub
}

func New(github GitHub) *Finder {
	return &Finder{github: github}
}

type Options struct {
	Language string
	Since    string
	Until    string
	Limit    int
}

const (
	StatusSuccess = "success"
	StatusWarning = "warning"
	StatusFailure = "failure"
)

type Evidence struct {
	IssueRank    int      `json:"issue_rank"`
	IssueClosed  bool     `json:"issue_closed"`
	PRLinked     bool     `json:"pr_linked"`
	PRMerged     bool     `json:"pr_merged"`
	MatchedTerms []string `json:"matched_terms,omitempty"`
}

type IssueRef struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	URL    string `json:"url"`
}

type Fix struct {
	Repo     string   `json:"repo"`
	PR       int      `json:"pr"`
	URL      string   `json:"url"`
	Title    string   `json:"title"`
	MergedAt string   `json:"merged_at"`
	Issue    IssueRef `json:"issue"`
	Evidence Evidence `json:"evidence"`
}

type Result struct {
	Status     string   `json:"status"`
	Query      string   `json:"q"`
	Language   string   `json:"language"`
	Since      string   `json:"since,omitempty"`
	Until      string   `json:"until,omitempty"`
	SearchType string   `json:"search_type,omitempty"`
	Fixes      []Fix    `json:"fixes"`
	Warnings   []string `json:"warnings,omitempty"`
	Error      string   `json:"error,omitempty"`
}

func (f *Finder) Find(ctx context.Context, query string, options Options) (Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Result{}, fmt.Errorf("query is empty")
	}
	language := strings.TrimSpace(options.Language)
	if language == "" {
		return Result{}, fmt.Errorf("language is empty")
	}
	if options.Limit < 1 {
		return Result{}, fmt.Errorf("limit must be at least 1")
	}
	if err := validateDateBounds(options.Since, options.Until); err != nil {
		return Result{}, err
	}

	result := Result{
		Status:   StatusSuccess,
		Query:    query,
		Language: language,
		Since:    options.Since,
		Until:    options.Until,
		Fixes:    make([]Fix, 0),
	}

	issueLimit := options.Limit * 4
	if issueLimit < 12 {
		issueLimit = 12
	}
	if issueLimit > 100 {
		issueLimit = 100
	}

	issues, searchInfo, err := f.github.SearchClosedIssues(ctx, gh.SearchOptions{
		Query:    query,
		Language: language,
		Since:    options.Since,
		Until:    options.Until,
		Limit:    issueLimit,
	})
	if err != nil {
		result.Status = StatusFailure
		result.Error = err.Error()
		return result, err
	}
	result.SearchType = searchInfo.Type

	fixes := make([]Fix, 0, options.Limit)
	warnings := append([]string(nil), searchInfo.Warnings...)
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	var mu sync.Mutex
	successfulLookups := 0

	for _, issue := range issues {
		issue := issue
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			prs, err := f.github.ClosingPullRequests(ctx, issue)
			if err != nil {
				mu.Lock()
				warnings = append(warnings, fmt.Sprintf("%s#%d: %v", issue.Repo, issue.Number, err))
				mu.Unlock()
				return
			}
			mu.Lock()
			successfulLookups++
			mu.Unlock()

			local := make([]Fix, 0, len(prs))
			for _, pr := range prs {
				if pr.MergedAt == "" {
					continue
				}
				text := issue.Title + " " + issue.Body + " " + pr.Title
				local = append(local, Fix{
					Repo:     pr.Repo,
					PR:       pr.Number,
					URL:      pr.URL,
					Title:    pr.Title,
					MergedAt: pr.MergedAt,
					Issue: IssueRef{
						Repo:   issue.Repo,
						Number: issue.Number,
						URL:    issue.URL,
					},
					Evidence: Evidence{
						IssueRank:    issue.Rank,
						IssueClosed:  issue.Closed,
						PRLinked:     true,
						PRMerged:     true,
						MatchedTerms: matchedTerms(query, text),
					},
				})
			}
			if len(local) != 0 {
				mu.Lock()
				fixes = append(fixes, local...)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	sort.Strings(warnings)
	if err := ctx.Err(); err != nil {
		err = fmt.Errorf("find fixes: %w", err)
		result.Status = StatusFailure
		result.Warnings = warnings
		result.Error = err.Error()
		return result, err
	}
	if len(issues) > 0 && successfulLookups == 0 {
		err := fmt.Errorf("all closing PR lookups failed")
		result.Status = StatusFailure
		result.Warnings = warnings
		result.Error = err.Error()
		return result, err
	}

	sort.SliceStable(fixes, func(i, j int) bool {
		if fixes[i].Evidence.IssueRank != fixes[j].Evidence.IssueRank {
			return fixes[i].Evidence.IssueRank < fixes[j].Evidence.IssueRank
		}
		if len(fixes[i].Evidence.MatchedTerms) != len(fixes[j].Evidence.MatchedTerms) {
			return len(fixes[i].Evidence.MatchedTerms) > len(fixes[j].Evidence.MatchedTerms)
		}
		return fixes[i].URL < fixes[j].URL
	})

	fixes = dedupe(fixes)
	if len(fixes) > options.Limit {
		fixes = fixes[:options.Limit]
	}
	result.Fixes = fixes
	result.Warnings = warnings
	if len(warnings) > 0 {
		result.Status = StatusWarning
	}
	return result, nil
}

func dedupe(fixes []Fix) []Fix {
	seen := make(map[string]struct{}, len(fixes))
	out := make([]Fix, 0, len(fixes))
	for _, fix := range fixes {
		key := fix.URL
		if key == "" {
			key = fmt.Sprintf("%s#%d", fix.Repo, fix.PR)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, fix)
	}
	return out
}

func matchedTerms(query, text string) []string {
	text = strings.ToLower(text)
	seen := map[string]struct{}{}
	var terms []string
	for _, term := range tokenize(query) {
		if !isUsefulTerm(term) || isStopword(term) {
			continue
		}
		if strings.Contains(text, strings.ToLower(term)) {
			lower := strings.ToLower(term)
			if _, ok := seen[lower]; ok {
				continue
			}
			seen[lower] = struct{}{}
			terms = append(terms, lower)
		}
	}
	if len(terms) > 8 {
		terms = terms[:8]
	}
	return terms
}

func tokenize(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == ':' || r == '.')
	})
}

func isStopword(s string) bool {
	switch strings.ToLower(s) {
	case "the", "and", "for", "with", "from", "when", "then", "that", "this", "into", "while", "does", "not", "are", "was", "were", "but", "can", "could", "should", "would", "has", "have", "had", "after", "before":
		return true
	default:
		return false
	}
}

func validateDateBounds(since, until string) error {
	if since != "" {
		if _, err := time.Parse("2006-01-02", since); err != nil {
			return fmt.Errorf("since must be YYYY-MM-DD")
		}
	}
	if until != "" {
		if _, err := time.Parse("2006-01-02", until); err != nil {
			return fmt.Errorf("until must be YYYY-MM-DD")
		}
	}
	if since != "" && until != "" && since > until {
		return fmt.Errorf("since must not be later than until")
	}
	return nil
}

func isUsefulTerm(term string) bool {
	runes := []rune(term)
	if len(runes) >= 3 {
		return true
	}
	if len(runes) < 2 {
		return false
	}
	for _, r := range runes {
		if r > unicode.MaxASCII {
			return true
		}
	}
	return false
}
