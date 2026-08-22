export function dailyJiraSnapshotFingerprint(snapshot: unknown): string {
  if (!snapshot || typeof snapshot !== 'object' || Array.isArray(snapshot)) {
    return JSON.stringify(snapshot);
  }

  const { generated_at: _generatedAt, ...businessSnapshot } = snapshot as Record<string, unknown>;
  return JSON.stringify(businessSnapshot);
}
