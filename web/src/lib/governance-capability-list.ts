export interface GovernanceIncludedComponent {
  key: string;
  kind: string;
  name: string;
  description: string;
  tools?: string[];
}

export interface GovernanceCapabilityRecord {
  capability_key: string;
  kind: string;
  status: string;
  owner?: string;
  description?: string;
  is_top_level_skill?: boolean;
  parent_skill_key?: string;
  included_components?: GovernanceIncludedComponent[];
}

export interface GovernanceCapabilityFilters {
  search: string;
  kind: string;
  status: string;
}

export interface GovernanceCapabilityOrphan {
  key: string;
  parentKeys: string[];
}

/** Resolve ownership before filtering so component matches retain their skill. */
export function resolveGovernanceCapabilityHierarchy<T extends GovernanceCapabilityRecord>(items: readonly T[]) {
  const byKey = new Map<string, T>();
  for (const item of items) {
    if (item.capability_key && !byKey.has(item.capability_key)) byKey.set(item.capability_key, item);
  }

  const owners = new Map<string, Set<string>>();
  const declaredComponents = new Map<string, GovernanceIncludedComponent>();
  const addOwner = (key: string, parent: string) => {
    if (!key || !parent || key === parent) return;
    const parents = owners.get(key) ?? new Set<string>();
    parents.add(parent);
    owners.set(key, parents);
  };
  for (const item of byKey.values()) {
    addOwner(item.capability_key, item.parent_skill_key?.trim() ?? '');
    for (const component of item.included_components ?? []) {
      addOwner(component.key, item.capability_key);
      if (!declaredComponents.has(component.key)) declaredComponents.set(component.key, component);
    }
  }

  const topLevelKeys = new Set([...byKey.keys()].filter((key) => !owners.has(key)));
  const children = new Map<string, string[]>();
  for (const [key, parentKeys] of owners) {
    for (const parent of parentKeys) children.set(parent, [...(children.get(parent) ?? []), key]);
  }
  const reachableComponents = new Set<string>();
  const records: T[] = [];
  for (const key of topLevelKeys) {
    const item = byKey.get(key)!;
    const components = new Map<string, GovernanceIncludedComponent>();
    const pending = [...(children.get(key) ?? [])];
    const visited = new Set<string>([key]);
    while (pending.length) {
      const componentKey = pending.shift()!;
      if (visited.has(componentKey)) continue;
      visited.add(componentKey);
      reachableComponents.add(componentKey);
      pending.push(...(children.get(componentKey) ?? []));
      const child = byKey.get(componentKey);
      const declared = declaredComponents.get(componentKey);
      if (!child && !declared) continue;
      components.set(componentKey, {
        key: componentKey,
        kind: declared?.kind || child?.kind || '',
        name: declared?.name || componentKey,
        description: declared?.description || child?.description || '',
        tools: [...(declared?.tools ?? [])]
      });
    }
    records.push({ ...item, included_components: [...components.values()] });
  }

  const orphans: GovernanceCapabilityOrphan[] = [];
  for (const [key, parentKeys] of owners) {
    if (byKey.has(key) && !reachableComponents.has(key)) {
      orphans.push({ key, parentKeys: [...parentKeys] });
    }
  }
  return { records, orphans };
}

export function filterGovernanceCapabilities<T extends GovernanceCapabilityRecord>(
  records: readonly T[],
  filters: GovernanceCapabilityFilters
): T[] {
  const search = filters.search.trim().toLowerCase();
  return records.filter((record) => {
    if (filters.kind === 'skills_only') {
      if (record.kind !== 'skill' && !record.is_top_level_skill) return false;
    } else if (filters.kind !== 'all' && record.kind !== filters.kind) {
      return false;
    }
    if (filters.status !== 'all' && record.status !== filters.status) return false;
    if (!search) return true;
    const values = [record.capability_key, record.kind, record.owner, record.description];
    for (const component of record.included_components ?? []) {
      values.push(component.key, component.kind, component.name, component.description, ...(component.tools ?? []));
    }
    return values.filter(Boolean).join(' ').toLowerCase().includes(search);
  });
}

export function countIncludedGovernanceComponents(records: readonly GovernanceCapabilityRecord[]): number {
  return new Set(records.flatMap((record) => (record.included_components ?? []).map((component) => component.key))).size;
}
