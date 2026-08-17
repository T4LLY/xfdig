---
name: xfdig
description: Find real merged GitHub fixes for bugs, errors, and symptoms by tracing similar closed issues to linked pull requests.
---

# xfdig

Use `xfdig` when a concrete bug, error, regression, hang, crash, or unexpected behavior may already have been fixed in another open-source project.

## Search

```bash
xfdig "<concrete symptom or error>"
```

The default output is compact JSON. Prefer specific error text, API names, failure modes, and subsystem terms over broad descriptions.

Useful options:

```bash
xfdig -n 8 "<query>"   # more candidates
xfdig -t "<query>"     # human-readable output
```

## Follow the evidence

Each result contains a merged PR URL and machine-readable `evidence`. Prefer candidates with a high `issue_rank`, a directly linked PR, and matching technical terms.

Inspect only promising candidates with existing GitHub tooling:

```bash
gh pr view <url>
gh pr diff <url>
```

Do not ask `xfdig` for diff contents; its job is fix discovery and handoff.

## Rules

- Treat retrieved fixes as evidence, not code to copy blindly.
- Compare the upstream bug conditions and version constraints with the local codebase before applying a fix.
- Prefer directly linked merged PRs over loosely similar discussions.
- If no convincing fix is found, continue normal codebase investigation rather than forcing a match.
- `xfdig` is read-only and must not modify repositories or create GitHub content.
