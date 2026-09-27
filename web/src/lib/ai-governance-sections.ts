export type AIGovernanceSection = 'skills' | 'prompts' | 'rules' | 'context' | 'open_capabilities';

export interface AIGovernanceSectionDefinition {
  id: AIGovernanceSection;
  label: string;
  subtitle: string;
  permissions: string[];
  roles?: string[];
}

export const AI_GOVERNANCE_SECTION_DEFINITIONS: AIGovernanceSectionDefinition[] = [
  { id: 'skills', label: '技能治理中心', subtitle: '微内核能力与生命周期', permissions: ['ai_context:read'] },
  { id: 'prompts', label: '提示词管理', subtitle: '多轮场景与版本管理', permissions: ['solution_prompt:manage'], roles: ['super_admin'] },
  { id: 'rules', label: '规则与标准', subtitle: '工程策略与合规审查', permissions: ['dashboard:read'] },
  { id: 'context', label: '上下文语料库', subtitle: '设计语料库与解构上下文', permissions: ['ai_context:read'] },
  { id: 'open_capabilities', label: '开放能力与 MCP', subtitle: '服务配置与客户端在线分发', permissions: ['ai_context:read'] }
];

export const AI_GOVERNANCE_SECTION_MAP = Object.fromEntries(
  AI_GOVERNANCE_SECTION_DEFINITIONS.map((section) => [section.id, section])
) as Record<AIGovernanceSection, AIGovernanceSectionDefinition>;

function hasPermission(permissions: string[], permission: string): boolean {
  return permissions.includes(permission) || permissions.includes('*');
}

export function isAIGovernanceSection(value: unknown): value is AIGovernanceSection {
  return typeof value === 'string' && value in AI_GOVERNANCE_SECTION_MAP;
}

export function canAccessAIGovernanceSection(
  section: AIGovernanceSection,
  permissions: string[],
  role: string
): boolean {
  const definition = AI_GOVERNANCE_SECTION_MAP[section];
  const roleAllowed = !definition.roles || definition.roles.includes(role);
  return roleAllowed && definition.permissions.every((permission) => hasPermission(permissions, permission));
}

export function firstAccessibleAIGovernanceSection(
  permissions: string[],
  role: string
): AIGovernanceSection | null {
  return AI_GOVERNANCE_SECTION_DEFINITIONS.find((section) =>
    canAccessAIGovernanceSection(section.id, permissions, role)
  )?.id || null;
}

export function resolveAIGovernanceSection(
  requested: unknown,
  permissions: string[],
  role: string
): AIGovernanceSection | null {
  if (isAIGovernanceSection(requested) && canAccessAIGovernanceSection(requested, permissions, role)) {
    return requested;
  }
  return firstAccessibleAIGovernanceSection(permissions, role);
}

export function canManageAIGovernanceSkills(permissions: string[], role: string): boolean {
  return role === 'super_admin' && hasPermission(permissions, 'solution_prompt:manage');
}

export function canWriteAIGovernanceRules(permissions: string[]): boolean {
  return hasPermission(permissions, 'config:write');
}

export function canReadSolutionPublicURL(permissions: string[], role: string): boolean {
  return canAccessAIGovernanceSection('prompts', permissions, role) && hasPermission(permissions, 'config:read');
}

export function canWriteSolutionPublicURL(permissions: string[], role: string): boolean {
  return canReadSolutionPublicURL(permissions, role) && hasPermission(permissions, 'config:write');
}

export function canManageOpenCapabilities(permissions: string[], role: string): boolean {
  return role === 'super_admin' || hasPermission(permissions, 'config:write');
}
