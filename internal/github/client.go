package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"unicode"
)

const closingPRQuery = `query($owner:String!,$name:String!,$number:Int!,$cursor:String){
  repository(owner:$owner,name:$name){
    issue(number:$number){
      closedByPullRequestsReferences(first:100,after:$cursor,includeClosedPrs:true){
        pageInfo{hasNextPage endCursor}
        nodes{
          number
          title
          url
          state
          mergedAt
          repository{nameWithOwner}
        }
      }
    }
  }
}`

type Runner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		stderr := strings.TrimSpace(string(exitErr.Stderr))
		if stderr != "" {
			return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), stderr)
		}
	}
	return nil, fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
}

type Client struct {
	runner Runner
}

func NewClient(r Runner) *Client {
	return &Client{runner: r}
}

type SearchOptions struct {
	Query    string
	Language string
	Since    string
	Until    string
	Limit    int
}

type SearchInfo struct {
	Type     string
	Warnings []string
}

type Issue struct {
	Repo   string
	Number int
	Title  string
	URL    string
	Body   string
	Closed bool
	Rank   int
}

type PullRequest struct {
	Repo     string
	Number   int
	Title    string
	URL      string
	State    string
	MergedAt string
}

type searchResponse struct {
	SearchType        string `json:"search_type"`
	IncompleteResults bool   `json:"incomplete_results"`
	Items             []struct {
		Number        int    `json:"number"`
		Title         string `json:"title"`
		HTMLURL       string `json:"html_url"`
		State         string `json:"state"`
		Body          string `json:"body"`
		RepositoryURL string `json:"repository_url"`
	} `json:"items"`
}

type graphqlResponse struct {
	Data struct {
		Repository *struct {
			Issue *struct {
				ClosedByPullRequestsReferences struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []struct {
						Number     int     `json:"number"`
						Title      string  `json:"title"`
						URL        string  `json:"url"`
						State      string  `json:"state"`
						MergedAt   *string `json:"mergedAt"`
						Repository struct {
							NameWithOwner string `json:"nameWithOwner"`
						} `json:"repository"`
					} `json:"nodes"`
				} `json:"closedByPullRequestsReferences"`
			} `json:"issue"`
		} `json:"repository"`
	} `json:"data"`
}

func (c *Client) SearchClosedIssues(ctx context.Context, options SearchOptions) ([]Issue, SearchInfo, error) {
	limit := options.Limit
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	q := buildIssueQuery(options)
	args := []string{
		"api", "--method", "GET", "search/issues",
		"-f", "q=" + q,
		"-f", "search_type=hybrid",
		"-F", "per_page=" + strconv.Itoa(limit),
	}
	out, err := c.runner.Run(ctx, args...)
	info := SearchInfo{Type: "hybrid"}
	if err != nil {
		hybridErr := err
		// GitHub Enterprise Server or older API surfaces may not accept
		// search_type. Fall back to ordinary lexical issue search.
		fallback := []string{
			"api", "--method", "GET", "search/issues",
			"-f", "q=" + q,
			"-F", "per_page=" + strconv.Itoa(limit),
		}
		out, err = c.runner.Run(ctx, fallback...)
		info.Type = "lexical_fallback"
		if err != nil {
			return nil, SearchInfo{}, err
		}
		info.Warnings = append(info.Warnings, fmt.Sprintf("hybrid issue search failed; used lexical fallback: %v", hybridErr))
	}

	var response searchResponse
	if err := json.Unmarshal(out, &response); err != nil {
		return nil, SearchInfo{}, fmt.Errorf("decode issue search response: %w", err)
	}
	if response.SearchType != "" && info.Type != "lexical_fallback" {
		info.Type = response.SearchType
		if !strings.EqualFold(response.SearchType, "hybrid") {
			info.Warnings = append(info.Warnings, fmt.Sprintf("GitHub performed %s issue search instead of requested hybrid search", response.SearchType))
		}
	}
	if response.IncompleteResults {
		info.Warnings = append(info.Warnings, "GitHub issue search returned incomplete results")
	}

	issues := make([]Issue, 0, len(response.Items))
	for i, item := range response.Items {
		repo, ok := repoFromAPIURL(item.RepositoryURL)
		if !ok {
			continue
		}
		issues = append(issues, Issue{
			Repo:   repo,
			Number: item.Number,
			Title:  item.Title,
			URL:    item.HTMLURL,
			Body:   item.Body,
			Closed: strings.EqualFold(item.State, "closed"),
			Rank:   i + 1,
		})
	}
	return issues, info, nil
}

func buildIssueQuery(options SearchOptions) string {
	parts := []string{strings.TrimSpace(options.Query), "is:issue", "is:closed"}
	language := strings.TrimSpace(options.Language)
	if language != "" && !strings.EqualFold(language, "any") {
		parts = append(parts, "language:"+quoteQualifier(language))
	}
	if options.Since != "" {
		parts = append(parts, "closed:>="+options.Since)
	}
	if options.Until != "" {
		parts = append(parts, "closed:<="+options.Until)
	}
	return strings.Join(parts, " ")
}

func quoteQualifier(value string) string {
	if strings.IndexFunc(value, unicode.IsSpace) < 0 && !strings.ContainsRune(value, '"') {
		return value
	}
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func (c *Client) ClosingPullRequests(ctx context.Context, issue Issue) ([]PullRequest, error) {
	owner, name, ok := splitRepo(issue.Repo)
	if !ok {
		return nil, fmt.Errorf("invalid repository %q", issue.Repo)
	}

	prs := make([]PullRequest, 0)
	cursor := ""
	for {
		args := []string{
			"api", "graphql",
			"-f", "query=" + closingPRQuery,
			"-f", "owner=" + owner,
			"-f", "name=" + name,
			"-F", "number=" + strconv.Itoa(issue.Number),
		}
		if cursor != "" {
			args = append(args, "-f", "cursor="+cursor)
		}

		out, err := c.runner.Run(ctx, args...)
		if err != nil {
			return nil, err
		}

		var response graphqlResponse
		if err := json.Unmarshal(out, &response); err != nil {
			return nil, fmt.Errorf("decode closing PR response: %w", err)
		}
		if response.Data.Repository == nil || response.Data.Repository.Issue == nil {
			return prs, nil
		}

		connection := response.Data.Repository.Issue.ClosedByPullRequestsReferences
		for _, node := range connection.Nodes {
			mergedAt := ""
			if node.MergedAt != nil {
				mergedAt = *node.MergedAt
			}
			prs = append(prs, PullRequest{
				Repo:     node.Repository.NameWithOwner,
				Number:   node.Number,
				Title:    node.Title,
				URL:      node.URL,
				State:    node.State,
				MergedAt: mergedAt,
			})
		}

		if !connection.PageInfo.HasNextPage {
			return prs, nil
		}
		if connection.PageInfo.EndCursor == "" || connection.PageInfo.EndCursor == cursor {
			return nil, fmt.Errorf("closing PR pagination returned an invalid cursor")
		}
		cursor = connection.PageInfo.EndCursor
	}
}

func repoFromAPIURL(raw string) (string, bool) {
	const marker = "/repos/"
	idx := strings.LastIndex(raw, marker)
	if idx < 0 {
		return "", false
	}
	repo := strings.Trim(strings.TrimSpace(raw[idx+len(marker):]), "/")
	_, _, ok := splitRepo(repo)
	return repo, ok
}

func splitRepo(repo string) (string, string, bool) {
	parts := strings.Split(strings.Trim(repo, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
