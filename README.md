# xfdig

`xfdig` finds real merged fixes in GitHub history.

Give it a programming language plus a bug, error, or symptom. It searches similar closed GitHub issues, follows GitHub's issue-to-PR relationship, keeps merged pull requests, and emits compact evidence that another CLI or AI agent can inspect further.

```text
language + problem
  -> similar closed issues
  -> linked pull requests
  -> merged fixes
  -> compact JSON
```

`xfdig` does **not** download diffs, edit code, clone repositories, or call an LLM.

## Requirements

- Go 1.23+ to build
- GitHub CLI (`gh`) installed and authenticated

## Build

```bash
go build ./cmd/xfdig
```

Install from the module after publishing:

```bash
go install github.com/T4LLY/xfdig/cmd/xfdig@latest
```

## Usage

```bash
xfdig go --since 1y "socket reconnect race"
xfdig rust --since 2y --until 6m -n 8 "shutdown hangs"
xfdig csharp -t "CustomRenderTexture does not update"
xfdig any "HTTP proxy connection reset"
```

```text
Usage: xfdig <language|any> [options] <bug, error, or symptom>

Options:
  -s, --since <time>  search issues closed since this time
  -u, --until <time>  search issues closed until this time
  -n <N>              maximum number of fixes (1-100, default 20)
  -t                  human-readable output
```

`--since` and `--until` accept either an absolute date or a relative time:

```text
14d         14 days ago
6m          6 calendar months ago
2y          2 calendar years ago
2025-04-01  exact date
```

For example, if today is 2026-08-17:

```bash
xfdig go --since 2y --until 6m "deadlock"
```

searches issues closed from 2024-08-17 through 2026-02-17. Omitting both bounds searches all available history.

The language is a GitHub repository-language filter. Use `any` only when the problem is meaningfully language-independent.

Default output is compact JSON:

```json
{
  "status": "success",
  "q": "stderr pipe deadlock",
  "language": "go",
  "since": "2025-08-17",
  "search_type": "hybrid",
  "fixes": [
    {
      "repo": "owner/project",
      "pr": 456,
      "url": "https://github.com/owner/project/pull/456",
      "title": "Drain stderr concurrently",
      "merged_at": "2026-08-01T00:00:00Z",
      "issue": {
        "repo": "owner/project",
        "number": 123,
        "url": "https://github.com/owner/project/issues/123"
      },
      "evidence": {
        "issue_rank": 1,
        "issue_closed": true,
        "pr_linked": true,
        "pr_merged": true,
        "matched_terms": ["stderr", "pipe", "deadlock"]
      }
    }
  ]
}
```

`issue_rank` is the position returned by GitHub issue search; lower is better. `matched_terms` is a simple lexical overlap used only as additional evidence. It is not an LLM-generated explanation or a confidence score.

`status` is `success` when discovery completed normally, `warning` when search fell back to lexical mode, GitHub reported incomplete search results, or one or more linked-PR lookups failed but the remaining results are usable, and `failure` when discovery could not be completed reliably. Failure responses include an `error` field and exit with a non-zero status; detailed per-issue lookup failures remain in `warnings`.

## CLI chaining

`url` is deliberately a first-class field so downstream tools do not need to reconstruct repository and PR identifiers.

### jq + GitHub CLI

```bash
pr=$(xfdig go --since 2y "stderr pipe deadlock" | jq -r '.fixes[0].url')
gh pr view "$pr"
gh pr diff "$pr"
```

### PowerShell

```powershell
$pr = (xfdig go --since 2y "stderr pipe deadlock" | ConvertFrom-Json).fixes[0].url
gh pr view $pr
gh pr diff $pr
```

The JSON also keeps `merged_at`, `repo`, `issue`, and `pr` so tools such as `jq` can perform additional filtering or formatting without making `xfdig` itself grow more search options.

## Search behavior

`xfdig` requests GitHub hybrid issue search through `gh api`. The search query always requires closed issues, applies the requested repository language unless it is `any`, and applies `closed:` date bounds when `--since` or `--until` is present.

If the hybrid search request fails but ordinary issue search succeeds, `xfdig` falls back to lexical search, reports `"search_type":"lexical_fallback"`, and sets `status` to `warning`. GitHub search responses with `"incomplete_results":true` also produce `warning` so an empty result is not mistaken for a complete search.

For each closed issue candidate, `xfdig` asks GitHub GraphQL for `closedByPullRequestsReferences(includeClosedPrs: true)` and keeps only PRs with a merge timestamp. This avoids guessing issue/PR relationships from text.

## Agent skill

The repository includes `skills/xfdig/SKILL.md` for agent usage.

After publishing the repository, it can be installed with the Skills CLI convention:

```bash
npx skills add T4LLY/xfdig --skill xfdig -g -y
```

## Scope of v0.3

Included:

- programming-language filter or `any`
- natural-language/error query
- optional `--since` / `--until` closed-date bounds
- cross-repository closed issue search
- direct issue -> PR relationship lookup
- merged PR filtering
- compact JSON evidence
- human-readable output

Not included:

- diff retrieval
- code search
- repository cloning
- automatic fixes
- embeddings or local indexes
- LLM calls
