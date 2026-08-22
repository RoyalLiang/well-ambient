export type SettingsSection =
  | 'gitlab'
  | 'feishu'
  | 'jira'
  | 'performance'
  | 'projects'
  | 'ai'
  | 'solution_prompts'
  | 'ai_context'
  | 'users'
  | 'matrix'
  | 'policies'
  | 'audit';

export interface SettingsSectionDefinition {
  id: SettingsSection;
  group: '集成设置' | 'AI 工作台' | '安全与授权';
  label: string;
  summary: string;
  domain: string;
  permissions: string[];
}

export const SETTINGS_SECTION_DEFINITIONS: SettingsSectionDefinition[] = [
  {
    id: 'gitlab',
    group: '集成设置',
    label: 'GitLab 仓库',
    summary: '维护仓库源、Webhook、令牌与同步项目。',
    domain: '仓库同步',
    permissions: ['config:read']
  },
  {
    id: 'feishu',
    group: '集成设置',
    label: '飞书消息同步',
    summary: '维护机器人、群聊与多维表格同步链路。',
    domain: '消息同步',
    permissions: ['config:read']
  },
  {
    id: 'jira',
    group: '集成设置',
    label: 'Jira 服务关联',
    summary: '维护任务源、同步项目、核心成员与自定义 JQL。',
    domain: '任务源集成',
    permissions: ['config:read']
  },
  {
    id: 'performance',
    group: '集成设置',
    label: '绩效计算',
    summary: '控制 core member 人员绩效的后台静默计算与滚动审计。',
    domain: '人员绩效治理',
    permissions: ['config:read']
  },
  {
    id: 'projects',
    group: '集成设置',
    label: '项目优先级',
    summary: '为已同步项目维护交付优先级与仓库映射。',
    domain: '项目映射',
    permissions: ['config:read']
  },
  {
    id: 'ai',
    group: 'AI 工作台',
    label: 'AI 引擎配置',
    summary: '维护模型供应商、端点、凭证与估算口径。',
    domain: '模型引擎',
    permissions: ['config:read']
  },
  {
    id: 'solution_prompts',
    group: 'AI 工作台',
    label: '方案润色规则',
    summary: '维护方案润色提示词版本、项目覆盖规则与 Jira 方案链接地址。',
    domain: '方案生成治理',
    permissions: ['solution_prompt:manage']
  },
  {
    id: 'ai_context',
    group: 'AI 工作台',
    label: '系统设计语料库',
    summary: '管理需求解构上下文、架构资料与交付语料。',
    domain: '设计语料',
    permissions: ['ai_context:read', 'config:read']
  },
  {
    id: 'users',
    group: '安全与授权',
    label: '成员角色管理',
    summary: '管理成员、用户组与仓库作用域分配。',
    domain: '身份管理',
    permissions: ['users:read']
  },
  {
    id: 'matrix',
    group: '安全与授权',
    label: '权限树配置',
    summary: '维护原子权限与用户组覆盖矩阵。',
    domain: '权限矩阵',
    permissions: ['users:read']
  },
  {
    id: 'policies',
    group: '安全与授权',
    label: '策略化授权',
    summary: '配置 allow/deny 策略并验证授权解释结果。',
    domain: '授权策略',
    permissions: ['users:read']
  },
  {
    id: 'audit',
    group: '安全与授权',
    label: '审计安全日志',
    summary: '追踪配置、登录、权限与授权策略变更。',
    domain: '安全审计',
    permissions: ['users:read']
  }
];

export const SETTINGS_SECTION_MAP = Object.fromEntries(
  SETTINGS_SECTION_DEFINITIONS.map((section) => [section.id, section])
) as Record<SettingsSection, SettingsSectionDefinition>;

export const SETTINGS_ROUTE_PERMISSIONS = ['config:read', 'solution_prompt:manage', 'ai_context:read', 'users:read'];

export const SETTINGS_FALLBACK_ORDER: SettingsSection[] = ['gitlab', 'solution_prompts', 'ai_context', 'users'];

export function isSettingsSection(value: unknown): value is SettingsSection {
  return typeof value === 'string' && value in SETTINGS_SECTION_MAP;
}

export function canAccessSettingsSection(section: SettingsSection, permissions: string[]): boolean {
  return SETTINGS_SECTION_MAP[section].permissions.some((permission) => permissions.includes(permission));
}

export function firstAccessibleSettingsSection(permissions: string[]): SettingsSection {
  return SETTINGS_FALLBACK_ORDER.find((section) => canAccessSettingsSection(section, permissions)) || 'gitlab';
}
