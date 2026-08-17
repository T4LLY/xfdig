# xfdig

`xfdig` finds real merged fixes in GitHub history.

Give it a bug, error, or symptom. It searches similar closed GitHub issues, follows GitHub's issue-to-PR relationship, keeps merged pull requests, and emits compact evidence that another CLI or AI agent can inspect further.

```text
problem
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
xfdig "IPC::Open3 hangs when stderr is not drained"
xfdig -n 8 "socket reconnect race"
xfdig -t "CustomRenderTexture does not update"
```

Default output is compact JSON:

```json
{
  "q": "stderr pipe deadlock",
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

`issue_rank` is the position returned by GitHub issue search. `matched_terms` is a simple lexical overlap used only as additional evidence; it is not an LLM-generated explanation or a confidence score.

## CLI chaining

`url` is deliberately a first-class field so downstream tools do not need to reconstruct repository and PR identifiers.

### jq + GitHub CLI

```bash
pr=$(xfdig "stderr pipe deadlock" | jq -r '.fixes[0].url')
gh pr view "$pr"
gh pr diff "$pr"
```

### PowerShell

```powershell
$pr = (xfdig "stderr pipe deadlock" | ConvertFrom-Json).fixes[0].url
gh pr view $pr
gh pr diff $pr
```

## Search behavior

`xfdig` requests GitHub hybrid issue search through `gh api`. If the current GitHub API surface does not accept `search_type=hybrid`, it falls back to ordinary lexical issue search and reports `"search_type":"lexical_fallback"`.

For each closed issue candidate, `xfdig` asks GitHub GraphQL for `closedByPullRequestsReferences(includeClosedPrs: true)` and keeps only PRs with a merge timestamp. This avoids guessing issue/PR relationships from text.

## Agent skill

The repository includes `skills/xfdig/SKILL.md` for agent usage.

After publishing the repository, it can be installed with the Skills CLI convention:

```bash
npx skills add T4LLY/xfdig --skill xfdig -g -y
```

## Scope of v0.1

Included:

- natural-language/error query
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
