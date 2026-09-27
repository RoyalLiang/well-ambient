---
name: code-review-reader
description: Locate and read published code review reports for an exact repository and code version, including completion, coverage gaps, freshness, findings, and evidence.
---

# Code Review Reader

Use this skill to find and interpret existing review reports. It doesn't start, retry, cancel, or publish a review.

## Read the applicable report

1. Identify the canonical GitLab Project ID and expected head SHA when available.
2. Call `review_search` to locate candidate runs by repository, ref, SHA, status, and time.
3. Prefer a report whose `head_sha` matches the code being discussed.
4. Call `review_get` with `current_head_sha` to check freshness and load filtered findings.
5. Cite finding evidence references and state the run status, reviewed SHA, coverage gaps, and freshness.

## Don't overstate the result

- `completed` with no findings doesn't prove every path was reviewed.
- `partial` means coverage gaps remain.
- `failed` isn't a passing review.
- `outdated` applies to the recorded SHA, not the current head.
- `not_checked` means the current head wasn't verified.

Read [report applicability](references/report-applicability.md), [tool contracts](references/tool-contracts.md), and [error handling](references/error-handling.md) when needed.
