export interface ReviewRepoPolicy {
  project_id: string;
  repo_name: string;
  domain: string;
  scenario: string;
  knowledge_scope: string;
  sync_mrs: boolean;
  sync_commits: boolean;
  rules: string;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function stringValue(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : fallback;
}

/** Flatten the repository DTO's name and nested policy for the governance editor. */
export function normalizeReviewRepoPolicies(repos: unknown): ReviewRepoPolicy[] {
  if (!Array.isArray(repos)) return [];

  const normalized: ReviewRepoPolicy[] = [];
  for (const value of repos) {
    const repo = asRecord(value);
    if (!repo || typeof repo.project_id !== 'string' || !repo.project_id.trim()) continue;

    const policy = asRecord(repo.policy);
    normalized.push({
      project_id: repo.project_id,
      repo_name: stringValue(repo.name),
      // These are Service.Policy's defaults for an unconfigured repository.
      domain: stringValue(policy?.domain, 'general'),
      scenario: stringValue(policy?.scenario, 'general'),
      knowledge_scope: stringValue(policy?.knowledge_scope),
      sync_mrs: policy?.sync_mrs === true,
      sync_commits: policy?.sync_commits === true,
      rules: stringValue(policy?.rules),
    });
  }
  return normalized;
}
