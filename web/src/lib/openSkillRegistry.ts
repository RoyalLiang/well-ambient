export interface OpenSkillItem {
  enabled?: boolean;
  name: string;
  canonical_name: string;
  version: string;
  title: string;
  description: string;
  tools: string[];
  archive_url: string;
  manifest_url: string;
  doc_url: string;
  parent?: string;
  children?: OpenSkillItem[];
}

export interface SkillCall {
  request_id: string; subskill: string; tool: string; source_id: string; transport: string;
  outcome: string; error_code?: string; duration_ms: number; created_at: string; policy_version: number;
}
export interface SubskillObservation {
  id: string; calls: number; succeeded: number; failed: number; in_progress: number;
  success_rate: number | null; average_ms: number | null; error_codes: Record<string, number>; conditions: string[];
}
export interface KnowledgeState {
  enabled: boolean; interval_minutes: number; version: number; generation_id: number;
  last_checked_at: string | null; last_error?: string;
}
export interface OpenSkillsObservation {
  generated_at: string; window_start: string;
  phases: { Read: boolean; Prepare: boolean; Execute: boolean };
  policy_version: number;
  summary: { calls: number; succeeded: number; failed: number; in_progress: number; success_rate: number | null };
  subskills: SubskillObservation[]; recent_calls: SkillCall[]; recent_limit: number;
  knowledge: KnowledgeState | null;
  knowledge_generation: { id: number; item_count: number; created_at: string } | null;
  gaps: string[]; notice: string;
}

export function filterSkills(skills: OpenSkillItem[], query: string): OpenSkillItem[] {
  const terms = query.trim().toLocaleLowerCase().split(/\s+/).filter(Boolean);
  return skills.filter(skill => {
    const text = [skill.name, skill.title, skill.description, ...skill.tools].join(' ').toLocaleLowerCase();
    return terms.every(term => text.includes(term));
  });
}

export function prefixError(value: string): string {
  const prefix = value.trim() || 'well';
  return prefix.length <= 32 && /^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$/.test(prefix)
    ? '' : '请输入 1–32 位小写字母、数字或单连字符，以字母开头，不以连字符结尾。';
}

export function publicationDisabledSkills(current: string[], children: string[], selected: string, enabled: boolean): string[] {
  const disabled = new Set(current);
  if (disabled.has('well-ambient')) {
    for (const child of children) disabled.add(child);
    disabled.delete('well-ambient');
  }
  if (enabled) disabled.delete(selected); else disabled.add(selected);
  return [...disabled];
}
