interface DailyJiraWindowBasis {
  generation: number;
  time_basis?: string;
}

export function canRetainDailyJiraWindow(previous: DailyJiraWindowBasis | undefined, next: DailyJiraWindowBasis): boolean {
  return !!previous?.time_basis
    && previous.generation === next.generation
    && previous.time_basis === next.time_basis;
}

export function dailyJiraSnapshotFingerprint(snapshot: unknown): string {
  if (!snapshot || typeof snapshot !== 'object' || Array.isArray(snapshot)) {
    return JSON.stringify(snapshot);
  }

  const { generated_at: _generatedAt, ...businessSnapshot } = snapshot as Record<string, unknown>;
  return JSON.stringify(businessSnapshot);
}
