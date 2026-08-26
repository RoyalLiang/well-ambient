export type DeconstructorCompletedSnapshot<TResult = unknown> = {
  result: TResult;
  taskGroupId: string;
  activeTaskId: string;
  isMockResponse: boolean;
  linkedDemandId: string;
  linkedDemandTitle: string;
  demandTitle: string;
  inputText: string;
};

export type DeconstructorSessionContext = {
  demandId?: string;
  title?: string;
  description?: string;
  host?: string;
};

function normalizeIdentityPart(value: string | undefined) {
  return (value || '').normalize('NFKC').trim().replace(/\s+/g, ' ').toLowerCase();
}

function compactHash(value: string) {
  let hash = 2166136261;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return (hash >>> 0).toString(36);
}

export function buildDeconstructorSessionKey(context: DeconstructorSessionContext) {
  const demandId = normalizeIdentityPart(context.demandId);
  if (demandId) return `demand:${demandId}`;

  const draftIdentity = [
    normalizeIdentityPart(context.host) || 'standalone',
    normalizeIdentityPart(context.title),
    normalizeIdentityPart(context.description)
  ].join('\u241f');
  return `draft:${compactHash(draftIdentity)}`;
}

function cloneSnapshot<TResult>(snapshot: DeconstructorCompletedSnapshot<TResult>) {
  if (typeof structuredClone === 'function') {
    return structuredClone(snapshot) as DeconstructorCompletedSnapshot<TResult>;
  }
  return JSON.parse(JSON.stringify(snapshot)) as DeconstructorCompletedSnapshot<TResult>;
}

export function rememberDeconstructorSnapshot<TResult>(
  snapshots: ReadonlyMap<string, DeconstructorCompletedSnapshot<TResult>>,
  key: string,
  snapshot: DeconstructorCompletedSnapshot<TResult>,
  limit = 12
) {
  const next = new Map(snapshots);
  next.delete(key);
  next.set(key, cloneSnapshot(snapshot));

  const normalizedLimit = Math.max(1, Math.floor(limit));
  while (next.size > normalizedLimit) {
    const oldestKey = next.keys().next().value as string | undefined;
    if (!oldestKey) break;
    next.delete(oldestKey);
  }
  return next;
}

export function restoreDeconstructorSnapshot<TResult>(
  snapshots: ReadonlyMap<string, DeconstructorCompletedSnapshot<TResult>>,
  key: string
) {
  const snapshot = snapshots.get(key);
  return snapshot ? cloneSnapshot(snapshot) : null;
}
