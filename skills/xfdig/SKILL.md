---
name: xfdig
description: Find real merged GitHub fixes for bugs, errors, and symptoms by tracing similar closed issues to linked pull requests.
---

# xfdig

Use `xfdig` when a concrete bug, error, regression, hang, crash, or unexpected behavior may already have been fixed in another open-source project.

## Search

```bash
xfdig <language|any> "<concrete symptom or error>"
```

Always provide the implementation language when it is known. Use `any` only for genuinely language-independent problems.

Prefer specific error text, API names, failure modes, and subsystem terms over broad descriptions.

Useful options:

```bash
xfdig go --since 1y "<query>"                  # fixes from the last year
xfdig rust --since 2y --until 6m "<query>"     # bounded historical window
xfdig csharp -n 8 "<query>"                    # more candidates
xfdig typescript -t "<query>"                  # human-readable output
```

`--since` / `--until` accept `Nd`, `Nm`, `Ny`, or `YYYY-MM-DD`. Relative values are resolved from the current local date. No date options means all available history.

## Follow the evidence

Each result contains a merged PR URL and machine-readable `evidence`. Prefer candidates with a low `issue_rank`, a directly linked merged PR, and matching technical terms.

Inspect only promising candidates with existing GitHub tooling:

```bash
gh pr view <url>
gh pr diff <url>
```

Use `jq` when additional filtering or formatting is needed. Do not ask `xfdig` for diff contents; its job is fix discovery and handoff.

## Rules

- Treat retrieved fixes as evidence, not code to copy blindly.
- Compare the upstream bug conditions and version constraints with the local codebase before applying a fix.
- Prefer directly linked merged PRs over loosely similar discussions.
- Narrow by language and, when recency matters, by `--since` / `--until` before increasing `-n`.
- If no convincing fix is found, continue normal codebase investigation rather than forcing a match.
- `xfdig` is read-only and must not modify repositories or create GitHub content.
