# Use the review tools

`review_search` returns bounded summaries and a stable cursor. Filter by published GitLab Project ID, kind, ref, SHA, state, and time. Repository names are display values, not authorization identities.

`review_get` returns a public report DTO. It can filter findings by severity, dimension, and file, and page them with `offset` and `limit`.

The tools don't return raw prompts, policy JSON, snapshots, credentials, internal stacks, or unfiltered run metadata.
