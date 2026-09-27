# Handle review tool errors

- `forbidden`: don't infer that an unpublished repository exists.
- `not_found`: report that no applicable published report was available.
- `invalid_query`: correct the run ID or filters.
- `rate_limited`: wait before retrying.

When stored coverage can't be decoded, report the missing reason instead of treating the run as complete.
