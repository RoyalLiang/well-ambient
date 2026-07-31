<script lang="ts">
  import '../styles/settings-config-workbench.css';
  import { onMount, onDestroy, tick } from 'svelte';
  import Button from './shared/Button.svelte';
  import Alert from './shared/Alert.svelte';
  import GitLabConfig from './config/GitLabConfig.svelte';
  import FeishuConfig from './config/FeishuConfig.svelte';
  import JiraConfig from './config/JiraConfig.svelte';
  import ProjectConfig from './config/ProjectConfig.svelte';
  import AIConfig from './config/AIConfig.svelte';
  import { lockBodyScroll, unlockBodyScroll } from '../lib/modalScrollLock';
  import { resetSettingsWorkspaceScroll } from '../lib/settings-ui';
  import {
    SETTINGS_SECTION_DEFINITIONS as SETTINGS_NAV_ITEMS,
    canAccessSettingsSection as hasSettingsSectionAccess,
    firstAccessibleSettingsSection as selectFirstAccessibleSettingsSection,
    isSettingsSection,
    type SettingsSection
  } from '../lib/settings-sections';
  import {
    ADMIN_TONE_CLASS,
    toneForStatus,
    type AdminInspectorRecord,
    type AdminTone
  } from '../lib/admin-console/contract';

  export let currentUserEmail = '';
  export let currentUserPermissions: string[] = [];
  export let activeSettingsSection = 'gitlab';
  export let onSectionChange: (section: string) => void = () => {};

  type ConnectionStatus = 'online' | 'offline' | 'warning';

  let serverStatus: ConnectionStatus = 'online';
  let activeHooks = 0;
  let statusIntervalId: any;

  async function fetchStatus() {
    try {
      const res = await fetch('/api/status');
      if (!res.ok) throw new Error('Offline');
      const data = await res.json();
      serverStatus = data.status === 'online' ? 'online' : 'warning';
      activeHooks = data.telemetry?.active_hooks ?? 0;
    } catch (e) {
      serverStatus = 'offline';
      activeHooks = 0;
    }
  }

  // Reactive connection status computations for navigation indicators
  let gitlabStatus: ConnectionStatus = 'warning';
  let feishuStatus: ConnectionStatus = 'warning';
  let jiraStatus: ConnectionStatus = 'warning';
  let aiStatus: ConnectionStatus = 'warning';

  $: gitlabStatus = globalConfig.gitlab?.enabled ? 'online' : 'warning';
  $: feishuStatus = globalConfig.feishu?.enabled ? 'online' : 'warning';
  $: jiraStatus = globalConfig.jira?.enabled ? 'online' : 'warning';
  $: aiStatus = globalConfig.ai?.enabled ? 'online' : 'warning';

  let activeSection: SettingsSection = 'gitlab';
  let settingsTitleEl: HTMLHeadingElement;

  function focusSettingsTitle() {
    void tick().then(() => settingsTitleEl?.focus({ preventScroll: true }));
  }

  $: if (
    isSettingsSection(activeSettingsSection) &&
    activeSettingsSection !== activeSection
  ) {
    activeSection = activeSettingsSection;
    saveError = '';
    saveSuccess = false;
    saveSuccessKey = null;
    resetSettingsWorkspaceScroll();
    focusSettingsTitle();
  }

  interface GlobalConfig {
    server: { host: string; port: number };
    gitlab: {
      enabled?: boolean;
      base_url: string;
      secret_token: string;
      repos: Array<{ name: string; path: string; project_id: string }>;
    };
    feishu: {
      enabled?: boolean;
      app_id: string;
      app_secret: string;
      bot: { enabled: boolean; chat_group: string };
      bitable: {
        enabled: boolean;
        app_token: string;
        table_id: string;
        status_column: string;
        task_id_column: string;
      };
    };
    ai: {
      enabled: boolean;
      provider: string;
      base_url: string;
      endpoint_type?: string;
      api_token: string;
      model: string;
      project_architecture?: string;
      delivery_workflow?: string;
      implemented_features?: string;
      estimation_guidelines?: string;
      default_work_hours_per_day?: number;
    };
    jira: {
      enabled: boolean;
      base_url: string;
      username: string;
      api_token: string;
      sync_projects?: string[];
      sync_users?: string[];
      sync_statuses?: string[];
      custom_jql?: string;
      version_sources?: Array<{
        project_key: string;
        project_name: string;
        version_url: string;
      }>;
    };
  }

  interface ConfigDiffEntry {
    path: string;
    before: any;
    after: any;
  }

  interface ConfigVersion {
    id: number;
    version: number;
    actor_id: string;
    actor_name: string;
    source: string;
    config: Record<string, any>;
    changed_sections: string[];
    diff: ConfigDiffEntry[];
    previous_version_id: number;
    rollback_from_version_id: number;
    created_at: string;
  }

  let globalConfig: GlobalConfig = {
    server: { host: '', port: 0 },
    gitlab: { base_url: '', secret_token: '', repos: [] },
    feishu: {
      app_id: '',
      app_secret: '',
      bot: { enabled: false, chat_group: '' },
      bitable: { enabled: false, app_token: '', table_id: '', status_column: '', task_id_column: '' }
    },
    ai: {
      enabled: false,
      provider: 'openai',
      base_url: '',
      endpoint_type: 'responses',
      api_token: '',
      model: '',
      project_architecture: '',
      delivery_workflow: '',
      implemented_features: '',
      estimation_guidelines: '',
      default_work_hours_per_day: 8
    },
    jira: { enabled: false, base_url: '', username: '', api_token: '', sync_projects: [], sync_users: [], sync_statuses: [], custom_jql: '', version_sources: [] }
  };

  let saving = false;
  let saveError = '';
  let saveSuccess = false;
  let saveSuccessKey: string | null = null;
  let configVersions: ConfigVersion[] = [];
  let selectedConfigVersionID: number | null = null;
  let configVersionError = '';
  let rollbackLoadingID: number | null = null;

  $: isIntegrationSection = ['gitlab', 'feishu', 'jira', 'projects', 'ai'].includes(activeSection);
  $: visibleConfigVersions = isIntegrationSection
    ? configVersions.filter(v => configVersionTouchesSection(v, activeSection))
    : configVersions;
  $: selectedConfigVersion = visibleConfigVersions.find(v => v.id === selectedConfigVersionID) || visibleConfigVersions[0] || null;

  function canAccessSection(section: SettingsSection) {
    return hasSettingsSectionAccess(section, currentUserPermissions);
  }

  function firstAccessibleSection(): SettingsSection {
    return selectFirstAccessibleSettingsSection(currentUserPermissions);
  }

  function switchSection(section: SettingsSection) {
    if (!canAccessSection(section)) return;
    activeSection = section;
    onSectionChange(section);
    saveError = '';
    saveSuccess = false;
    saveSuccessKey = null;
    resetSettingsWorkspaceScroll();
    focusSettingsTitle();
  }

  // RBAC lists
  interface UserMembership {
    group_name: string;
    group_display_name: string;
    scope: string;
    scope_id: string;
  }
  interface User {
    id: number;
    username: string;
    email: string;
    name: string;
    avatar: string;
    memberships: UserMembership[];
    created_at: string;
  }
  interface Group {
    id: number;
    name: string;
    displayName: string;
    description: string;
    permissions: string[];
  }
  interface AuditLog {
    id: number;
    actor_name: string;
    actor_username: string;
    action: string;
    target_id: string;
    target_type: string;
    detail: string;
    ip_address: string;
    created_at: string;
  }
  interface AuthorizationPolicy {
    id: number;
    effect: string;
    subject_type: string;
    subject_id: string;
    action: string;
    resource_type: string;
    resource_id: string;
    scope: string;
    scope_id: string;
    condition_json: string;
    priority: number;
    enabled: boolean;
    reason: string;
    created_at: string;
    updated_at: string;
  }
  interface AuthorizationAuditLog {
    id: number;
    subject_username: string;
    action: string;
    resource_type: string;
    resource_id: string;
    scope: string;
    scope_id: string;
    allowed: boolean;
    reason: string;
    missing_permission: string;
    risk_level: string;
    matched_policy_id?: number;
    created_at: string;
  }
  interface AuthorizationDecision {
    allowed: boolean;
    reason: string;
    missing_permission: string;
    scope: string;
    risk_level: string;
    matched_policy?: AuthorizationPolicy;
  }
  interface PermissionMeta {
    code: string;
    name: string;
    desc: string;
  }
  interface PermissionTreeBranch {
    key: string;
    label: string;
    summary: string;
    order: number;
    permissions: PermissionMeta[];
  }

  let users: User[] = [];
  $: userMap = new Map<string, User>(users.map(u => [u.username, u]));
  let groups: Group[] = [];
  let permissionCatalog: PermissionMeta[] = [];
  let auditLogs: AuditLog[] = [];
  let authorizationPolicies: AuthorizationPolicy[] = [];
  let authorizationAuditLogs: AuthorizationAuditLog[] = [];
  let authorizationPolicyError = '';
  let authorizationPolicySuccess = '';
  let authorizationPolicySaving = false;
  let authorizationDecision: AuthorizationDecision | null = null;
  let authorizationExplainError = '';
  type PolicyWorkspaceTab = 'list' | 'create' | 'explain' | 'audit';
  let policyWorkspaceTab: PolicyWorkspaceTab = 'list';

  const fallbackPermissionMeta: PermissionMeta[] = [
    { code: 'ai_context:preview', name: '预览 AI 上下文包', desc: '按需求范围预览 AI 解构将使用的上下文包内容' },
    { code: 'ai_context:read', name: '查看系统设计语料库', desc: '查看用于 AI 需求解构的架构、流程、功能边界与估算口径资料' },
    { code: 'ai_context:write', name: '管理系统设计语料库', desc: '新增、修改、停用系统设计资料、文档与上下文包配置' },
    { code: 'authorization_audit:read', name: '查看授权决策审计', desc: '查看拒绝或高风险授权决策的审计日志' },
    { code: 'config:read', name: '查看系统集成配置', desc: '查看 GitLab、飞书、Jira 以及 AI 大模型等集成密钥及连接状态' },
    { code: 'config:write', name: '修改及测试系统配置', desc: '修改并测试 GitLab、飞书、Jira 以及 AI 大模型等核心配置参数' },
    { code: 'dashboard:read', name: '查看协同看板页面', desc: '查看主界面协同看板、AI 需求解构日志与全部任务看板' },
    { code: 'decision:read', name: '查看决策大屏页面', desc: '查看红区卡点诊断盘与决策会议大屏' },
    { code: 'demands:read', name: '查看需求看板页面', desc: '查看需求看板泳道及其排期卡片' },
    { code: 'demands:write', name: '创建与指派需求', desc: '在需求看板中创建新需求并指派负责人' },
    { code: 'kpi:read', name: '查看团队 KPI 看板', desc: '查看团队成员的 KPI 绩效统计、完成任务及 Bug 指标' },
    { code: 'policies:read', name: '查看授权策略', desc: '查看策略化授权规则、作用范围与命中原因' },
    { code: 'policies:write', name: '管理授权策略', desc: '新增、修改、启停 allow/deny 授权策略' },
    { code: 'users:read', name: '查看成员及权限列表', desc: '查看所有注册用户、用户组以及审计日志' },
    { code: 'users:transfer_super_admin', name: '转让超级管理员角色', desc: '转让超级管理员权限给其他用户，自身降级为系统管理员' },
    { code: 'users:write', name: '管理成员组与权限分配', desc: '创建自定义组、修改权限集，并分配特定组与 Scope' }
  ];

  const permissionCategoryMeta: Record<string, { label: string; summary: string; order: number }> = {
    config: { label: '系统集成配置', summary: 'GitLab、飞书、Jira 与 AI 引擎的连接和测试权限。', order: 10 },
    dashboard: { label: '协同工作台', summary: '主协同看板和任务总览的访问边界。', order: 20 },
    demands: { label: '需求与交付', summary: '需求看板、创建指派和交付排期相关权限。', order: 30 },
    decision: { label: '决策视图', summary: '红区卡点诊断盘与会议大屏访问权限。', order: 40 },
    kpi: { label: '绩效分析', summary: '团队 KPI、报表预览和绩效指标权限。', order: 50 },
    ai_context: { label: '系统设计语料', summary: '系统设计资料、上下文包预览与需求解构参考资料权限。', order: 60 },
    users: { label: '成员与角色', summary: '成员、用户组、权限树和超级管理员交接权限。', order: 70 },
    policies: { label: '策略化授权', summary: 'allow/deny 策略的查看、创建、启停与解释。', order: 80 },
    authorization_audit: { label: '授权审计', summary: '拒绝、高风险授权决策和命中链路审计。', order: 90 }
  };
  const builtInGroupNames = new Set(['super_admin', 'admin', 'member']);

  const effectOptions = [
    { value: 'deny', label: '拒绝', desc: '优先阻断高风险动作' },
    { value: 'allow', label: '允许', desc: '临时放行明确动作' }
  ];

  const subjectTypeOptions = [
    { value: 'group', label: '用户组' },
    { value: 'user', label: '单个用户' },
    { value: 'any', label: '所有主体' }
  ];

  const scopeOptions = [
    { value: 'global', label: '全局' },
    { value: 'repo', label: '仓库' }
  ];

  const resourceTypeOptions = [
    { value: 'config', label: '配置' },
    { value: 'user', label: '成员' },
    { value: 'demand', label: '需求' },
    { value: 'dashboard', label: '看板' },
    { value: 'context', label: '上下文' },
    { value: 'policy', label: '策略' },
    { value: 'audit', label: '审计' }
  ];

  const policyQuickStarts = [
    {
      label: '冻结成员改配置',
      effect: 'deny',
      subject_type: 'group',
      subject_id: 'member',
      action: 'config:write',
      resource_type: 'config',
      resource_id: '',
      scope: 'global',
      scope_id: '',
      priority: 200,
      enabled: true,
      reason: '限制普通成员修改系统级集成配置。'
    },
    {
      label: '开放上下文预览',
      effect: 'allow',
      subject_type: 'group',
      subject_id: 'member',
      action: 'ai_context:preview',
      resource_type: 'context',
      resource_id: '',
      scope: 'global',
      scope_id: '',
      priority: 120,
      enabled: true,
      reason: '允许成员在需求评估前预览上下文包。'
    },
    {
      label: '限制策略写入',
      effect: 'deny',
      subject_type: 'group',
      subject_id: 'admin',
      action: 'policies:write',
      resource_type: 'policy',
      resource_id: '',
      scope: 'global',
      scope_id: '',
      priority: 240,
      enabled: true,
      reason: '策略写入仅由超级管理员审批。'
    }
  ];

  let permissionMeta: PermissionMeta[] = fallbackPermissionMeta;
  $: permissionMeta = permissionCatalog.length ? permissionCatalog : fallbackPermissionMeta;
  $: permissionTree = buildPermissionTree(permissionMeta);
  $: policyActionOptions = permissionMeta.map(perm => ({ code: perm.code, name: perm.name }));
  $: permissionCatalogSourceLabel = permissionCatalog.length ? '实时权限目录' : '本地兜底目录';
  $: activeSectionMeta = SETTINGS_NAV_ITEMS.find(item => item.id === activeSection) || SETTINGS_NAV_ITEMS[0];
  let lastUpdatedBySection = {} as Record<SettingsSection, string>;
  let settingsInspector: AdminInspectorRecord;
  $: lastUpdatedBySection = SETTINGS_NAV_ITEMS.reduce((result, item) => {
    result[item.id] = configVersions.find(version => (version.changed_sections || []).includes(item.id))?.created_at || '';
    return result;
  }, {} as Record<SettingsSection, string>);
  $: {
    globalConfig;
    configVersions;
    lastUpdatedBySection;
    users;
    permissionMeta;
    authorizationPolicies;
    auditLogs;
    authorizationAuditLogs;
    currentUserPermissions;
    gitlabStatus;
    feishuStatus;
    jiraStatus;
    aiStatus;
    serverStatus;
    settingsInspector = buildSettingsInspector();
  }
  $: settingsContextDetail = settingsInspector?.facts.find((fact) => fact.label === '链路状态')?.value || '-';

  let policyForm = {
    effect: 'deny',
    subject_type: 'group',
    subject_id: 'member',
    action: 'config:write',
    resource_type: 'config',
    resource_id: '',
    scope: 'global',
    scope_id: '',
    priority: 100,
    enabled: true,
    reason: ''
  };
  let explainForm = {
    username: '',
    action: 'config:read',
    resource_type: 'config',
    resource_id: '',
    scope: 'global',
    scope_id: ''
  };

  // Modals state for membership configuration
  let showAddMembershipModal = false;
  let membershipTargetUser = '';
  let membershipSelectedGroup = '';
  let showGroupDropdown = false;
  let membershipScope: 'global' | 'repo' = 'global';
  let membershipScopeID = '';
  let membershipError = '';
  let membershipSuccess = '';

  // Transfer admin state
  let showTransferModal = false;
  let transferTargetUsername = '';
  let transferConfirmName = '';
  let transferError = '';

  // Create custom group state
  let showCreateGroupModal = false;
  let newGroupName = '';
  let newGroupDisplayName = '';
  let newGroupDescription = '';
  let createGroupError = '';
  let groupMutationError = '';
  let groupMutationSuccess = '';
  let groupDeletingName = '';
  let showDeleteGroupModal = false;
  let deleteGroupTarget: Group | null = null;
  let showRemoveMembershipModal = false;
  let removeMembershipTarget: { username: string; membership: UserMembership } | null = null;
  let removingMembership = false;
  let removeMembershipError = '';
  let manualModalScrollLocked = false;
  $: settingsBreadcrumbs = ['管理台配置', activeSectionMeta.group, activeSectionMeta.label];

  $: {
    const shouldLock = showAddMembershipModal || showTransferModal || showCreateGroupModal || showDeleteGroupModal || showRemoveMembershipModal;
    if (shouldLock && !manualModalScrollLocked) {
      lockBodyScroll();
      manualModalScrollLocked = true;
    } else if (!shouldLock && manualModalScrollLocked) {
      unlockBodyScroll();
      manualModalScrollLocked = false;
    }
  }

  async function fetchConfig() {
    try {
      const res = await fetch('/api/config');
      if (res.ok) {
        globalConfig = await res.json();
      }
    } catch (e) {
      console.error('Failed to load settings', e);
    }
  }

  async function fetchConfigVersions(selectLatest = false) {
    if (!currentUserPermissions.includes('config:read')) return;
    try {
      const res = await fetch('/api/config/versions?limit=40');
      if (!res.ok) throw new Error(await res.text());
      configVersions = await res.json();
      if ((selectLatest || !selectedConfigVersionID || !configVersions.some(v => v.id === selectedConfigVersionID)) && configVersions.length > 0) {
        selectedConfigVersionID = configVersions[0].id;
      }
      configVersionError = '';
    } catch (e: any) {
      configVersionError = e.message || '配置版本记录加载失败';
    }
  }

  function sectionLastUpdated(section: string) {
    return isSettingsSection(section) ? lastUpdatedBySection[section] || '' : '';
  }

  function configVersionTouchesSection(version: ConfigVersion, section: string) {
    const sections = version.changed_sections || [];
    return sections.includes(section);
  }

  function formatDateTime(value: string) {
    if (!value) return '暂无版本记录';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString();
  }

  async function readResponsePayload(res: Response): Promise<any> {
    const text = await res.text();
    if (!text) return {};
    try {
      return JSON.parse(text);
    } catch {
      return { message: text };
    }
  }

  function formatDiffValue(value: any) {
    if (value === undefined || value === null || value === '') return '空';
    if (typeof value === 'string') return value;
    return JSON.stringify(value, null, 2);
  }

  async function rollbackConfigVersion(version: ConfigVersion) {
    if (!version || rollbackLoadingID) return;
    if (!confirm(`确定回滚到配置版本 v${version.version} 吗？当前配置会生成一条新的回滚版本记录。`)) return;

    rollbackLoadingID = version.id;
    configVersionError = '';
    try {
      const res = await fetch(`/api/config/versions/${version.id}/rollback`, {
        method: 'POST'
      });
      if (!res.ok) throw new Error(await res.text());
      await fetchConfig();
      await fetchConfigVersions(true);
      window.dispatchEvent(new CustomEvent('config-updated', { detail: globalConfig }));
    } catch (e: any) {
      configVersionError = e.message || '配置回滚失败';
    } finally {
      rollbackLoadingID = null;
    }
  }

  async function handleSaveConfig(event: CustomEvent<{ key: string; data: any; isToggle?: boolean }>) {
    const { key, data, isToggle } = event.detail;
    const newConfig = {
      ...globalConfig,
      [key]: data
    };

    saving = true;
    saveError = '';
    saveSuccess = false;
    saveSuccessKey = null;

    try {
      const res = await fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newConfig)
      });
      if (!res.ok) throw new Error('Failed to save config');
      const result = await res.json();
      if (result.success) {
        globalConfig = newConfig;
        if (!isToggle) {
          saveSuccess = true;
          saveSuccessKey = key;
        }
        await fetchConfigVersions(true);
        window.dispatchEvent(new CustomEvent('config-updated', { detail: newConfig }));
      } else {
        saveError = '保存配置失败: ' + result.message;
      }
    } catch (e: any) {
      saveError = '网络请求失败: ' + e.message;
    } finally {
      saving = false;
    }
  }

  function handleConfigClose() {
    saveError = '';
    saveSuccess = false;
    saveSuccessKey = null;
  }

  function statusForSection(section: SettingsSection): ConnectionStatus {
    if (section === 'gitlab') return gitlabStatus;
    if (section === 'feishu') return feishuStatus;
    if (section === 'jira') return jiraStatus;
    if (section === 'ai') return aiStatus;
    if (section === 'ai_context') return currentUserPermissions.includes('ai_context:read') ? 'online' : 'warning';
    if (['users', 'matrix', 'policies', 'audit'].includes(section)) return currentUserPermissions.includes('users:read') ? 'online' : 'warning';
    return serverStatus;
  }

  function statusLabel(status: 'online' | 'offline' | 'warning') {
    if (status === 'online') return '在线';
    if (status === 'offline') return '离线';
    return '待配置';
  }

  function statusTone(status: 'online' | 'offline' | 'warning'): AdminTone {
    return toneForStatus(status === 'online' ? 'completed' : status === 'offline' ? 'failed' : 'pending');
  }

  function integrationDetail(section: SettingsSection) {
    if (section === 'gitlab') return `${globalConfig.gitlab?.repos?.length || 0} 个仓库`;
    if (section === 'feishu') return globalConfig.feishu?.bot?.enabled ? '机器人已启用' : '机器人待配置';
    if (section === 'jira') return `${globalConfig.jira?.sync_projects?.length || 0} 个项目`;
    if (section === 'projects') return `${globalConfig.jira?.sync_projects?.length || 0} 个映射`;
    if (section === 'ai') return globalConfig.ai?.model || '模型待配置';
    if (section === 'ai_context') return globalConfig.ai?.project_architecture ? '语料已就绪' : '语料待配置';
    if (section === 'users') return `${users.length} 位成员`;
    if (section === 'matrix') return `${permissionMeta.length} 个权限`;
    if (section === 'policies') return `${authorizationPolicies.length} 条策略`;
    if (section === 'audit') return `${auditLogs.length + authorizationAuditLogs.length} 条日志`;
    return '-';
  }

  function sectionLastUpdatedLabel(section: SettingsSection) {
    const updated = sectionLastUpdated(section);
    return updated ? formatDateTime(updated) : '暂无版本';
  }

  function requiredPermissionLabel(section: SettingsSection) {
    if (['gitlab', 'feishu', 'jira', 'projects', 'ai'].includes(section)) return '配置只读';
    if (section === 'ai_context') return '语料只读';
    return '成员只读';
  }

  function sectionDisplayName(section: string) {
    return SETTINGS_NAV_ITEMS.find(item => item.id === section)?.label || section;
  }

  function formatChangedSections(sections: string[] = []) {
    return sections.map(sectionDisplayName).join(' / ') || '无差异';
  }

  function sectionApiLinks(section: SettingsSection) {
    if (['gitlab', 'feishu', 'jira', 'projects', 'ai', 'ai_context'].includes(section)) {
      return ['/api/config', '/api/config/versions'];
    }
    if (section === 'users') return ['/api/users', '/api/groups'];
    if (section === 'matrix') return ['/api/groups', '/api/permissions'];
    if (section === 'policies') return ['/api/authz/policies', '/api/authz/explain', '/api/authz/audit-logs'];
    if (section === 'audit') return ['/api/audit-logs', '/api/authz/audit-logs'];
    return [];
  }

  function buildSettingsInspector(): AdminInspectorRecord {
    const status = statusForSection(activeSectionMeta.id);
    return {
      id: activeSectionMeta.id,
      title: activeSectionMeta.label,
      status: statusLabel(status),
      tone: statusTone(status),
      facts: [
        { label: '分类', value: activeSectionMeta.group },
        { label: '权限', value: requiredPermissionLabel(activeSectionMeta.id) },
        { label: '最近更新', value: sectionLastUpdatedLabel(activeSectionMeta.id) },
        { label: '链路状态', value: integrationDetail(activeSectionMeta.id) }
      ],
      sections: [
        {
          title: '真实数据链路',
          items: sectionApiLinks(activeSectionMeta.id)
        },
        {
          title: '当前说明',
          body: activeSectionMeta.summary
        }
      ]
    };
  }

  // RBAC Requests
  async function fetchUsers() {
    if (!currentUserPermissions.includes('users:read')) return;
    try {
      const res = await fetch('/api/users');
      if (res.ok) users = await res.json();
    } catch (e) {
      console.error(e);
    }
  }

  async function fetchGroups() {
    if (!currentUserPermissions.includes('users:read')) return;
    try {
      const res = await fetch('/api/groups');
      if (res.ok) {
        const data = await res.json();
        groups = Array.isArray(data) ? data.map(normalizeGroup) : [];
      }
    } catch (e) {
      console.error(e);
    }
  }

  async function fetchPermissions() {
    if (!currentUserPermissions.includes('users:read')) return;
    try {
      const res = await fetch('/api/permissions');
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      permissionCatalog = Array.isArray(data) ? data.map(normalizePermission).filter(perm => perm.code) : [];
    } catch (e) {
      console.error(e);
      permissionCatalog = [];
    }
  }

  async function fetchAuditLogs() {
    if (!currentUserPermissions.includes('users:read')) return;
    try {
      const res = await fetch('/api/audit-logs');
      if (res.ok) auditLogs = await res.json();
    } catch (e) {
      console.error(e);
    }
  }

  async function fetchAuthorizationPolicies() {
    if (!currentUserPermissions.includes('policies:read') && !currentUserPermissions.includes('users:read')) return;
    try {
      const res = await fetch('/api/authz/policies');
      const data = await res.json();
      if (!res.ok) throw new Error(data.message || '策略列表加载失败');
      authorizationPolicies = Array.isArray(data) ? data : (data.policies || []);
      authorizationPolicyError = '';
    } catch (e: any) {
      authorizationPolicyError = e.message || '策略列表加载失败';
    }
  }

  async function fetchAuthorizationAuditLogs() {
    if (!currentUserPermissions.includes('authorization_audit:read') && !currentUserPermissions.includes('users:read')) return;
    try {
      const res = await fetch('/api/authz/audit-logs?limit=40');
      const data = await res.json();
      if (res.ok) authorizationAuditLogs = Array.isArray(data) ? data : (data.logs || []);
    } catch (e) {
      console.error(e);
    }
  }

  async function saveAuthorizationPolicy() {
    authorizationPolicyError = '';
    authorizationPolicySuccess = '';
    if (!policyForm.action.trim()) {
      authorizationPolicyError = 'Action 不能为空';
      return;
    }
    if (policyForm.subject_type !== 'any' && !policyForm.subject_id.trim()) {
      authorizationPolicyError = '非 any 主体需要填写 Subject ID';
      return;
    }

    authorizationPolicySaving = true;
    try {
      const res = await fetch('/api/authz/policies', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(policyForm)
      });
      const data = await res.json();
      if (!res.ok || data.success === false) throw new Error(data.message || '策略保存失败');
      authorizationPolicySuccess = '授权策略已保存';
      await fetchAuthorizationPolicies();
    } catch (e: any) {
      authorizationPolicyError = e.message || '策略保存失败';
    } finally {
      authorizationPolicySaving = false;
    }
  }

  async function explainAuthorization() {
    authorizationExplainError = '';
    authorizationDecision = null;
    if (!explainForm.username.trim() || !explainForm.action.trim()) {
      authorizationExplainError = 'Username 与 Action 不能为空';
      return;
    }

    try {
      const res = await fetch('/api/authz/explain', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(explainForm)
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.message || '授权解释失败');
      authorizationDecision = data;
      await fetchAuthorizationAuditLogs();
    } catch (e: any) {
      authorizationExplainError = e.message || '授权解释失败';
    }
  }

  function policyScopeLabel(policy: Pick<AuthorizationPolicy, 'scope' | 'scope_id'>) {
    if (!policy.scope || policy.scope === 'global') return 'global';
    return `${policy.scope}:${policy.scope_id || '*'}`;
  }

  function normalizeGroup(raw: any): Group {
    return {
      id: raw?.id,
      name: raw?.name || '',
      displayName: raw?.displayName || raw?.display_name || raw?.name || '未命名用户组',
      description: raw?.description || '',
      permissions: Array.isArray(raw?.permissions) ? raw.permissions : []
    };
  }

  function normalizePermission(raw: any): PermissionMeta {
    return {
      code: raw?.code || '',
      name: raw?.name || raw?.code || '未命名权限',
      desc: raw?.desc || raw?.description || ''
    };
  }

  function permissionNamespace(code: string) {
    const index = code.indexOf(':');
    return index > 0 ? code.slice(0, index) : 'other';
  }

  function buildPermissionTree(permissions: PermissionMeta[]): PermissionTreeBranch[] {
    const branches = new Map<string, PermissionTreeBranch>();
    for (const perm of permissions) {
      const key = permissionNamespace(perm.code);
      const meta = permissionCategoryMeta[key] || {
        label: key === 'other' ? '未分类权限' : `${key} 权限`,
        summary: '系统自动归入新命名空间，后续可补充中文名称。',
        order: 900
      };
      if (!branches.has(key)) {
        branches.set(key, {
          key,
          label: meta.label,
          summary: meta.summary,
          order: meta.order,
          permissions: []
        });
      }
      branches.get(key)?.permissions.push(perm);
    }

    return Array.from(branches.values())
      .map(branch => ({
        ...branch,
        permissions: [...branch.permissions].sort((a, b) => a.code.localeCompare(b.code))
      }))
      .sort((a, b) => a.order - b.order || a.label.localeCompare(b.label));
  }

  function groupHasPermission(group: Group, code: string) {
    return group.name === 'super_admin' || group.permissions.includes(code);
  }

  function groupPermissionCount(group: Group) {
    if (group.name === 'super_admin') return permissionMeta.length;
    return group.permissions.filter(code => permissionMeta.some(perm => perm.code === code)).length;
  }

  function groupPermissionRatio(group: Group) {
    if (!permissionMeta.length) return '0%';
    return `${Math.round((groupPermissionCount(group) / permissionMeta.length) * 100)}%`;
  }

  function isBuiltInGroup(group: Group) {
    return builtInGroupNames.has(group.name);
  }

  function groupMemberCount(group: Group) {
    return users.reduce((count, user) => {
      return count + membershipsForUser(user).filter(membership => membership.group_name === group.name).length;
    }, 0);
  }

  function membershipsForUser(user: User) {
    return Array.isArray(user.memberships) ? user.memberships : [];
  }

  function applyPolicyQuickStart(template: typeof policyQuickStarts[number]) {
    policyForm = {
      effect: template.effect,
      subject_type: template.subject_type,
      subject_id: template.subject_id,
      action: template.action,
      resource_type: template.resource_type,
      resource_id: template.resource_id,
      scope: template.scope,
      scope_id: template.scope_id,
      priority: template.priority,
      enabled: template.enabled,
      reason: template.reason
    };
  }

  function setPolicyWorkspaceTab(tab: PolicyWorkspaceTab) {
    policyWorkspaceTab = tab;
    resetSettingsWorkspaceScroll();
  }

  function setPolicyField<K extends keyof typeof policyForm>(field: K, value: (typeof policyForm)[K]) {
    policyForm = { ...policyForm, [field]: value };
    if (field === 'subject_type' && value === 'any') {
      policyForm = { ...policyForm, subject_id: '' };
    }
    if (field === 'scope' && value === 'global') {
      policyForm = { ...policyForm, scope_id: '' };
    }
  }

  function setExplainField<K extends keyof typeof explainForm>(field: K, value: (typeof explainForm)[K]) {
    explainForm = { ...explainForm, [field]: value };
    if (field === 'scope' && value === 'global') {
      explainForm = { ...explainForm, scope_id: '' };
    }
  }

  function stepPolicyPriority(delta: number) {
    const next = Number(policyForm.priority) + delta;
    policyForm = { ...policyForm, priority: Math.max(0, Math.min(999, next)) };
  }

  function resourceTypeForAction(action: string) {
    const key = permissionNamespace(action);
    const map: Record<string, string> = {
      config: 'config',
      users: 'user',
      demands: 'demand',
      dashboard: 'dashboard',
      decision: 'dashboard',
      kpi: 'dashboard',
      ai_context: 'context',
      policies: 'policy',
      authorization_audit: 'audit'
    };
    return map[key] || key || 'global';
  }

  function selectPolicyAction(action: string) {
    policyForm = {
      ...policyForm,
      action,
      resource_type: resourceTypeForAction(action)
    };
  }

  function selectExplainAction(action: string) {
    explainForm = {
      ...explainForm,
      action,
      resource_type: resourceTypeForAction(action)
    };
  }

  function openAddMembership(username: string) {
    membershipTargetUser = username;
    membershipSelectedGroup = groups[0]?.name || '';
    membershipScope = 'global';
    membershipScopeID = '';
    membershipError = '';
    membershipSuccess = '';
    showGroupDropdown = false;
    showAddMembershipModal = true;
  }

  async function saveMembership() {
    membershipError = '';
    membershipSuccess = '';
    if (membershipScope === 'repo' && !membershipScopeID) {
      membershipError = '请填写作用域的具体仓库名/项目ID';
      return;
    }

    try {
      const res = await fetch('/api/users/groups', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: membershipTargetUser,
          group_name: membershipSelectedGroup,
          scope: membershipScope,
          scope_id: membershipScopeID,
          action: 'add'
        })
      });
      const data = await res.json();
      if (res.ok) {
        membershipSuccess = '成员分配添加成功！';
        fetchUsers();
        setTimeout(() => {
          showAddMembershipModal = false;
        }, 1000);
      } else {
        membershipError = data.message || '操作失败';
      }
    } catch (e: any) {
      membershipError = '请求失败: ' + e.message;
    }
  }

  function membershipScopeLabel(membership: UserMembership) {
    return membership.scope === 'global' ? '全局' : `仓库: ${membership.scope_id}`;
  }

  function requestRemoveMembership(username: string, membership: UserMembership) {
    removeMembershipTarget = { username, membership };
    removeMembershipError = '';
    showRemoveMembershipModal = true;
  }

  function closeRemoveMembershipModal() {
    if (removingMembership) return;
    showRemoveMembershipModal = false;
    removeMembershipTarget = null;
    removeMembershipError = '';
  }

  async function confirmRemoveMembership() {
    if (!removeMembershipTarget) return;
    const { username, membership } = removeMembershipTarget;
    removeMembershipError = '';
    removingMembership = true;
    try {
      const res = await fetch('/api/users/groups', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: username,
          group_name: membership.group_name,
          scope: membership.scope,
          scope_id: membership.scope_id,
          action: 'remove'
        })
      });
      const data = await readResponsePayload(res);
      if (res.ok) {
        showRemoveMembershipModal = false;
        removeMembershipTarget = null;
        await fetchUsers();
        await fetchAuditLogs();
      } else {
        removeMembershipError = data.message || data.error || '移除失败';
      }
    } catch (e: any) {
      removeMembershipError = '请求失败: ' + e.message;
    } finally {
      removingMembership = false;
    }
  }

  function openTransferAdmin(username: string) {
    transferTargetUsername = username;
    transferConfirmName = '';
    transferError = '';
    showTransferModal = true;
  }

  async function executeTransfer() {
    transferError = '';
    const targetUser = userMap.get(transferTargetUsername);
    if (!targetUser) return;
    if (transferConfirmName.trim() !== targetUser.name) {
      transferError = `输入的姓名不匹配，请输入「${targetUser.name}」以确认安全转让`;
      return;
    }

    try {
      const res = await fetch('/api/users/transfer-admin', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: transferTargetUsername })
      });
      const data = await res.json();
      if (res.ok) {
        alert('超级管理员权限已成功转让！您将自动降级为系统管理员，系统将重新加载。');
        window.location.reload();
      } else {
        transferError = data.message || '转让失败';
      }
    } catch (e: any) {
      transferError = '网络请求失败: ' + e.message;
    }
  }

  async function togglePermissionInMatrix(group: Group, code: string) {
    if (!currentUserPermissions.includes('users:write')) return;
    const hasPerm = group.permissions.includes(code);
    let newPerms = [];
    if (hasPerm) {
      newPerms = group.permissions.filter(p => p !== code);
    } else {
      newPerms = [...group.permissions, code];
    }

    try {
      const res = await fetch('/api/groups/permissions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          group_name: group.name,
          permissions: newPerms
        })
      });
      if (res.ok) {
        fetchGroups();
      } else {
        const data = await res.json();
        alert('修改失败: ' + data.message);
      }
    } catch (e: any) {
      alert('网络请求失败: ' + e.message);
    }
  }

  function requestDeleteCustomGroup(group: Group) {
    if (!currentUserPermissions.includes('users:write') || isBuiltInGroup(group)) return;
    groupMutationError = '';
    groupMutationSuccess = '';

    const members = groupMemberCount(group);
    if (members > 0) {
      groupMutationError = `「${group.displayName}」仍有 ${members} 个成员，请先移除成员身份后再删除`;
      return;
    }

    deleteGroupTarget = group;
    showDeleteGroupModal = true;
  }

  async function deleteCustomGroup() {
    if (!deleteGroupTarget || isBuiltInGroup(deleteGroupTarget)) return;

    const group = deleteGroupTarget;
    groupMutationError = '';
    groupMutationSuccess = '';
    groupDeletingName = group.name;
    try {
      const res = await fetch(`/api/groups/${encodeURIComponent(group.name)}`, {
        method: 'DELETE'
      });
      const data = await readResponsePayload(res);
      if (!res.ok) {
        throw new Error(data.message || data.error || '删除用户组失败');
      }

      groupMutationSuccess = `已删除自定义组「${group.displayName}」`;
      showDeleteGroupModal = false;
      deleteGroupTarget = null;
      await fetchGroups();
      await fetchUsers();
      await fetchAuditLogs();
    } catch (e: any) {
      groupMutationError = e.message || '删除用户组失败';
    } finally {
      groupDeletingName = '';
    }
  }

  async function createCustomGroup() {
    createGroupError = '';
    groupMutationError = '';
    groupMutationSuccess = '';
    if (!newGroupName || !newGroupDisplayName) {
      createGroupError = '组代码与显示名称不能为空';
      return;
    }
    try {
      const res = await fetch('/api/groups', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: newGroupName.trim(),
          display_name: newGroupDisplayName.trim(),
          description: newGroupDescription.trim()
        })
      });
      const data = await res.json();
      if (res.ok) {
        const createdDisplayName = newGroupDisplayName.trim();
        showCreateGroupModal = false;
        newGroupName = '';
        newGroupDisplayName = '';
        newGroupDescription = '';
        groupMutationSuccess = `已创建自定义组「${createdDisplayName}」`;
        fetchGroups();
      } else {
        createGroupError = data.message || '创建用户组失败';
      }
    } catch (e: any) {
      createGroupError = '请求错误: ' + e.message;
    }
  }

  onMount(() => {
    if (isSettingsSection(activeSettingsSection)) {
      activeSection = activeSettingsSection;
    } else if (!canAccessSection(activeSection)) {
      activeSection = firstAccessibleSection();
    }
    onSectionChange(activeSection);
    fetchConfig();
    fetchConfigVersions();
    fetchStatus();
    fetchUsers();
    fetchGroups();
    fetchPermissions();
    fetchAuditLogs();
    fetchAuthorizationPolicies();
    fetchAuthorizationAuditLogs();

    statusIntervalId = setInterval(fetchStatus, 5000);

    const handleFocus = (e: any) => {
      if (isSettingsSection(e.detail)) {
        switchSection(e.detail);
      }
    };
    const handleConfigUpdated = async (e: any) => {
      if (e.detail && (e.detail.gitlab || e.detail.feishu || e.detail.jira || e.detail.ai)) {
        globalConfig = e.detail;
      } else {
        await fetchConfig();
      }
      await fetchConfigVersions(true);
    };

    window.addEventListener('focus-settings-section', handleFocus);
    window.addEventListener('config-updated', handleConfigUpdated);
    return () => {
      window.removeEventListener('focus-settings-section', handleFocus);
      window.removeEventListener('config-updated', handleConfigUpdated);
      if (statusIntervalId) {
        clearInterval(statusIntervalId);
      }
    };
  });

  onDestroy(() => {
    if (manualModalScrollLocked) {
      unlockBodyScroll();
      manualModalScrollLocked = false;
    }
  });
</script>

<div id="settings-unified-root" class="settings-container settings-unified phase41-settings phase46-settings phase49-settings">
  <main class="settings-main">
    <section class="settings-content-shell" aria-labelledby="settings-content-title">
      <header class="settings-context-panel">
        <div class="settings-context-topline">
          <nav class="settings-breadcrumb-bar" aria-label="面包屑导航">
            <ol>
              {#each settingsBreadcrumbs as crumb, index}
                <li class:current={index === settingsBreadcrumbs.length - 1}>
                  <span aria-current={index === settingsBreadcrumbs.length - 1 ? 'page' : undefined}>{crumb}</span>
                </li>
              {/each}
            </ol>
          </nav>
          <div class="breadcrumb-meta" aria-label="页面状态">
            <span class="wa-admin-pill {ADMIN_TONE_CLASS[settingsInspector.tone || 'neutral']}">{settingsInspector.status}</span>
            <span class="font-mono">{settingsContextDetail}</span>
          </div>
        </div>

        <div class="settings-content-header">
          <div class="settings-title-copy">
            <span class="settings-kicker">{activeSectionMeta.domain}</span>
            <h1 id="settings-content-title" bind:this={settingsTitleEl} tabindex="-1">{activeSectionMeta.label}</h1>
            <p>{activeSectionMeta.summary}</p>
          </div>
          <dl class="settings-content-facts">
            {#each settingsInspector.facts.slice(1) as fact}
              <div>
                <dt>{fact.label}</dt>
                <dd>{fact.value}</dd>
              </div>
            {/each}
          </dl>
        </div>
      </header>

      <div class="settings-module-panel">
        <div class="settings-workbench-grid" class:without-audit={!isIntegrationSection}>
          <div class="settings-primary-pane" class:integration-surface={isIntegrationSection || activeSection === 'ai_context'}>
    {#if activeSection === 'gitlab'}
            <GitLabConfig config={globalConfig.gitlab} lastUpdated={lastUpdatedBySection.gitlab} on:save={handleSaveConfig} on:close={handleConfigClose} {saveError} {saving} saveSuccess={saveSuccess && saveSuccessKey === 'gitlab'} />
    {:else if activeSection === 'feishu'}
            <FeishuConfig config={globalConfig.feishu} lastUpdated={lastUpdatedBySection.feishu} on:save={handleSaveConfig} on:close={handleConfigClose} {saveError} {saving} saveSuccess={saveSuccess && saveSuccessKey === 'feishu'} />
    {:else if activeSection === 'jira'}
            <JiraConfig config={globalConfig.jira} lastUpdated={lastUpdatedBySection.jira} on:save={handleSaveConfig} on:close={handleConfigClose} {saveError} {saving} saveSuccess={saveSuccess && saveSuccessKey === 'jira'} />
    {:else if activeSection === 'projects'}
            <ProjectConfig lastUpdated={lastUpdatedBySection.projects} syncProjects={globalConfig.jira?.sync_projects || []} />
    {:else if activeSection === 'ai'}
            <AIConfig view="engine" config={globalConfig.ai} lastUpdated={lastUpdatedBySection.ai} on:save={handleSaveConfig} on:close={handleConfigClose} {saveError} {saving} saveSuccess={saveSuccess && saveSuccessKey === 'ai'} {currentUserPermissions} />
    {:else if activeSection === 'ai_context'}
            <AIConfig view="context" config={globalConfig.ai} lastUpdated={lastUpdatedBySection.ai} on:save={handleSaveConfig} on:close={handleConfigClose} {saveError} {saving} saveSuccess={false} {currentUserPermissions} />
    {:else if activeSection === 'users'}
            <div class="section-card">
        <div class="card-header">
          <h2>成员角色与项目隔离 (Scope) 管理</h2>
          <p>查看并管理注册成员所属的权限组，在此可配置细粒度的 GitLab 仓库作用域隔离（Scope）。</p>
        </div>

        <!-- svelte-ignore a11y_no_noninteractive_tabindex (bounded member directory needs keyboard scrolling) -->
        <div class="table-responsive settings-data-scroll users-data-scroll" role="region" aria-label="成员角色目录，可横向或纵向滚动" tabindex="0">
          <table class="rbac-table">
            <thead>
              <tr>
                <th>成员信息</th>
                <th>邮箱/账号</th>
                <th>拥有角色及作用域 (Scope)</th>
                <th style="text-align: right;">管理操作</th>
              </tr>
            </thead>
            <tbody>
              {#each users as user}
                <tr>
                  <td>
                    <div class="user-profile-cell">
                      <div class="user-avatar">
                        {#if user.avatar}
                          <img src={user.avatar} alt="avatar" />
                        {:else}
                          {user.name ? user.name.charAt(0) : 'U'}
                        {/if}
                      </div>
                      <span class="user-name">{user.name || '未定义'}</span>
                    </div>
                  </td>
                  <td class="font-mono text-sm">{user.username}</td>
                  <td>
                    <div class="tags-container">
                      {#each membershipsForUser(user) as membership}
                        <div class="membership-badge badge-{membership.group_name}">
                          <span class="badge-role">{membership.group_display_name}</span>
                          <span class="badge-scope">
                            {membership.scope === 'global' ? '全局' : membership.scope_id}
                          </span>
                          {#if currentUserPermissions.includes('users:write') && membership.group_name !== 'super_admin'}
                            <button class="remove-membership-btn" on:click={() => requestRemoveMembership(user.username, membership)} title="移除组身份">&times;</button>
                          {/if}
                        </div>
                      {:else}
                        <span class="text-muted text-xs font-mono">暂未分配用户组（无管理权限）</span>
                      {/each}
                    </div>
                  </td>
                  <td style="text-align: right;">
                    <div class="actions-cell">
                      {#if currentUserPermissions.includes('users:write')}
                        <Button size="small" variant="ghost" on:click={() => openAddMembership(user.username)}>
                          分配组
                        </Button>
                      {/if}
                      {#if currentUserPermissions.includes('users:transfer_super_admin') && user.username !== currentUserEmail}
                        <Button size="small" variant="danger" on:click={() => openTransferAdmin(user.username)}>
                          转让超管
                        </Button>
                      {/if}
                    </div>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {:else if activeSection === 'matrix'}
      <div class="section-card">
        <div class="card-header flex-header">
          <div>
            <h2>可视化权限树配置</h2>
            <p>按权限命名空间自动归类，纵向展示每个原子权限与用户组覆盖关系。</p>
            <div class="permission-catalog-status">
              <span class:live={permissionCatalog.length > 0}>{permissionCatalogSourceLabel}</span>
              <small>{permissionMeta.length} permissions · {permissionTree.length} branches</small>
            </div>
          </div>
          <div class="matrix-header-actions">
            <Button variant="ghost" on:click={() => { fetchPermissions(); fetchGroups(); }}>
              刷新目录
            </Button>
            {#if currentUserPermissions.includes('users:write')}
              <Button variant="ghost" on:click={() => showCreateGroupModal = true}>
                创建自定义组
              </Button>
            {/if}
          </div>
        </div>

        {#if groupMutationError}
          <div class="error-banner">{groupMutationError}</div>
        {/if}
        {#if groupMutationSuccess}
          <div class="success-banner">{groupMutationSuccess}</div>
        {/if}

        <div class="permission-summary-strip">
          {#each groups as group}
            <div class="group-coverage-card">
              <div>
                <span class="group-title-label badge-{group.name}">{group.displayName}</span>
                <p>{group.description || '暂无描述'}</p>
              </div>
              <div class="group-coverage-meta">
                <strong class="font-mono">{groupPermissionCount(group)}/{permissionMeta.length}</strong>
                <small>{groupMemberCount(group)} 成员</small>
              </div>
              <span class="coverage-bar" aria-hidden="true">
                <i style:width={groupPermissionRatio(group)}></i>
              </span>
              <div class="group-card-actions">
                <span>{isBuiltInGroup(group) ? '系统组' : '自定义组'}</span>
                {#if currentUserPermissions.includes('users:write') && !isBuiltInGroup(group)}
                  <button
                    type="button"
                    class="group-delete-btn"
                    disabled={groupDeletingName === group.name || groupMemberCount(group) > 0}
                    title={groupMemberCount(group) > 0 ? '请先移除该组下的成员身份' : `删除 ${group.displayName}`}
                    on:click={() => requestDeleteCustomGroup(group)}
                  >
                    {groupDeletingName === group.name ? '删除中' : '删除'}
                  </button>
                {/if}
              </div>
            </div>
          {/each}
        </div>

        <div class="permission-tree settings-data-scroll permission-tree-scroll" role="region" aria-label="权限树配置列表">
          {#each permissionTree as branch}
            <section class="permission-branch">
              <div class="branch-stem" aria-hidden="true"></div>
              <div class="branch-content">
                <div class="branch-header">
                  <div>
                    <span class="branch-key font-mono">{branch.key}</span>
                    <h3>{branch.label}</h3>
                    <p>{branch.summary}</p>
                  </div>
                  <span class="branch-count font-mono">{branch.permissions.length} permissions</span>
                </div>

                <div class="permission-nodes">
                  {#each branch.permissions as perm}
                    <div class="permission-node">
                      <div class="permission-node-main">
                        <span class="permission-code font-mono">{perm.code}</span>
                        <strong>{perm.name}</strong>
                        <p>{perm.desc || '暂无权限说明'}</p>
                      </div>
                      <div class="permission-group-toggles">
                        {#each groups as group}
                          <button
                            type="button"
                            class:enabled={groupHasPermission(group, perm.code)}
                            class:locked={group.name === 'super_admin'}
                            disabled={!currentUserPermissions.includes('users:write') || group.name === 'super_admin'}
                            aria-pressed={groupHasPermission(group, perm.code)}
                            title={group.name === 'super_admin' ? '超级管理员默认拥有全部权限' : `${group.displayName} ${groupHasPermission(group, perm.code) ? '已拥有' : '未拥有'} ${perm.code}`}
                            on:click={() => togglePermissionInMatrix(group, perm.code)}
                          >
                            <span>{group.displayName}</span>
                            <small>{groupHasPermission(group, perm.code) ? 'ON' : 'OFF'}</small>
                          </button>
                        {/each}
                      </div>
                    </div>
                  {/each}
                </div>
              </div>
            </section>
          {/each}
        </div>
      </div>
    {:else if activeSection === 'policies'}
      <div class="section-card">
        <div class="policy-task-toolbar">
          <div class="policy-workspace-summary">
            <span class="audit-kicker font-mono">授权工作台</span>
            <span>{authorizationPolicies.length} 条策略 · {authorizationAuditLogs.length} 条决策审计</span>
          </div>
          <Button variant="ghost" on:click={() => { fetchAuthorizationPolicies(); fetchAuthorizationAuditLogs(); }}>
            刷新策略
          </Button>
        </div>

        {#if authorizationPolicyError}
          <div class="error-banner">{authorizationPolicyError}</div>
        {/if}
        {#if authorizationPolicySuccess}
          <div class="success-banner">{authorizationPolicySuccess}</div>
        {/if}

        <div class="settings-task-tabs" role="tablist" aria-label="策略化授权任务">
          <button
            id="policy-tab-list"
            type="button"
            role="tab"
            aria-selected={policyWorkspaceTab === 'list'}
            aria-controls="policy-panel-list"
            class:active={policyWorkspaceTab === 'list'}
            on:click={() => setPolicyWorkspaceTab('list')}
          >
            <span>策略列表</span><small>{authorizationPolicies.length}</small>
          </button>
          <button
            id="policy-tab-create"
            type="button"
            role="tab"
            aria-selected={policyWorkspaceTab === 'create'}
            aria-controls="policy-panel-create"
            class:active={policyWorkspaceTab === 'create'}
            on:click={() => setPolicyWorkspaceTab('create')}
          >
            <span>新建策略</span>
          </button>
          <button
            id="policy-tab-explain"
            type="button"
            role="tab"
            aria-selected={policyWorkspaceTab === 'explain'}
            aria-controls="policy-panel-explain"
            class:active={policyWorkspaceTab === 'explain'}
            on:click={() => setPolicyWorkspaceTab('explain')}
          >
            <span>授权解释</span>
          </button>
          <button
            id="policy-tab-audit"
            type="button"
            role="tab"
            aria-selected={policyWorkspaceTab === 'audit'}
            aria-controls="policy-panel-audit"
            class:active={policyWorkspaceTab === 'audit'}
            on:click={() => setPolicyWorkspaceTab('audit')}
          >
            <span>授权审计</span><small>{authorizationAuditLogs.length}</small>
          </button>
        </div>

        {#if policyWorkspaceTab === 'create'}
        <div class="policy-template-strip" aria-label="策略快速模板">
          {#each policyQuickStarts as template}
            <button type="button" on:click={() => applyPolicyQuickStart(template)}>
              <span>{template.label}</span>
              <small class="font-mono">{template.effect} · {template.action}</small>
            </button>
          {/each}
        </div>

        <div class="policy-workbench refined">
          <div
            id="policy-panel-create"
            class="policy-panel policy-builder"
            role="tabpanel"
            aria-labelledby="policy-tab-create"
            tabindex="0"
          >
            <div class="policy-panel-header">
              <span class="audit-kicker font-mono">策略编辑</span>
              <h3>新增授权策略</h3>
            </div>

            <div class="policy-builder-grid">
              <div class="policy-choice-block">
                <span class="field-label">授权效果</span>
                <div class="choice-row">
                  {#each effectOptions as option}
                    <button
                      type="button"
                      class:active={policyForm.effect === option.value}
                      class={`choice-card effect-${option.value}`}
                      aria-pressed={policyForm.effect === option.value}
                      on:click={() => setPolicyField('effect', option.value)}
                    >
                      <strong>{option.label}</strong>
                      <small>{option.desc}</small>
                    </button>
                  {/each}
                </div>
              </div>

              <div class="policy-choice-block">
                <span class="field-label">主体类型</span>
                <div class="segmented-pills">
                  {#each subjectTypeOptions as option}
                    <button
                      type="button"
                      class:active={policyForm.subject_type === option.value}
                      aria-pressed={policyForm.subject_type === option.value}
                      on:click={() => setPolicyField('subject_type', option.value)}
                    >
                      {option.label}
                    </button>
                  {/each}
                </div>
              </div>

              {#if policyForm.subject_type !== 'any'}
                <div class="policy-choice-block policy-wide">
                  <label for="policy-subject-id">主体标识</label>
                  {#if policyForm.subject_type === 'group'}
                    <div class="suggestion-pills">
                      {#each groups.filter(group => group.name !== 'super_admin') as group}
                        <button
                          type="button"
                          class:active={policyForm.subject_id === group.name}
                          aria-pressed={policyForm.subject_id === group.name}
                          on:click={() => setPolicyField('subject_id', group.name)}
                        >
                          {group.displayName}
                        </button>
                      {/each}
                    </div>
                  {:else}
                    <div class="suggestion-pills">
                      {#each users.slice(0, 8) as user}
                        <button
                          type="button"
                          class:active={policyForm.subject_id === user.username}
                          aria-pressed={policyForm.subject_id === user.username}
                          on:click={() => setPolicyField('subject_id', user.username)}
                        >
                          {user.name || user.username}
                        </button>
                      {/each}
                    </div>
                  {/if}
                  <input id="policy-subject-id" bind:value={policyForm.subject_id} class="custom-input font-mono" placeholder={policyForm.subject_type === 'group' ? 'member / admin' : 'user@example.com'} />
                </div>
              {/if}

              <div class="policy-choice-block policy-wide">
                <label for="policy-action">动作权限</label>
                <div class="action-chip-grid" aria-label="可选动作权限">
                  {#each policyActionOptions as option}
                    <button
                      type="button"
                      class:active={policyForm.action === option.code}
                      aria-pressed={policyForm.action === option.code}
                      on:click={() => selectPolicyAction(option.code)}
                    >
                      <span>{option.name}</span>
                      <small class="font-mono">{option.code}</small>
                    </button>
                  {/each}
                </div>
                <input id="policy-action" bind:value={policyForm.action} class="custom-input font-mono" placeholder="config:write" />
              </div>

              <div class="policy-choice-block">
                <span class="field-label">资源类型</span>
                <div class="segmented-pills wrap">
                  {#each resourceTypeOptions as option}
                    <button
                      type="button"
                      class:active={policyForm.resource_type === option.value}
                      aria-pressed={policyForm.resource_type === option.value}
                      on:click={() => setPolicyField('resource_type', option.value)}
                    >
                      {option.label}
                    </button>
                  {/each}
                </div>
              </div>

              <div class="policy-choice-block">
                <label for="policy-resource-id">资源标识</label>
                <input id="policy-resource-id" bind:value={policyForm.resource_id} class="custom-input font-mono" placeholder="可留空或填写具体资源" />
              </div>

              <div class="policy-choice-block">
                <span class="field-label">作用域</span>
                <div class="segmented-pills">
                  {#each scopeOptions as option}
                    <button
                      type="button"
                      class:active={policyForm.scope === option.value}
                      aria-pressed={policyForm.scope === option.value}
                      on:click={() => setPolicyField('scope', option.value)}
                    >
                      {option.label}
                    </button>
                  {/each}
                </div>
              </div>

              <div class="policy-choice-block">
                <label for="policy-scope-id">Scope ID</label>
                <input id="policy-scope-id" bind:value={policyForm.scope_id} disabled={policyForm.scope === 'global'} class="custom-input font-mono" placeholder="repo 名称，可留空" />
              </div>

              <div class="policy-choice-block">
                <span class="field-label">优先级</span>
                <div class="priority-stepper">
                  <button type="button" on:click={() => stepPolicyPriority(-10)}>-10</button>
                  <strong class="font-mono">{policyForm.priority}</strong>
                  <button type="button" on:click={() => stepPolicyPriority(10)}>+10</button>
                </div>
              </div>

              <div class="policy-choice-block">
                <span class="field-label">状态</span>
                <button
                  type="button"
                  class="toggle-pill"
                  class:on={policyForm.enabled}
                  role="switch"
                  aria-checked={policyForm.enabled}
                  on:click={() => setPolicyField('enabled', !policyForm.enabled)}
                >
                  <span>{policyForm.enabled ? '已启用' : '已停用'}</span>
                  <i></i>
                </button>
              </div>

              <div class="policy-choice-block policy-wide">
                <label for="policy-reason">审计原因</label>
                <input id="policy-reason" bind:value={policyForm.reason} class="custom-input" placeholder="写清为什么允许或拒绝，便于审计解释" />
              </div>
            </div>

            <div class="policy-actions">
              <Button variant="primary" loading={authorizationPolicySaving} on:click={saveAuthorizationPolicy}>
                保存策略
              </Button>
            </div>
          </div>
        </div>
        {:else if policyWorkspaceTab === 'explain'}
        <div class="policy-workbench refined">
          <div
            id="policy-panel-explain"
            class="policy-panel"
            role="tabpanel"
            aria-labelledby="policy-tab-explain"
            tabindex="0"
          >
            <div class="policy-panel-header">
              <span class="audit-kicker font-mono">授权解释</span>
              <h3>授权解释器</h3>
            </div>
            {#if authorizationExplainError}
              <div class="error-banner">{authorizationExplainError}</div>
            {/if}
            <div class="policy-form-grid explain-grid">
              <div class="field-item policy-wide">
                <label for="explain-username">Username</label>
                <input id="explain-username" bind:value={explainForm.username} class="custom-input font-mono" placeholder="user@example.com" />
              </div>
              <div class="field-item policy-wide">
                <label for="explain-action">Action</label>
                <div class="action-chip-grid compact" aria-label="授权解释动作权限">
                  {#each policyActionOptions as option}
                    <button
                      type="button"
                      class:active={explainForm.action === option.code}
                      aria-pressed={explainForm.action === option.code}
                      on:click={() => selectExplainAction(option.code)}
                    >
                      <span>{option.name}</span>
                      <small class="font-mono">{option.code}</small>
                    </button>
                  {/each}
                </div>
                <input id="explain-action" bind:value={explainForm.action} class="custom-input font-mono" />
              </div>
              <div class="field-item">
                <label for="explain-resource-type">Resource Type</label>
                <input id="explain-resource-type" bind:value={explainForm.resource_type} class="custom-input font-mono" />
              </div>
              <div class="field-item">
                <label for="explain-resource-id">Resource ID</label>
                <input id="explain-resource-id" bind:value={explainForm.resource_id} class="custom-input font-mono" />
              </div>
              <div class="field-item">
                <span class="field-label">Scope</span>
                <div class="segmented-pills">
                  {#each scopeOptions as option}
                    <button
                      type="button"
                      class:active={explainForm.scope === option.value}
                      aria-pressed={explainForm.scope === option.value}
                      on:click={() => setExplainField('scope', option.value)}
                    >
                      {option.label}
                    </button>
                  {/each}
                </div>
              </div>
              <div class="field-item">
                <label for="explain-scope-id">Scope ID</label>
                <input id="explain-scope-id" bind:value={explainForm.scope_id} disabled={explainForm.scope === 'global'} class="custom-input font-mono" />
              </div>
            </div>
            <div class="policy-actions">
              <Button variant="secondary" on:click={explainAuthorization}>解释授权结果</Button>
            </div>

            {#if authorizationDecision}
              <div class="decision-card {authorizationDecision.allowed ? 'allowed' : 'denied'}">
                <span>{authorizationDecision.allowed ? 'Allowed' : 'Denied'}</span>
                <strong>{authorizationDecision.reason}</strong>
                <p class="font-mono">
                  scope={authorizationDecision.scope || 'global'} · risk={authorizationDecision.risk_level || 'low'}
                  {#if authorizationDecision.missing_permission}
                    · missing={authorizationDecision.missing_permission}
                  {/if}
                </p>
                {#if authorizationDecision.matched_policy}
                  <p class="font-mono">matched_policy=#{authorizationDecision.matched_policy.id} {authorizationDecision.matched_policy.effect}</p>
                {/if}
              </div>
            {/if}
          </div>
        </div>
        {:else if policyWorkspaceTab === 'list'}
        <div class="policy-grid">
          <div
            id="policy-panel-list"
            class="policy-panel"
            role="tabpanel"
            aria-labelledby="policy-tab-list"
            tabindex="0"
          >
            <div class="policy-panel-header">
              <span class="audit-kicker font-mono">策略清单</span>
              <h3>策略列表</h3>
            </div>
            <!-- svelte-ignore a11y_no_noninteractive_tabindex (bounded table needs keyboard scrolling) -->
            <div class="table-responsive policy-table-wrap" role="region" aria-label="授权策略列表，可横向或纵向滚动" tabindex="0">
              <table class="policy-table font-mono">
                <thead>
                  <tr>
                    <th>Effect</th>
                    <th>Subject</th>
                    <th>Action</th>
                    <th>Resource</th>
                    <th>Scope</th>
                    <th>Priority</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {#each authorizationPolicies as policy}
                    <tr>
                      <td><span class="policy-effect effect-{policy.effect}">{policy.effect}</span></td>
                      <td>{policy.subject_type}:{policy.subject_id || '*'}</td>
                      <td>{policy.action}</td>
                      <td>{policy.resource_type || '*'}:{policy.resource_id || '*'}</td>
                      <td>{policyScopeLabel(policy)}</td>
                      <td>{policy.priority}</td>
                      <td>{policy.enabled ? 'enabled' : 'disabled'}</td>
                    </tr>
                  {:else}
                    <tr>
                      <td colspan="7" class="empty-table-cell">暂无策略，当前仅使用用户组权限矩阵。</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </div>
        </div>
        {:else}
        <div class="policy-grid">
          <div
            id="policy-panel-audit"
            class="policy-panel"
            role="tabpanel"
            aria-labelledby="policy-tab-audit"
            tabindex="0"
          >
            <div class="policy-panel-header">
              <span class="audit-kicker font-mono">授权审计</span>
              <h3>授权决策审计</h3>
            </div>
            <!-- svelte-ignore a11y_no_noninteractive_tabindex (bounded audit list needs keyboard scrolling) -->
            <div class="decision-log-list" role="region" aria-label="授权决策审计记录" tabindex="0">
              {#each authorizationAuditLogs as log}
                <div class="decision-log-item {log.allowed ? 'allowed' : 'denied'}">
                  <div>
                    <strong class="font-mono">{log.subject_username || 'unknown'} · {log.action}</strong>
                    <p>{log.reason || log.missing_permission || '无解释'}</p>
                  </div>
                  <span class="font-mono">{log.allowed ? 'allow' : 'deny'} · {log.risk_level || 'low'}</span>
                </div>
              {:else}
                <div class="empty-version-state">暂无授权审计记录。拒绝或高风险授权会自动进入这里。</div>
              {/each}
            </div>
          </div>
        </div>
        {/if}
      </div>
    {:else if activeSection === 'audit'}
      <div class="section-card">
        <div class="card-header">
          <h2>安全审计日志痕迹 (Audit Logs)</h2>
          <p>系统自动记录所有敏感配置修改、鉴权登录及用户权限矩阵分配的操作行迹，以便溯源审计。</p>
        </div>

        <!-- svelte-ignore a11y_no_noninteractive_tabindex (bounded audit table needs keyboard scrolling) -->
        <div class="table-responsive settings-data-scroll audit-data-scroll" role="region" aria-label="安全审计日志，可横向或纵向滚动" tabindex="0">
          <table class="audit-table font-mono">
            <thead>
              <tr>
                <th style="width: 150px;">操作时间</th>
                <th>操作人员</th>
                <th style="width: 150px;">敏感行为</th>
                <th>行为描述</th>
                <th style="width: 120px;">IP 地址</th>
              </tr>
            </thead>
            <tbody>
              {#each auditLogs as log}
                <tr>
                  <td class="text-xs">{new Date(log.created_at).toLocaleString()}</td>
                  <td>
                    <span class="actor-tag">{log.actor_name}</span>
                    <span class="actor-sub font-mono">({log.actor_username})</span>
                  </td>
                  <td>
                    <span class="action-badge badge-action-{log.action}">{log.action}</span>
                  </td>
                  <td class="text-sm text-slate-300">{log.detail}</td>
                  <td class="text-xs text-slate-400">{log.ip_address}</td>
                </tr>
              {:else}
                <tr>
                  <td colspan="5" style="text-align: center; color: #64748b; padding: 24px;">暂无敏感操作审计记录</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {/if}
          </div>

    {#if isIntegrationSection}
          <aside class="settings-audit-pane" aria-label="配置版本审计">
            <section class="config-audit-panel">
              <div class="config-audit-header">
                <div>
                  <span class="audit-kicker font-mono">版本审计</span>
                  <h3>配置版本审计与回滚 <span class="audit-scope">{activeSectionMeta.label}</span></h3>
                </div>
                <Button size="small" variant="ghost" on:click={() => fetchConfigVersions()}>刷新记录</Button>
              </div>

              {#if configVersionError}
                <div class="config-version-error">{configVersionError}</div>
              {/if}

              {#if configVersions.length === 0}
                <div class="empty-version-state">暂无数据库配置版本。首次保存后会自动生成可审计快照。</div>
              {:else if visibleConfigVersions.length === 0}
                <div class="empty-version-state">当前配置页暂无独立版本记录。</div>
              {:else}
                <div class="version-layout">
                  <div class="version-list" role="list" aria-label="配置版本">
                    {#each visibleConfigVersions as version}
                      <button
                        type="button"
                        class="version-item {selectedConfigVersion?.id === version.id ? 'active' : ''}"
                        on:click={() => selectedConfigVersionID = version.id}
                      >
                        <span class="version-title">v{version.version}</span>
                        <span class="version-meta">{formatDateTime(version.created_at)}</span>
                        <span class="version-sections">{formatChangedSections(version.changed_sections || [])}</span>
                      </button>
                    {/each}
                  </div>

                  {#if selectedConfigVersion}
                    <div class="version-detail">
                      <div class="version-detail-header">
                        <div>
                          <span class="version-title">版本 v{selectedConfigVersion.version}</span>
                          <p>{selectedConfigVersion.actor_name || selectedConfigVersion.actor_id || 'system'} · {selectedConfigVersion.source || 'manual'}</p>
                        </div>
                        <Button
                          size="small"
                          variant="danger"
                          loading={rollbackLoadingID === selectedConfigVersion.id}
                          disabled={rollbackLoadingID !== null || selectedConfigVersion.id === configVersions[0]?.id}
                          on:click={() => rollbackConfigVersion(selectedConfigVersion)}
                        >
                          回滚到此版本
                        </Button>
                      </div>

                      {#if selectedConfigVersion.rollback_from_version_id}
                        <div class="rollback-note">由 v{selectedConfigVersion.rollback_from_version_id} 回滚生成</div>
                      {/if}

                      <div class="diff-table">
                        {#each selectedConfigVersion.diff || [] as diff}
                          <div class="diff-row">
                            <span class="diff-path font-mono">{diff.path}</span>
                            <div class="diff-value before">
                              <span>变更前</span>
                              <pre class="font-mono">{formatDiffValue(diff.before)}</pre>
                            </div>
                            <span class="diff-arrow">→</span>
                            <div class="diff-value after">
                              <span>变更后</span>
                              <pre class="font-mono">{formatDiffValue(diff.after)}</pre>
                            </div>
                          </div>
                        {:else}
                          <div class="diff-empty">该版本为初始快照或无字段差异。</div>
                        {/each}
                      </div>
                    </div>
                  {/if}
                </div>
              {/if}
            </section>
          </aside>
            {/if}
          </div>
      </div>
    </section>
  </main>
</div>

<!-- Modal 1: Add scoped membership -->
{#if showAddMembershipModal}
  <div class="modal-overlay">
    <div class="modal-card">
      <div class="modal-header">
        <h3>分配权限组与作用域隔离</h3>
        <button class="close-modal-btn" on:click={() => showAddMembershipModal = false}>&times;</button>
      </div>
      <div class="modal-body">
        {#if membershipError}
          <div class="error-banner">{membershipError}</div>
        {/if}
        {#if membershipSuccess}
          <div class="success-banner">{membershipSuccess}</div>
        {/if}
        
        <div class="field-item">
          <label for="membership-target-user">目标成员账户</label>
          <input id="membership-target-user" type="text" value={membershipTargetUser} disabled class="input-disabled font-mono" />
        </div>
        <div class="field-item relative">
          <label for="membership-selected-group-trigger">分配目标用户组</label>
          <button 
            id="membership-selected-group-trigger"
            type="button" 
            class="dropdown-trigger-btn font-mono"
            on:click={() => showGroupDropdown = !showGroupDropdown}
          >
            <span>{groups.find(g => g.name === membershipSelectedGroup)?.displayName || membershipSelectedGroup} ({membershipSelectedGroup})</span>
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="dropdown-chevron"><polyline points="6 9 12 15 18 9"></polyline></svg>
          </button>
          
          {#if showGroupDropdown}
            <button type="button" class="dropdown-backdrop-overlay" aria-label="关闭用户组下拉菜单" on:click={() => showGroupDropdown = false}></button>
            <div class="dropdown-options-list glass-panel font-mono">
              {#each groups as g}
                {#if g.name !== 'super_admin'}
                  <button 
                    type="button"
                    class="dropdown-option-item {membershipSelectedGroup === g.name ? 'selected' : ''}"
                    on:click={() => {
                      membershipSelectedGroup = g.name;
                      showGroupDropdown = false;
                    }}
                  >
                    {g.displayName} ({g.name})
                  </button>
                {/if}
              {/each}
            </div>
          {/if}
        </div>
        <div class="field-item">
          <span class="field-label">分配作用域等级 (Scope)</span>
          <div class="radio-group font-mono">
            <label>
              <input type="radio" value="global" bind:group={membershipScope} /> 全局级作用域 (Global)
            </label>
            <label>
              <input type="radio" value="repo" bind:group={membershipScope} /> 仓库项目级隔离 (Repo-scoped)
            </label>
          </div>
        </div>
        
        {#if membershipScope === 'repo'}
          <div class="field-item">
            <label for="membership-scope-id">特定作用域仓库标识 (ProjectID/RepoName)</label>
            <input id="membership-scope-id" type="text" bind:value={membershipScopeID} placeholder="输入关联的代码仓名称，如 frontend-dashboard" class="custom-input font-mono" />
            <p class="field-desc">该组权限将仅在用户操作指定的仓库项目时生效，其余仓库对该用户无管理权限。</p>
          </div>
        {/if}
      </div>
      <div class="modal-footer">
        <Button variant="ghost" on:click={() => showAddMembershipModal = false}>取消</Button>
        <Button on:click={saveMembership}>确定保存</Button>
      </div>
    </div>
  </div>
{/if}

{#if showRemoveMembershipModal && removeMembershipTarget}
  <div class="modal-overlay">
    <div class="modal-card remove-membership-modal" role="dialog" aria-modal="true" aria-labelledby="remove-membership-title">
      <div class="modal-header delete-modal-header">
        <div>
          <span class="danger-kicker font-mono">Remove membership</span>
          <h3 id="remove-membership-title">移除成员用户组</h3>
        </div>
        <button
          class="close-modal-btn"
          aria-label="关闭移除成员用户组确认弹窗"
          on:click={closeRemoveMembershipModal}
          disabled={removingMembership}
        >
          &times;
        </button>
      </div>
      <div class="modal-body">
        <div class="delete-target-card">
          <div>
            <span class="group-title-label badge-{removeMembershipTarget.membership.group_name}">
              {removeMembershipTarget.membership.group_display_name}
            </span>
            <p class="font-mono">{removeMembershipTarget.username}</p>
          </div>
          <div class="delete-target-stats">
            <strong class="font-mono">{membershipScopeLabel(removeMembershipTarget.membership)}</strong>
            <small>scope</small>
          </div>
        </div>
        <p class="delete-confirm-copy">
          移除后，该成员将不再拥有此用户组在当前作用域下的权限。该操作不会删除成员账号。
        </p>
        {#if removeMembershipError}
          <div class="error-banner">{removeMembershipError}</div>
        {/if}
      </div>
      <div class="modal-footer">
        <Button variant="ghost" on:click={closeRemoveMembershipModal} disabled={removingMembership}>
          取消
        </Button>
        <Button variant="danger" on:click={confirmRemoveMembership} loading={removingMembership}>
          确认移除
        </Button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal 2: Transfer Super Admin -->
{#if showTransferModal}
  <div class="modal-overlay">
    <div class="modal-card">
      <div class="modal-header">
        <h3 style="color: #ef4444;">警告：超级管理员控制权转让</h3>
        <button class="close-modal-btn" on:click={() => showTransferModal = false}>&times;</button>
      </div>
      <div class="modal-body">
        {#if transferError}
          <div class="error-banner">{transferError}</div>
        {/if}
        <Alert type="warning">
          转让超级管理员是高危操作。转让完成后，您的账号将被降级为「系统管理员组」，原有的超级管理员管理权与审批权将不可撤销地转让给他人。
        </Alert>
        <div class="field-item" style="margin-top: 16px;">
          <label for="transfer-target-username">目标接收者账户</label>
          <input id="transfer-target-username" type="text" value={transferTargetUsername} disabled class="input-disabled font-mono" />
        </div>
        <div class="field-item">
          <label for="transfer-confirm-name">请输入目标用户的真实姓名以确认安全转让</label>
          <input 
            id="transfer-confirm-name"
            type="text" 
            bind:value={transferConfirmName} 
            placeholder="例如: Eddie" 
            class="custom-input font-mono" 
          />
        </div>
      </div>
      <div class="modal-footer">
        <Button variant="ghost" on:click={() => showTransferModal = false}>取消</Button>
        <Button variant="danger" on:click={executeTransfer}>确认转让</Button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal 3: Create Custom Group -->
{#if showCreateGroupModal}
  <div class="modal-overlay">
    <div class="modal-card">
      <div class="modal-header">
        <h3>创建自定义组 (Role Group)</h3>
        <button class="close-modal-btn" on:click={() => showCreateGroupModal = false}>&times;</button>
      </div>
      <div class="modal-body">
        {#if createGroupError}
          <div class="error-banner">{createGroupError}</div>
        {/if}
        <div class="field-item">
          <label for="new-group-name">组代码名称 (英文小写下划线，作为系统唯一 Key)</label>
          <input id="new-group-name" type="text" bind:value={newGroupName} placeholder="如 developer_lead" class="custom-input font-mono" />
        </div>
        <div class="field-item">
          <label for="new-group-display-name">显示名称</label>
          <input id="new-group-display-name" type="text" bind:value={newGroupDisplayName} placeholder="如 研发主导组" class="custom-input" />
        </div>
        <div class="field-item">
          <label for="new-group-description">组描述说明</label>
          <textarea id="new-group-description" bind:value={newGroupDescription} placeholder="请输入该组的使用场景和权限划分意图" class="custom-textarea" rows="3"></textarea>
        </div>
      </div>
      <div class="modal-footer">
        <Button variant="ghost" on:click={() => showCreateGroupModal = false}>取消</Button>
        <Button on:click={createCustomGroup}>立即创建</Button>
      </div>
    </div>
  </div>
{/if}

{#if showDeleteGroupModal && deleteGroupTarget}
  <div class="modal-overlay">
    <div class="modal-card delete-group-modal" role="dialog" aria-modal="true" aria-labelledby="delete-group-title">
      <div class="modal-header delete-modal-header">
        <div>
          <span class="danger-kicker font-mono">Delete custom group</span>
          <h3 id="delete-group-title">删除自定义用户组</h3>
        </div>
        <button
          class="close-modal-btn"
          aria-label="关闭删除用户组确认弹窗"
          on:click={() => { showDeleteGroupModal = false; deleteGroupTarget = null; }}
        >
          &times;
        </button>
      </div>
      <div class="modal-body">
        <div class="delete-target-card">
          <div>
            <span class="group-title-label badge-{deleteGroupTarget.name}">{deleteGroupTarget.displayName}</span>
            <p>{deleteGroupTarget.description || '暂无描述'}</p>
          </div>
          <div class="delete-target-stats">
            <strong class="font-mono">{groupPermissionCount(deleteGroupTarget)}</strong>
            <small>permissions</small>
          </div>
        </div>
        <p class="delete-confirm-copy">
          删除后，该组的权限映射会被一并清理。该操作不会删除成员账号，也不会影响系统内置组。
        </p>
        {#if groupMutationError}
          <div class="error-banner">{groupMutationError}</div>
        {/if}
      </div>
      <div class="modal-footer">
        <Button
          variant="ghost"
          on:click={() => { showDeleteGroupModal = false; deleteGroupTarget = null; }}
          disabled={groupDeletingName === deleteGroupTarget.name}
        >
          取消
        </Button>
        <Button
          variant="danger"
          on:click={deleteCustomGroup}
          loading={groupDeletingName === deleteGroupTarget.name}
        >
          确认删除
        </Button>
      </div>
    </div>
  </div>
{/if}

<style>
  .settings-container {
    display: block;
    flex: 1;
    width: 100%;
    height: auto;
    min-height: 0;
    overflow: visible;
    padding-bottom: 32px;
    box-sizing: border-box;
  }

  .settings-main {
    min-width: 0;
    height: auto;
    min-height: 0;
    max-height: none;
    display: flex;
    flex-direction: column;
    overflow-y: auto;
    padding-right: 8px;
    box-sizing: border-box;
  }

  .settings-main::-webkit-scrollbar {
    width: 6px;
  }

  .settings-main::-webkit-scrollbar-track {
    background: rgba(15, 23, 42, 0.3);
    border-radius: 3px;
  }

  .settings-main::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.3);
    border-radius: 3px;
    transition: background 0.2s;
  }

  .settings-main::-webkit-scrollbar-thumb:hover {
    background: rgba(99, 102, 241, 0.5);
  }

  .section-card {
    flex: 0 0 auto;
    min-height: 0;
    height: auto;
    overflow: visible;
    box-sizing: border-box;
    background: linear-gradient(180deg, rgba(15, 23, 42, 0.98), rgba(10, 16, 31, 0.98));
    border: 1px solid rgba(71, 85, 105, 0.46);
    border-radius: 12px;
    padding: 24px;
    box-shadow: 0 18px 48px -34px rgba(0, 0, 0, 0.78), inset 0 1px 0 rgba(255, 255, 255, 0.035);
  }

  .config-audit-panel {
    flex: 0 0 auto;
    margin-top: 12px;
    background: rgba(11, 19, 41, 0.72);
    border: 1px solid rgba(51, 65, 85, 0.36);
    border-radius: 10px;
    padding: 12px;
  }

  .policy-workbench,
  .policy-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 16px;
    margin-top: 16px;
  }

  .policy-grid {
    align-items: start;
  }

  .policy-template-strip {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
    margin: 14px 0 16px;
  }

  .policy-template-strip button {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.52);
    background: rgba(2, 6, 23, 0.3);
    color: #cbd5e1;
    border-radius: 8px;
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    text-align: left;
    cursor: pointer;
    transition: transform 0.18s ease, border-color 0.18s ease, background 0.18s ease;
  }

  .policy-template-strip button:hover {
    transform: translateY(-1px);
    border-color: rgba(56, 189, 248, 0.42);
    background: rgba(8, 47, 73, 0.22);
  }

  .policy-template-strip span {
    color: #f8fafc;
    font-size: 0.86rem;
    font-weight: 800;
  }

  .policy-template-strip small {
    color: #64748b;
    overflow-wrap: anywhere;
  }

  .policy-workbench.refined {
    grid-template-columns: minmax(0, 1.28fr) minmax(320px, 0.72fr);
  }

  .policy-panel {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.48);
    background: rgba(15, 23, 42, 0.42);
    border-radius: 8px;
    padding: 14px;
  }

  .policy-panel-header {
    border-bottom: 1px solid rgba(51, 65, 85, 0.36);
    padding-bottom: 10px;
    margin-bottom: 14px;
  }

  .policy-panel-header h3 {
    margin: 4px 0 0 0;
    color: #f8fafc;
    font-size: 1rem;
  }

  .policy-builder {
    background:
      linear-gradient(135deg, rgba(56, 189, 248, 0.08), transparent 36%),
      rgba(15, 23, 42, 0.42);
  }

  .policy-builder-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 14px;
  }

  .policy-choice-block {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .policy-choice-block label,
  .field-label {
    color: #94a3b8;
    font-size: 0.78rem;
    font-weight: 800;
  }

  .choice-row,
  .segmented-pills,
  .suggestion-pills,
  .action-chip-grid {
    display: flex;
    gap: 8px;
  }

  .choice-row {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .choice-card,
  .segmented-pills button,
  .suggestion-pills button,
  .action-chip-grid button,
  .priority-stepper button,
  .toggle-pill {
    border: 1px solid rgba(51, 65, 85, 0.56);
    background: rgba(2, 6, 23, 0.36);
    color: #94a3b8;
    border-radius: 8px;
    cursor: pointer;
    transition: transform 0.16s ease, border-color 0.16s ease, background 0.16s ease, color 0.16s ease;
  }

  .choice-card {
    min-height: 70px;
    padding: 10px;
    display: flex;
    flex-direction: column;
    gap: 5px;
    text-align: left;
  }

  .choice-card strong {
    color: #e2e8f0;
    font-size: 0.92rem;
  }

  .choice-card small {
    color: #64748b;
    line-height: 1.35;
  }

  .choice-card:hover,
  .segmented-pills button:hover,
  .suggestion-pills button:hover,
  .action-chip-grid button:hover,
  .priority-stepper button:hover,
  .toggle-pill:hover {
    transform: translateY(-1px);
    color: #e2e8f0;
    border-color: rgba(148, 163, 184, 0.52);
  }

  .choice-card.active.effect-deny {
    border-color: rgba(248, 113, 113, 0.48);
    background: rgba(127, 29, 29, 0.24);
  }

  .choice-card.active.effect-allow {
    border-color: rgba(52, 211, 153, 0.48);
    background: rgba(6, 78, 59, 0.22);
  }

  .segmented-pills {
    flex-wrap: nowrap;
    background: rgba(2, 6, 23, 0.38);
    border: 1px solid rgba(51, 65, 85, 0.42);
    padding: 4px;
    border-radius: 10px;
  }

  .segmented-pills.wrap,
  .suggestion-pills,
  .action-chip-grid {
    flex-wrap: wrap;
    background: transparent;
    border: none;
    padding: 0;
  }

  .segmented-pills button,
  .suggestion-pills button {
    min-height: 34px;
    padding: 7px 10px;
    flex: 1;
    font-weight: 800;
    font-size: 0.78rem;
  }

  .segmented-pills.wrap button,
  .suggestion-pills button {
    flex: none;
  }

  .segmented-pills button.active,
  .suggestion-pills button.active {
    color: #f8fafc;
    border-color: rgba(56, 189, 248, 0.54);
    background: rgba(8, 47, 73, 0.5);
  }

  .action-chip-grid {
    max-height: 172px;
    overflow: auto;
    padding-right: 3px;
  }

  .action-chip-grid.compact {
    max-height: 128px;
  }

  .action-chip-grid button {
    min-width: 154px;
    max-width: 220px;
    min-height: 48px;
    padding: 8px 10px;
    display: flex;
    flex-direction: column;
    gap: 3px;
    text-align: left;
  }

  .action-chip-grid button.active {
    color: #f8fafc;
    border-color: rgba(99, 102, 241, 0.56);
    background: rgba(49, 46, 129, 0.36);
  }

  .action-chip-grid span {
    font-size: 0.78rem;
    font-weight: 800;
  }

  .action-chip-grid small {
    color: #64748b;
    overflow-wrap: anywhere;
  }

  .priority-stepper {
    display: grid;
    grid-template-columns: 54px minmax(0, 1fr) 54px;
    gap: 8px;
    align-items: stretch;
  }

  .priority-stepper strong {
    min-height: 38px;
    display: grid;
    place-items: center;
    border: 1px solid rgba(51, 65, 85, 0.56);
    background: rgba(2, 6, 23, 0.42);
    border-radius: 8px;
    color: #f8fafc;
  }

  .priority-stepper button {
    font-weight: 900;
    color: #cbd5e1;
  }

  .toggle-pill {
    min-height: 38px;
    padding: 4px 5px 4px 12px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    font-weight: 800;
  }

  .toggle-pill i {
    width: 24px;
    height: 24px;
    border-radius: 999px;
    background: #475569;
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.12);
  }

  .toggle-pill.on {
    color: #bbf7d0;
    border-color: rgba(52, 211, 153, 0.38);
    background: rgba(6, 78, 59, 0.24);
  }

  .toggle-pill.on i {
    background: #34d399;
  }

  .policy-form-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }

  .explain-grid {
    align-items: start;
  }

  .policy-wide {
    grid-column: 1 / -1;
  }

  .policy-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 12px;
  }

  .decision-card {
    margin-top: 14px;
    border-radius: 8px;
    padding: 12px;
    border: 1px solid rgba(51, 65, 85, 0.48);
    background: rgba(2, 6, 23, 0.32);
  }

  .decision-card span,
  .policy-effect {
    display: inline-flex;
    border-radius: 4px;
    padding: 2px 7px;
    font-size: 0.7rem;
    font-weight: 900;
    text-transform: uppercase;
    margin-bottom: 8px;
  }

  .decision-card.allowed span,
  .effect-allow {
    color: #34d399;
    background: rgba(16, 185, 129, 0.12);
    border: 1px solid rgba(16, 185, 129, 0.24);
  }

  .decision-card.denied span,
  .effect-deny {
    color: #f87171;
    background: rgba(239, 68, 68, 0.12);
    border: 1px solid rgba(239, 68, 68, 0.24);
  }

  .decision-card strong {
    display: block;
    color: #e2e8f0;
    font-size: 0.92rem;
    line-height: 1.45;
  }

  .decision-card p {
    margin: 6px 0 0 0;
    color: #94a3b8;
    font-size: 0.76rem;
    overflow-wrap: anywhere;
  }

  .policy-table-wrap {
    max-height: 360px;
    overflow: auto;
  }

  .policy-table td,
  .policy-table th {
    font-size: 0.76rem;
    white-space: nowrap;
  }

  .empty-table-cell {
    text-align: center;
    color: #64748b;
    padding: 22px;
  }

  .decision-log-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: 360px;
    overflow: auto;
  }

  .decision-log-item {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    border: 1px solid rgba(51, 65, 85, 0.42);
    background: rgba(2, 6, 23, 0.28);
    border-radius: 8px;
    padding: 10px;
  }

  .decision-log-item.denied {
    border-color: rgba(239, 68, 68, 0.2);
  }

  .decision-log-item.allowed {
    border-color: rgba(16, 185, 129, 0.2);
  }

  .decision-log-item strong {
    color: #e2e8f0;
    font-size: 0.78rem;
  }

  .decision-log-item p {
    margin: 4px 0 0 0;
    color: #94a3b8;
    font-size: 0.76rem;
    line-height: 1.4;
  }

  .decision-log-item > span {
    color: #64748b;
    font-size: 0.72rem;
    flex: none;
  }

  .config-audit-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.3);
    padding-bottom: 10px;
    margin-bottom: 10px;
  }

  .audit-kicker {
    color: #38bdf8;
    font-size: 0.66rem;
    font-weight: 900;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .config-audit-header h3 {
    margin: 4px 0 0 0;
    color: #f8fafc;
    font-size: 0.92rem;
  }

  .config-version-error,
  .empty-version-state,
  .rollback-note,
  .diff-empty {
    border-radius: 8px;
    padding: 10px 12px;
    font-size: 0.82rem;
    line-height: 1.45;
  }

  .config-version-error {
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.22);
    color: #fca5a5;
    margin-bottom: 12px;
  }

  .empty-version-state,
  .diff-empty {
    background: rgba(15, 23, 42, 0.52);
    border: 1px solid rgba(51, 65, 85, 0.42);
    color: #64748b;
  }

  .version-layout {
    display: grid;
    grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.35fr);
    gap: 8px;
  }

  .version-list {
    display: flex;
    align-items: stretch;
    gap: 6px;
    max-height: 108px;
    overflow-x: auto;
    overflow-y: hidden;
    padding: 0 0 4px 0;
    scrollbar-width: thin;
    scrollbar-color: rgba(99, 102, 241, 0.28) rgba(15, 23, 42, 0.36);
  }

  .version-list::-webkit-scrollbar {
    height: 8px;
  }

  .version-list::-webkit-scrollbar-track {
    background: rgba(15, 23, 42, 0.36);
    border-radius: 999px;
  }

  .version-list::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.38);
    border-radius: 999px;
  }

  .version-item {
    flex: 0 0 132px;
    border: 1px solid rgba(51, 65, 85, 0.48);
    background: rgba(15, 23, 42, 0.56);
    color: #cbd5e1;
    border-radius: 8px;
    padding: 7px 8px;
    text-align: left;
    cursor: pointer;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .version-item:hover,
  .version-item.active {
    border-color: rgba(99, 102, 241, 0.58);
    background: rgba(49, 46, 129, 0.28);
  }

  .version-title {
    color: #f8fafc;
    font-size: 0.82rem;
    font-weight: 800;
  }

  .version-meta,
  .version-sections {
    color: #64748b;
    font-size: 0.72rem;
    line-height: 1.35;
  }

  .version-detail {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.36);
    background: rgba(15, 23, 42, 0.24);
    border-radius: 8px;
    padding: 8px;
  }

  .version-detail-header {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    align-items: flex-start;
    margin-bottom: 8px;
  }

  .version-detail-header p {
    margin: 3px 0 0 0;
    color: #64748b;
    font-size: 0.76rem;
  }

  .rollback-note {
    background: rgba(245, 158, 11, 0.08);
    border: 1px solid rgba(245, 158, 11, 0.22);
    color: #fbbf24;
    margin-bottom: 10px;
  }

  .diff-table {
    display: flex;
    flex-direction: column;
    gap: 6px;
    max-height: 122px;
    overflow-y: auto;
    scrollbar-width: thin;
    scrollbar-color: rgba(99, 102, 241, 0.22) transparent;
  }

  .diff-table::-webkit-scrollbar,
  .diff-value::-webkit-scrollbar {
    width: 7px;
    height: 7px;
  }

  .diff-table::-webkit-scrollbar-track,
  .diff-value::-webkit-scrollbar-track {
    background: rgba(15, 23, 42, 0.24);
    border-radius: 999px;
  }

  .diff-table::-webkit-scrollbar-thumb,
  .diff-value::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.3);
    border-radius: 999px;
  }

  .diff-row {
    display: grid;
    grid-template-columns: 120px minmax(0, 1fr) 14px minmax(0, 1fr);
    gap: 6px;
    align-items: start;
    border-bottom: 1px solid rgba(51, 65, 85, 0.34);
    padding-bottom: 8px;
  }

  .diff-row:last-child {
    border-bottom: none;
    padding-bottom: 0;
  }

  .diff-path {
    color: #7dd3fc;
    font-size: 0.72rem;
    overflow-wrap: anywhere;
  }

  .diff-value {
    color: #94a3b8;
    font-size: 0.72rem;
    max-height: 48px;
    overflow: auto;
    overflow-wrap: anywhere;
    white-space: pre-wrap;
  }

  .diff-value.after {
    color: #cbd5e1;
  }

  .diff-arrow {
    color: #64748b;
    text-align: center;
  }

  .card-header {
    border-bottom: 1px solid rgba(51, 65, 85, 0.4);
    padding-bottom: 16px;
    margin-bottom: 20px;
  }

  .card-header h2 {
    margin: 0 0 6px 0;
    font-size: 1.4rem;
    color: #f8fafc;
  }

  .card-header p {
    margin: 0;
    font-size: 0.9rem;
    color: #94a3b8;
    line-height: 1.5;
  }

  .flex-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    flex-wrap: wrap;
  }

  /* Table styling */
  .table-responsive {
    overflow-x: auto;
    border-radius: 8px;
    border: 1px solid rgba(51, 65, 85, 0.4);
  }

  table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
  }

  th {
    background: #0b1329;
    color: #94a3b8;
    font-size: 0.8rem;
    font-weight: 700;
    padding: 12px 16px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.6);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  td {
    padding: 14px 16px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.4);
    font-size: 0.9rem;
    color: #e2e8f0;
  }

  tr:last-child td {
    border-bottom: none;
  }

  tr:hover td {
    background: rgba(30, 41, 59, 0.2);
  }

  /* Tags & Badges */
  .tags-container {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .membership-badge {
    display: inline-flex;
    align-items: center;
    font-size: 0.75rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    padding: 3px 8px;
    border-radius: 4px;
    gap: 6px;
  }

  .badge-super_admin {
    background: rgba(239, 68, 68, 0.15);
    color: #fca5a5;
    border: 1px solid rgba(239, 68, 68, 0.3);
  }

  .badge-admin {
    background: rgba(168, 85, 247, 0.15);
    color: #d8b4fe;
    border: 1px solid rgba(168, 85, 247, 0.3);
  }

  .badge-member {
    background: rgba(59, 130, 246, 0.15);
    color: #93c5fd;
    border: 1px solid rgba(59, 130, 246, 0.3);
  }

  /* fallback for custom groups */
  .badge-role {
    font-weight: 700;
  }

  .badge-scope {
    opacity: 0.85;
    background: rgba(0,0,0,0.25);
    padding: 1px 4px;
    border-radius: 3px;
    font-size: 0.7rem;
  }

  .remove-membership-btn {
    background: transparent;
    border: none;
    color: currentColor;
    font-weight: bold;
    cursor: pointer;
    font-size: 1rem;
    line-height: 1;
    padding: 0;
    opacity: 0.6;
    transition: opacity 0.2s;
  }

  .remove-membership-btn:hover {
    opacity: 1;
  }

  .user-profile-cell {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .user-avatar {
    width: 32px;
    height: 32px;
    border-radius: 50%;
    background: #1e293b;
    border: 1px solid rgba(148, 163, 184, 0.2);
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 700;
    color: #cbd5e1;
    overflow: hidden;
  }

  .user-avatar img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .actions-cell {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }

  .group-title-label {
    display: inline-flex;
    align-items: center;
    width: fit-content;
    font-size: 0.8rem;
    padding: 2px 6px;
    border-radius: 4px;
    margin-bottom: 4px;
  }

  .group-desc-para {
    margin: 0;
    font-size: 0.8rem;
    color: #64748b;
  }

  .permission-summary-strip {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 12px;
    margin-bottom: 18px;
  }

  .matrix-header-actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 10px;
  }

  .permission-catalog-status {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-top: 10px;
  }

  .permission-catalog-status span {
    display: inline-flex;
    align-items: center;
    border: 1px solid rgba(148, 163, 184, 0.28);
    border-radius: 999px;
    padding: 3px 8px;
    background: rgba(15, 23, 42, 0.58);
    color: #cbd5e1;
    font-size: 0.72rem;
    font-weight: 800;
  }

  .permission-catalog-status span.live {
    border-color: rgba(52, 211, 153, 0.32);
    background: rgba(16, 185, 129, 0.1);
    color: #86efac;
  }

  .permission-catalog-status small {
    color: #64748b;
    font-size: 0.72rem;
  }

  .group-coverage-card {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.46);
    background: rgba(2, 6, 23, 0.28);
    border-radius: 8px;
    padding: 12px;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 10px;
    align-items: start;
  }

  .group-coverage-card p {
    margin: 3px 0 0 0;
    color: #64748b;
    font-size: 0.76rem;
    line-height: 1.35;
  }

  .group-coverage-card strong {
    color: #f8fafc;
    font-size: 0.86rem;
  }

  .group-coverage-meta {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 3px;
    min-width: 58px;
  }

  .group-coverage-meta small {
    color: #64748b;
    font-size: 0.7rem;
    white-space: nowrap;
  }

  .coverage-bar {
    grid-column: 1 / -1;
    height: 5px;
    background: rgba(51, 65, 85, 0.56);
    border-radius: 999px;
    overflow: hidden;
  }

  .coverage-bar i {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: linear-gradient(90deg, #38bdf8, #34d399);
  }

  .group-card-actions {
    grid-column: 1 / -1;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    border-top: 1px solid rgba(51, 65, 85, 0.34);
    padding-top: 8px;
    color: #64748b;
    font-size: 0.72rem;
  }

  .group-delete-btn {
    min-height: 28px;
    border: 1px solid rgba(248, 113, 113, 0.36);
    border-radius: 6px;
    padding: 0 10px;
    background: rgba(127, 29, 29, 0.18);
    color: #fecaca;
    font-size: 0.72rem;
    font-weight: 800;
    cursor: pointer;
    transition: border-color 0.18s ease, background 0.18s ease, transform 0.18s ease;
  }

  .group-delete-btn:hover:not(:disabled) {
    border-color: rgba(248, 113, 113, 0.72);
    background: rgba(127, 29, 29, 0.3);
    transform: translateY(-1px);
  }

  .group-delete-btn:disabled {
    opacity: 0.46;
    cursor: not-allowed;
  }

  .permission-tree {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .permission-branch {
    display: grid;
    grid-template-columns: 18px minmax(0, 1fr);
    gap: 12px;
  }

  .branch-stem {
    width: 2px;
    justify-self: center;
    border-radius: 999px;
    background: linear-gradient(180deg, rgba(56, 189, 248, 0.65), rgba(52, 211, 153, 0.12));
  }

  .branch-content {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.5);
    background: rgba(15, 23, 42, 0.38);
    border-radius: 8px;
    overflow: hidden;
  }

  .branch-header {
    display: flex;
    justify-content: space-between;
    gap: 14px;
    padding: 14px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.42);
    background: rgba(2, 6, 23, 0.26);
  }

  .branch-key {
    color: #38bdf8;
    font-size: 0.68rem;
    font-weight: 900;
  }

  .branch-header h3 {
    margin: 4px 0;
    color: #f8fafc;
    font-size: 1rem;
  }

  .branch-header p {
    margin: 0;
    color: #94a3b8;
    font-size: 0.8rem;
    line-height: 1.45;
  }

  .branch-count {
    flex: none;
    color: #64748b;
    font-size: 0.72rem;
  }

  .permission-nodes {
    display: flex;
    flex-direction: column;
  }

  .permission-node {
    display: grid;
    grid-template-columns: minmax(220px, 0.86fr) minmax(280px, 1.14fr);
    gap: 14px;
    padding: 14px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.34);
  }

  .permission-node:last-child {
    border-bottom: none;
  }

  .permission-code {
    color: #7dd3fc;
    font-size: 0.72rem;
    font-weight: 800;
  }

  .permission-node-main strong {
    display: block;
    color: #e2e8f0;
    font-size: 0.92rem;
    margin-top: 4px;
  }

  .permission-node-main p {
    margin: 5px 0 0 0;
    color: #64748b;
    font-size: 0.78rem;
    line-height: 1.42;
  }

  .permission-group-toggles {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
    gap: 8px;
    align-content: start;
  }

  .permission-group-toggles button {
    min-width: 0;
    min-height: 38px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
    border-radius: 8px;
    border: 1px solid rgba(51, 65, 85, 0.58);
    background: rgba(2, 6, 23, 0.32);
    color: #64748b;
    padding: 8px 9px;
    cursor: pointer;
    transition: transform 0.16s ease, border-color 0.16s ease, background 0.16s ease, color 0.16s ease;
  }

  .permission-group-toggles button:hover:not(:disabled) {
    transform: translateY(-1px);
    border-color: rgba(56, 189, 248, 0.46);
    color: #cbd5e1;
  }

  .permission-group-toggles button.enabled {
    color: #bbf7d0;
    border-color: rgba(52, 211, 153, 0.36);
    background: rgba(6, 78, 59, 0.2);
  }

  .permission-group-toggles button.locked {
    color: #fca5a5;
    border-color: rgba(248, 113, 113, 0.3);
    background: rgba(127, 29, 29, 0.18);
    cursor: not-allowed;
  }

  .permission-group-toggles button:disabled:not(.locked) {
    opacity: 0.62;
    cursor: not-allowed;
  }

  .permission-group-toggles span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 0.76rem;
    font-weight: 800;
  }

  .permission-group-toggles small {
    flex: none;
    color: currentColor;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 0.68rem;
    font-weight: 900;
  }

  /* Audit Table Custom styles */
  .actor-tag {
    color: #e2e8f0;
    font-weight: 700;
    font-size: 0.85rem;
  }

  .actor-sub {
    font-size: 0.75rem;
    color: #64748b;
    margin-left: 4px;
  }

  .action-badge {
    font-size: 0.7rem;
    padding: 2px 6px;
    border-radius: 4px;
    border: 1px solid rgba(148, 163, 184, 0.2);
    font-weight: 700;
  }

  /* Style based on common audit log actions */
  .badge-action-user_login { background: rgba(59, 130, 246, 0.1); color: #3b82f6; }
  .badge-action-admin_transfer { background: rgba(220, 38, 38, 0.1); color: #ef4444; border-color: rgba(220, 38, 38, 0.2); }
  .badge-action-group_permissions_update { background: rgba(245, 158, 11, 0.1); color: #f59e0b; }
  .badge-action-group_member_add { background: rgba(16, 185, 129, 0.1); color: #10b981; }
  .badge-action-group_member_remove { background: rgba(100, 116, 139, 0.1); color: #64748b; }
  .badge-action-config_update { background: rgba(99, 102, 241, 0.1); color: #6366f1; }

  /* Modals Overlay */
  .modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(2, 6, 17, 0.8);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .modal-card {
    background: #0f172a;
    border: 1px solid rgba(51, 65, 85, 0.6);
    border-radius: 12px;
    width: 100%;
    max-width: 500px;
    box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.5);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .delete-group-modal {
    max-width: 520px;
    border-color: rgba(248, 113, 113, 0.32);
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.4);
    background: #0b1329;
  }

  .delete-modal-header {
    align-items: flex-start;
    background:
      linear-gradient(135deg, rgba(127, 29, 29, 0.28), rgba(15, 23, 42, 0.96)),
      #0b1329;
  }

  .modal-header h3 {
    margin: 0;
    font-size: 1.1rem;
    color: #f8fafc;
  }

  .danger-kicker {
    display: block;
    margin-bottom: 5px;
    color: #fca5a5;
    font-size: 0.68rem;
    font-weight: 900;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .delete-target-card {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 14px;
    align-items: center;
    border: 1px solid rgba(51, 65, 85, 0.48);
    border-radius: 8px;
    padding: 14px;
    background: rgba(2, 6, 23, 0.34);
  }

  .delete-target-card p,
  .delete-confirm-copy {
    margin: 6px 0 0 0;
    color: #94a3b8;
    font-size: 0.82rem;
    line-height: 1.5;
  }

  .delete-target-stats {
    min-width: 84px;
    border-left: 1px solid rgba(51, 65, 85, 0.5);
    padding-left: 14px;
    text-align: right;
  }

  .delete-target-stats strong {
    display: block;
    color: #fecaca;
    font-size: 1.25rem;
  }

  .delete-target-stats small {
    color: #64748b;
    font-size: 0.7rem;
  }

  .delete-confirm-copy {
    margin-top: 14px;
  }

  .close-modal-btn {
    background: transparent;
    border: none;
    color: #64748b;
    font-size: 1.5rem;
    cursor: pointer;
    line-height: 1;
    padding: 0;
  }

  .close-modal-btn:hover {
    color: #e2e8f0;
  }

  .modal-body {
    padding: 20px;
    overflow-y: auto;
    max-height: 60vh;
  }

  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    padding: 16px 20px;
    border-top: 1px solid rgba(51, 65, 85, 0.4);
    background: #0b1329;
  }

  /* Form controls inside modal */
  .field-item {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: 16px;
  }

  .field-item label {
    font-size: 0.8rem;
    font-weight: 700;
    color: #94a3b8;
  }

  .field-desc {
    margin: 4px 0 0 0;
    font-size: 0.75rem;
    color: #64748b;
    line-height: 1.4;
  }

  .custom-input, .custom-textarea {
    background: #1e293b;
    border: 1px solid rgba(51, 65, 85, 0.6);
    border-radius: 6px;
    color: #f1f5f9;
    padding: 10px 12px;
    font-size: 0.9rem;
    outline: none;
    transition: border-color 0.2s;
  }

  .custom-input:focus, .custom-textarea:focus {
    border-color: #6366f1;
  }

  .custom-select {
    appearance: none;
    -webkit-appearance: none;
    background-image: url("data:image/svg+xml;utf8,<svg fill='%2394a3b8' height='24' viewBox='0 0 24 24' width='24' xmlns='http://www.w3.org/2000/svg'><path d='M7 10l5 5 5-5z'/><path d='M0 0h24v24H0z' fill='none'/></svg>");
    background-repeat: no-repeat;
    background-position: right 12px center;
    padding-right: 36px;
    background-color: #1e293b;
    border: 1px solid rgba(51, 65, 85, 0.6);
    border-radius: 6px;
    color: #f1f5f9;
    padding: 10px 12px;
    font-size: 0.9rem;
    outline: none;
    transition: border-color 0.2s, box-shadow 0.2s;
  }

  .custom-select:focus {
    border-color: #6366f1;
    box-shadow: 0 0 8px rgba(99, 102, 241, 0.3);
  }

  /* Dropdown Styles for user group */
  .relative {
    position: relative;
  }

  .dropdown-trigger-btn {
    width: 100%;
    background-color: #1e293b;
    border: 1px solid rgba(51, 65, 85, 0.6);
    border-radius: 6px;
    color: #f1f5f9;
    padding: 10px 12px;
    font-size: 0.9rem;
    outline: none;
    cursor: pointer;
    text-align: left;
    display: flex;
    justify-content: space-between;
    align-items: center;
    transition: border-color 0.2s, box-shadow 0.2s;
  }

  .dropdown-trigger-btn:focus {
    border-color: #6366f1;
    box-shadow: 0 0 8px rgba(99, 102, 241, 0.3);
  }

  .dropdown-chevron {
    color: #94a3b8;
    transition: transform 0.2s ease;
  }

  .dropdown-options-list {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    width: 100%;
    max-height: 200px;
    overflow-y: auto;
    background: rgba(15, 23, 42, 0.98);
    border: 1px solid rgba(99, 102, 241, 0.4);
    border-radius: 8px;
    z-index: 1100;
    padding: 4px;
    box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5), 0 8px 10px -6px rgba(0, 0, 0, 0.5);
    box-sizing: border-box;
  }

  .dropdown-options-list::-webkit-scrollbar {
    width: 6px;
  }

  .dropdown-options-list::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.3);
    border-radius: 3px;
  }

  .dropdown-option-item {
    width: 100%;
    border: none;
    background: transparent;
    text-align: left;
    padding: 8px 12px;
    color: #cbd5e1;
    font-size: 0.8rem;
    cursor: pointer;
    border-radius: 6px;
    transition: background-color 0.15s ease, color 0.15s ease;
    box-sizing: border-box;
  }

  .dropdown-option-item:hover {
    background: rgba(99, 102, 241, 0.2);
    color: #ffffff;
  }

  .dropdown-option-item.selected {
    background: rgba(99, 102, 241, 0.4);
    color: #ffffff;
    font-weight: 600;
  }

  .dropdown-backdrop-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 1099;
    background: transparent;
    border: 0;
    padding: 0;
    cursor: default;
  }

  .input-disabled {
    background: rgba(30, 41, 59, 0.5);
    border-color: rgba(51, 65, 85, 0.3);
    color: #64748b;
    cursor: not-allowed;
    padding: 10px 12px;
    border-radius: 6px;
  }

  .radio-group {
    display: flex;
    gap: 16px;
    margin-top: 4px;
  }

  .radio-group label {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.85rem;
    color: #e2e8f0;
    cursor: pointer;
    font-weight: normal;
  }

  .radio-group input[type="radio"] {
    width: 16px;
    height: 16px;
    accent-color: #6366f1;
  }

  .error-banner, .success-banner {
    padding: 10px 12px;
    border-radius: 6px;
    font-size: 0.85rem;
    margin-bottom: 16px;
    font-weight: 500;
  }

  .error-banner {
    background: rgba(239, 68, 68, 0.1);
    color: #f87171;
    border: 1px solid rgba(239, 68, 68, 0.2);
  }

  .success-banner {
    background: rgba(16, 185, 129, 0.1);
    color: #34d399;
    border: 1px solid rgba(16, 185, 129, 0.2);
  }

  /* Phase 41 settings shell: light management workspace, scoped away from modals. */
  .phase41-settings {
    display: block;
    color: #102033;
    padding-bottom: 28px;
  }

  .settings-kicker {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: #008f96;
    font-size: 0.7rem;
    font-weight: 900;
    letter-spacing: 0;
    text-transform: uppercase;
  }

  .phase41-settings .settings-main {
    gap: 16px;
    overflow: visible;
    padding-right: 0;
  }

  .settings-breadcrumb-bar {
    border-radius: 8px;
    border: 1px solid rgba(139, 159, 181, 0.28);
    background: rgba(255, 255, 255, 0.74);
    box-shadow: 0 12px 28px rgba(25, 43, 61, 0.07);
    padding: 10px 12px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    backdrop-filter: blur(18px);
    -webkit-backdrop-filter: blur(18px);
  }

  .settings-breadcrumb-bar ol {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    color: #69798b;
    font-size: 0.82rem;
    font-weight: 800;
  }

  .settings-breadcrumb-bar li {
    min-width: 0;
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  .settings-breadcrumb-bar li + li::before {
    content: "/";
    color: #a4b0bd;
    font-weight: 700;
  }

  .settings-breadcrumb-bar button {
    border: 0;
    background: transparent;
    color: #0d1b2a;
    font: inherit;
    font-weight: 900;
    padding: 0;
    cursor: pointer;
  }

  .settings-breadcrumb-bar li.current span {
    color: #0b1724;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .breadcrumb-meta {
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    color: #7a8897;
    font-size: 0.76rem;
  }

  .settings-content-shell {
    border-radius: 8px;
    border: 1px solid rgba(139, 159, 181, 0.32);
    background: rgba(255, 255, 255, 0.76);
    box-shadow: 0 16px 34px rgba(25, 43, 61, 0.08);
    backdrop-filter: blur(18px);
    -webkit-backdrop-filter: blur(18px);
  }

  .settings-content-header {
    border-bottom: 1px solid rgba(139, 159, 181, 0.24);
    padding: 12px;
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(320px, 0.86fr);
    align-items: center;
    justify-content: space-between;
    gap: 14px;
  }

  .settings-content-header h1 {
    margin: 3px 0 0;
    color: #0d1b2a;
    font-size: 1.12rem;
    line-height: 1.15;
    letter-spacing: 0;
  }

  .settings-content-header p {
    display: none;
  }

  .settings-content-facts {
    min-width: 0;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }

  .settings-content-facts div {
    min-width: 0;
    min-height: 48px;
    border-radius: 8px;
    border: 1px solid rgba(139, 159, 181, 0.22);
    background: rgba(247, 250, 252, 0.78);
    padding: 9px 10px;
    display: grid;
    align-content: center;
    gap: 4px;
  }

  .settings-content-facts span {
    color: #7a8897;
    font-size: 0.72rem;
    font-weight: 800;
  }

  .settings-content-facts strong {
    color: #0d1b2a;
    font-size: 0.86rem;
    overflow-wrap: anywhere;
  }

  .settings-module-panel {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 16px;
  }

  .phase41-settings .section-card,
  .phase41-settings .config-audit-panel {
    border-radius: 8px;
    border: 1px solid rgba(139, 159, 181, 0.24);
    background: rgba(255, 255, 255, 0.62);
    box-shadow: none;
    color: #13243a;
  }

  .phase41-settings .card-header {
    border-bottom-color: rgba(139, 159, 181, 0.24);
  }

  .phase41-settings .card-header h2,
  .phase41-settings .config-audit-header h3,
  .phase41-settings .policy-panel-header h3 {
    color: #0d1b2a;
  }

  .phase41-settings .card-header p,
  .phase41-settings .version-detail-header p,
  .phase41-settings .decision-card p {
    color: #667789;
  }

  .phase41-settings .table-responsive,
  .phase41-settings .policy-panel,
  .phase41-settings .version-detail,
  .phase41-settings .decision-card,
  .phase41-settings .permission-branch,
  .phase41-settings .group-coverage-card {
    border-color: rgba(139, 159, 181, 0.24);
    background: rgba(255, 255, 255, 0.6);
  }

  .phase41-settings th,
  .phase41-settings td {
    color: #25384d;
    border-bottom-color: rgba(139, 159, 181, 0.18);
    background: transparent;
  }

  .phase41-settings th {
    color: #69798b;
    background: rgba(241, 246, 249, 0.78);
    letter-spacing: 0;
  }

  .phase41-settings tr:hover td {
    background: rgba(238, 247, 247, 0.62);
  }

  @media (max-width: 1100px) {
    .settings-container {
      height: auto;
      min-height: 0;
      overflow: visible;
    }

    .settings-main {
      height: auto;
      max-height: none;
      overflow: visible;
      padding-right: 0;
    }

    .section-card {
      height: auto;
      min-height: 0;
      overflow: visible;
    }

    .policy-workbench,
    .policy-workbench.refined,
    .policy-grid,
    .version-layout {
      grid-template-columns: minmax(0, 1fr);
    }

    .permission-node {
      grid-template-columns: minmax(0, 1fr);
    }
  }

  @media (max-width: 760px) {
    .section-card,
    .config-audit-panel {
      padding: 16px;
    }

    .policy-template-strip,
    .policy-builder-grid,
    .policy-form-grid,
    .permission-summary-strip,
    .pack-summary {
      grid-template-columns: minmax(0, 1fr);
    }

    .choice-row {
      grid-template-columns: minmax(0, 1fr);
    }

    .permission-branch {
      grid-template-columns: minmax(0, 1fr);
    }

    .branch-stem {
      display: none;
    }

    .branch-header,
    .config-audit-header,
    .version-detail-header,
    .flex-header {
      flex-direction: column;
      align-items: stretch;
    }

    .permission-group-toggles {
      grid-template-columns: minmax(0, 1fr);
    }

    .diff-row {
      grid-template-columns: minmax(0, 1fr);
    }

    .diff-arrow {
      text-align: left;
    }
  }

  /* Phase 41 final settings calibration: flat shell, light child config surfaces. */
  .phase41-settings {
    --settings-surface: rgba(255, 255, 255, 0.78);
    --settings-surface-strong: rgba(255, 255, 255, 0.92);
    --settings-surface-soft: rgba(247, 250, 252, 0.78);
    --settings-border: rgba(121, 139, 159, 0.18);
    --settings-border-strong: rgba(85, 106, 128, 0.3);
    max-width: var(--wa-content-max, 1720px);
    margin: 0 auto;
    padding-bottom: 16px;
  }

  .phase41-settings .settings-content-shell {
    border: 0;
    background: transparent;
    box-shadow: none;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }

  .phase41-settings .settings-content-header,
  .settings-breadcrumb-bar {
    border-radius: var(--wa-radius-md, 8px);
    border-color: var(--settings-border);
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.9), rgba(247, 250, 252, 0.72)),
      var(--settings-surface);
    box-shadow: 0 10px 24px rgba(26, 41, 58, 0.06);
  }

  .phase41-settings .settings-content-header {
    padding: 14px 16px;
  }

  .phase41-settings .settings-content-header p {
    display: block;
    max-width: 720px;
    margin: 6px 0 0;
    color: var(--wa-text-muted, #667789);
    font-size: 0.82rem;
    line-height: 1.5;
  }

  .phase41-settings .settings-module-panel {
    padding: 14px 0 0;
  }

  .phase41-settings .section-card,
  .phase41-settings .config-audit-panel {
    border-radius: var(--wa-radius-md, 8px);
    border-color: var(--settings-border);
    background: var(--settings-surface);
    box-shadow: 0 12px 28px rgba(26, 41, 58, 0.065);
  }

  .phase41-settings :global(.card-header h2),
  .phase41-settings :global(.config-overview h3),
  .phase41-settings :global(.summary-card h3),
  .phase41-settings :global(.webhook-automation-panel h3),
  .phase41-settings :global(.context-registry-panel h3),
  .phase41-settings :global(.pack-preview-panel h3) {
    color: var(--wa-text-strong, #0d1722);
    background: none;
    -webkit-text-fill-color: currentColor;
    letter-spacing: 0;
  }

  .phase41-settings :global(.card-header p),
  .phase41-settings :global(.config-overview p),
  .phase41-settings :global(.summary-card p),
  .phase41-settings :global(.field-desc),
  .phase41-settings :global(.helper-text),
  .phase41-settings :global(.text-muted) {
    color: var(--wa-text-muted, #667789);
  }

  .phase41-settings :global(.config-overview),
  .phase41-settings :global(.overview-grid > div),
  .phase41-settings :global(.summary-card),
  .phase41-settings :global(.webhook-automation-panel),
  .phase41-settings :global(.context-health-grid),
  .phase41-settings :global(.context-config-panel),
  .phase41-settings :global(.context-registry-panel),
  .phase41-settings :global(.pack-preview-panel),
  .phase41-settings :global(.form-container),
  .phase41-settings :global(.table-container),
  .phase41-settings :global(.table-responsive),
  .phase41-settings :global(.autoload-section),
  .phase41-settings :global(.project-card-item) {
    border-color: var(--settings-border) !important;
    border-radius: var(--wa-radius-md, 8px) !important;
    background: var(--settings-surface-soft) !important;
    color: var(--wa-text-main, #293847) !important;
    box-shadow: none !important;
  }

  .phase41-settings :global(table) {
    color: var(--wa-text-main, #293847);
  }

  .phase41-settings :global(th) {
    color: var(--wa-text-muted, #667789) !important;
    background: rgba(241, 246, 249, 0.92) !important;
    border-bottom-color: var(--settings-border) !important;
    letter-spacing: 0 !important;
    text-transform: none !important;
  }

  .phase41-settings :global(td) {
    color: var(--wa-text-main, #293847) !important;
    border-bottom-color: var(--settings-border) !important;
  }

  .phase41-settings :global(tr:hover td) {
    background: rgba(238, 247, 247, 0.72) !important;
  }

  .phase41-settings :global(input),
  .phase41-settings :global(textarea),
  .phase41-settings :global(select),
  .phase41-settings :global(.custom-input),
  .phase41-settings :global(.custom-textarea),
  .phase41-settings :global(.dropdown-trigger-btn),
  .phase41-settings :global(.custom-select-dropdown),
  .phase41-settings :global(.dropdown-options-list) {
    border-color: var(--settings-border) !important;
    background: rgba(255, 255, 255, 0.86) !important;
    color: var(--wa-text-strong, #0d1722) !important;
    box-shadow: none !important;
  }

  .phase41-settings :global(input:focus),
  .phase41-settings :global(textarea:focus),
  .phase41-settings :global(select:focus),
  .phase41-settings :global(.custom-input:focus),
  .phase41-settings :global(.custom-textarea:focus) {
    border-color: var(--wa-accent, #008f96) !important;
    background: #ffffff !important;
    box-shadow: 0 0 0 3px rgba(0, 143, 150, 0.1) !important;
  }

  .phase41-settings :global(.dropdown-item),
  .phase41-settings :global(.dropdown-option-item) {
    color: var(--wa-text-main, #293847) !important;
  }

  .phase41-settings :global(.dropdown-item:hover),
  .phase41-settings :global(.dropdown-option-item:hover),
  .phase41-settings :global(.dropdown-option-item.selected) {
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.12)) !important;
    color: var(--wa-accent-strong, #006f76) !important;
  }

  .phase41-settings .policy-template-strip button,
  .phase41-settings .policy-panel,
  .phase41-settings .choice-card,
  .phase41-settings .segmented-pills,
  .phase41-settings .segmented-pills button,
  .phase41-settings .suggestion-pills button,
  .phase41-settings .action-chip-grid button,
  .phase41-settings .priority-stepper strong,
  .phase41-settings .priority-stepper button,
  .phase41-settings .toggle-pill,
  .phase41-settings .decision-log-item,
  .phase41-settings .permission-node,
  .phase41-settings .delete-target-card {
    border-color: var(--settings-border);
    background: rgba(255, 255, 255, 0.62);
    color: var(--wa-text-main, #293847);
  }

  .phase41-settings .policy-template-strip span,
  .phase41-settings .choice-card strong,
  .phase41-settings .permission-code,
  .phase41-settings .priority-stepper strong,
  .phase41-settings .decision-log-item strong {
    color: var(--wa-text-strong, #0d1722);
  }

  .phase41-settings .policy-template-strip small,
  .phase41-settings .choice-card small,
  .phase41-settings .action-chip-grid small {
    color: var(--wa-text-muted, #667789);
  }

  .phase41-settings .segmented-pills button.active,
  .phase41-settings .suggestion-pills button.active,
  .phase41-settings .action-chip-grid button.active {
    border-color: rgba(0, 143, 150, 0.34);
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.12));
    color: var(--wa-accent-strong, #006f76);
  }

  .phase41-settings .toggle-pill i {
    background: rgba(102, 119, 137, 0.38);
  }

  .phase41-settings .toggle-pill.on {
    border-color: rgba(4, 150, 111, 0.24);
    background: var(--wa-success-soft, rgba(4, 150, 111, 0.12));
    color: var(--wa-success, #04966f);
  }

  .phase41-settings .toggle-pill.on i {
    background: var(--wa-success, #04966f);
  }

  @media (max-width: 920px) {
    .settings-breadcrumb-bar,
    .phase41-settings .settings-content-header {
      align-items: stretch;
      flex-direction: column;
    }

    .phase41-settings .settings-content-header {
      grid-template-columns: 1fr;
    }

    .settings-content-facts {
      grid-template-columns: 1fr;
    }
  }

  /* Phase 42 configuration center visual reset. This layer intentionally wins over legacy dark config skins. */
  .phase41-settings {
    --settings-glass: rgba(255, 255, 255, 0.66);
    --settings-glass-strong: rgba(255, 255, 255, 0.82);
    --settings-glass-soft: rgba(248, 252, 253, 0.54);
    --settings-edge: rgba(255, 255, 255, 0.76);
    --settings-line: rgba(107, 127, 146, 0.16);
    --settings-line-strong: rgba(74, 97, 118, 0.24);
    --settings-text: #243342;
    --settings-strong: #0c1724;
    --settings-muted: #607384;
    --settings-subtle: #8a9baa;
    --settings-accent: #007f86;
    --settings-accent-soft: rgba(0, 127, 134, 0.11);
    --settings-blue: #255fa8;
    --settings-blue-soft: rgba(37, 95, 168, 0.1);
    --settings-purple: #6841a8;
    --settings-purple-soft: rgba(104, 65, 168, 0.1);
    --settings-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.86), 0 18px 54px rgba(28, 47, 65, 0.11);
    width: 100%;
    max-width: min(1720px, 100%);
    color: var(--settings-text);
  }

  .phase41-settings .settings-main {
    gap: 12px;
    padding-right: 0;
    overflow: visible;
  }

  .phase41-settings .settings-content-shell {
    display: grid;
    gap: 12px;
    border: 0;
    background: transparent;
    box-shadow: none;
  }

  .phase41-settings .settings-breadcrumb-bar,
  .phase41-settings .settings-content-header,
  .phase41-settings .section-card,
  .phase41-settings .config-audit-panel {
    border: 1px solid var(--settings-edge);
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.86), rgba(246, 250, 252, 0.62)),
      var(--settings-glass);
    box-shadow: var(--settings-shadow);
    backdrop-filter: blur(22px) saturate(138%);
    -webkit-backdrop-filter: blur(22px) saturate(138%);
  }

  .phase41-settings .settings-breadcrumb-bar {
    min-height: 48px;
    padding: 8px 12px;
    border-radius: 16px;
  }

  .phase41-settings .settings-breadcrumb-bar ol {
    gap: 6px;
  }

  .phase41-settings .settings-breadcrumb-bar button,
  .phase41-settings .settings-breadcrumb-bar span,
  .phase41-settings .breadcrumb-meta {
    color: var(--settings-muted);
  }

  .phase41-settings .settings-breadcrumb-bar li.current span {
    color: var(--settings-strong);
  }

  .phase41-settings .settings-content-header {
    display: grid;
    grid-template-columns: minmax(0, 1.2fr) minmax(360px, 0.8fr);
    gap: 18px;
    align-items: end;
    min-height: 132px;
    padding: 22px 24px;
    border-radius: 22px;
  }

  .phase41-settings .settings-kicker {
    color: var(--settings-accent);
    font-size: 0.72rem;
    font-weight: 820;
    letter-spacing: 0.08em;
  }

  .phase41-settings .settings-content-header h1 {
    margin: 5px 0 0;
    color: var(--settings-strong);
    font-size: clamp(1.55rem, 1.2vw + 1.1rem, 2.25rem);
    line-height: 1.08;
    font-weight: 820;
    text-wrap: balance;
  }

  .phase41-settings .settings-content-header p {
    max-width: 760px;
    margin-top: 10px;
    color: var(--settings-muted);
    font-size: 0.9rem;
    line-height: 1.65;
  }

  .phase41-settings .settings-content-facts {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
  }

  .phase41-settings .settings-content-facts div {
    min-height: 64px;
    padding: 11px 12px;
    border: 1px solid rgba(255, 255, 255, 0.68);
    border-radius: 14px;
    background: rgba(255, 255, 255, 0.48);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.76);
  }

  .phase41-settings .settings-content-facts span {
    color: var(--settings-subtle);
    font-size: 0.7rem;
  }

  .phase41-settings .settings-content-facts strong {
    color: var(--settings-strong);
    font-size: 0.82rem;
  }

  .phase41-settings .settings-module-panel {
    padding: 0;
  }

  .phase41-settings .section-card,
  .phase41-settings .config-audit-panel {
    padding: 22px;
    border-radius: 22px;
  }

  .phase41-settings :global(.config-overview),
  .phase41-settings :global(.webhook-automation-panel),
  .phase41-settings :global(.summary-card),
  .phase41-settings :global(.form-sub-section),
  .phase41-settings :global(.form-section),
  .phase41-settings :global(.form-container),
  .phase41-settings :global(.table-container),
  .phase41-settings :global(.table-responsive),
  .phase41-settings :global(.autoload-section),
  .phase41-settings :global(.repo-mapping-section),
  .phase41-settings :global(.add-repo-form),
  .phase41-settings :global(.context-config-panel),
  .phase41-settings :global(.context-registry-panel),
  .phase41-settings :global(.pack-preview-panel),
  .phase41-settings :global(.context-health-grid),
  .phase41-settings :global(.overview-row),
  .phase41-settings :global(.registry-fact-card),
  .phase41-settings :global(.project-card-item),
  .phase41-settings :global(.summary-repo-item),
  .phase41-settings :global(.webhook-result-table) {
    border-color: var(--settings-line) !important;
    border-radius: 16px !important;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.58), rgba(248, 252, 253, 0.42)),
      var(--settings-glass-soft) !important;
    color: var(--settings-text) !important;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72) !important;
    backdrop-filter: blur(16px) saturate(126%) !important;
    -webkit-backdrop-filter: blur(16px) saturate(126%) !important;
  }

  .phase41-settings :global(.overview-grid),
  .phase41-settings :global(.context-grid),
  .phase41-settings :global(.registry-form-grid),
  .phase41-settings :global(.add-repo-inputs) {
    gap: 10px !important;
  }

  .phase41-settings :global(.card-header),
  .phase41-settings :global(.overview-header),
  .phase41-settings :global(.context-header),
  .phase41-settings :global(.webhook-auto-header),
  .phase41-settings .config-audit-header {
    gap: 8px;
    margin-bottom: 16px;
  }

  .phase41-settings :global(.card-header h2),
  .phase41-settings :global(.overview-header h4),
  .phase41-settings :global(.context-header h4),
  .phase41-settings :global(.webhook-auto-header h5),
  .phase41-settings :global(.config-overview h3),
  .phase41-settings :global(.summary-card h3),
  .phase41-settings :global(.context-registry-panel h3),
  .phase41-settings :global(.pack-preview-panel h3),
  .phase41-settings .config-audit-header h3 {
    color: var(--settings-strong) !important;
    background: none !important;
    -webkit-text-fill-color: currentColor !important;
    font-weight: 780 !important;
    letter-spacing: 0 !important;
    text-transform: none !important;
  }

  .phase41-settings :global(.card-header p),
  .phase41-settings :global(.overview-header p),
  .phase41-settings :global(.context-intro),
  .phase41-settings :global(.webhook-auto-header p),
  .phase41-settings :global(.webhook-auto-hint),
  .phase41-settings :global(.helper-text),
  .phase41-settings :global(.field-desc),
  .phase41-settings :global(.summary-label),
  .phase41-settings :global(.overview-row span),
  .phase41-settings :global(.form-label-custom),
  .phase41-settings :global(.section-subtitle),
  .phase41-settings :global(.registry-section-title),
  .phase41-settings :global(.text-muted) {
    color: var(--settings-muted) !important;
  }

  .phase41-settings :global(.summary-value),
  .phase41-settings :global(.overview-row strong),
  .phase41-settings :global(.summary-row strong),
  .phase41-settings :global(.webhook-url-box),
  .phase41-settings :global(.repo-name-cell),
  .phase41-settings :global(.registry-fact-card strong),
  .phase41-settings :global(.pack-summary strong),
  .phase41-settings :global(.font-mono) {
    color: var(--settings-strong) !important;
  }

  .phase41-settings :global(table) {
    overflow: hidden;
    width: 100%;
    border-collapse: separate !important;
    border-spacing: 0 !important;
    color: var(--settings-text) !important;
    font-variant-numeric: tabular-nums;
  }

  .phase41-settings :global(th) {
    height: 42px;
    padding: 10px 14px !important;
    border-bottom: 1px solid var(--settings-line) !important;
    background: rgba(243, 248, 250, 0.78) !important;
    color: var(--settings-muted) !important;
    font-size: 0.76rem !important;
    font-weight: 740 !important;
    letter-spacing: 0 !important;
    text-transform: none !important;
  }

  .phase41-settings :global(td) {
    min-height: 48px;
    padding: 12px 14px !important;
    border-bottom: 1px solid rgba(107, 127, 146, 0.12) !important;
    color: var(--settings-text) !important;
    background: transparent !important;
  }

  .phase41-settings :global(tbody tr:hover td) {
    background: rgba(234, 246, 247, 0.52) !important;
  }

  .phase41-settings :global(input),
  .phase41-settings :global(textarea),
  .phase41-settings :global(select),
  .phase41-settings :global(.text-input),
  .phase41-settings :global(.select-trigger),
  .phase41-settings :global(.select-inline-input),
  .phase41-settings :global(.context-input),
  .phase41-settings :global(.context-textarea),
  .phase41-settings :global(.custom-input),
  .phase41-settings :global(.custom-textarea),
  .phase41-settings :global(.dropdown-trigger-btn) {
    border-color: var(--settings-line-strong) !important;
    background: rgba(255, 255, 255, 0.74) !important;
    color: var(--settings-strong) !important;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7) !important;
  }

  .phase41-settings :global(input:focus),
  .phase41-settings :global(textarea:focus),
  .phase41-settings :global(select:focus),
  .phase41-settings :global(.text-input:focus),
  .phase41-settings :global(.select-trigger.is-active),
  .phase41-settings :global(.context-input:focus),
  .phase41-settings :global(.context-textarea:focus),
  .phase41-settings :global(.custom-input:focus),
  .phase41-settings :global(.custom-textarea:focus) {
    border-color: rgba(0, 127, 134, 0.72) !important;
    background: rgba(255, 255, 255, 0.94) !important;
    box-shadow: 0 0 0 3px rgba(0, 127, 134, 0.11), inset 0 1px 0 rgba(255, 255, 255, 0.8) !important;
  }

  .phase41-settings :global(.custom-select-dropdown),
  .phase41-settings :global(.dropdown-options-list),
  .phase41-settings :global(.select-menu) {
    border: 1px solid var(--settings-edge) !important;
    border-radius: 16px !important;
    background: rgba(255, 255, 255, 0.92) !important;
    box-shadow: 0 18px 48px rgba(28, 47, 65, 0.16) !important;
    backdrop-filter: blur(22px) saturate(132%) !important;
    -webkit-backdrop-filter: blur(22px) saturate(132%) !important;
  }

  .phase41-settings :global(.dropdown-item),
  .phase41-settings :global(.dropdown-option-item),
  .phase41-settings :global(.select-option),
  .phase41-settings :global(.context-choice-grid button) {
    color: var(--settings-text) !important;
  }

  .phase41-settings :global(.dropdown-item:hover),
  .phase41-settings :global(.dropdown-option-item:hover),
  .phase41-settings :global(.dropdown-option-item.selected),
  .phase41-settings :global(.select-option:hover),
  .phase41-settings :global(.select-option.selected),
  .phase41-settings :global(.context-choice-grid button:hover),
  .phase41-settings :global(.context-choice-grid button.active) {
    border-color: rgba(0, 127, 134, 0.22) !important;
    background: var(--settings-accent-soft) !important;
    color: var(--settings-accent) !important;
  }

  .phase41-settings :global(.badge),
  .phase41-settings :global(.context-chip),
  .phase41-settings :global(.webhook-status-pill),
  .phase41-settings :global(.phase-badge),
  .phase41-settings :global(.priority-badge),
  .phase41-settings .wa-admin-pill {
    border-radius: 999px !important;
    box-shadow: none !important;
    font-weight: 760 !important;
    letter-spacing: 0 !important;
  }

  .phase41-settings :global(.webhook-status-pill.ok),
  .phase41-settings :global(.webhook-status-pill.created),
  .phase41-settings :global(.webhook-status-pill.updated) {
    border-color: rgba(4, 150, 111, 0.2) !important;
    background: rgba(4, 150, 111, 0.1) !important;
    color: #047857 !important;
  }

  .phase41-settings :global(.webhook-status-pill.drift),
  .phase41-settings :global(.webhook-status-pill.missing) {
    border-color: rgba(216, 135, 0, 0.24) !important;
    background: rgba(216, 135, 0, 0.12) !important;
    color: #9a5b00 !important;
  }

  .phase41-settings :global(.webhook-status-pill.error),
  .phase41-settings :global(.text-warning) {
    color: #b42318 !important;
  }

  .phase41-settings :global(.btn) {
    border-radius: 10px !important;
    transition:
      transform 160ms var(--wa-ease, ease),
      border-color 160ms var(--wa-ease, ease),
      background 160ms var(--wa-ease, ease),
      box-shadow 160ms var(--wa-ease, ease) !important;
  }

  .phase41-settings :global(.btn:hover:not(:disabled)) {
    transform: translateY(-1px);
  }

  .phase41-settings :global(.btn:active:not(:disabled)) {
    transform: translateY(0);
  }

  .phase41-settings :global(.btn-secondary),
  .phase41-settings :global(.btn-ghost) {
    border-color: var(--settings-line) !important;
    background: rgba(255, 255, 255, 0.56) !important;
    color: var(--settings-text) !important;
  }

  .phase41-settings :global(.btn-secondary:hover:not(:disabled)),
  .phase41-settings :global(.btn-ghost:hover:not(:disabled)) {
    border-color: var(--settings-line-strong) !important;
    background: rgba(255, 255, 255, 0.86) !important;
    color: var(--settings-strong) !important;
  }

  .phase41-settings .policy-template-strip button,
  .phase41-settings .policy-panel,
  .phase41-settings .choice-card,
  .phase41-settings .segmented-pills,
  .phase41-settings .segmented-pills button,
  .phase41-settings .suggestion-pills button,
  .phase41-settings .action-chip-grid button,
  .phase41-settings .priority-stepper strong,
  .phase41-settings .priority-stepper button,
  .phase41-settings .toggle-pill,
  .phase41-settings .decision-log-item,
  .phase41-settings .permission-node,
  .phase41-settings .delete-target-card,
  .phase41-settings .group-coverage-card,
  .phase41-settings .decision-card {
    border-color: var(--settings-line) !important;
    background: rgba(255, 255, 255, 0.52) !important;
    color: var(--settings-text) !important;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7) !important;
  }

  .phase41-settings .policy-template-strip button:hover,
  .phase41-settings .choice-card:hover,
  .phase41-settings .suggestion-pills button:hover,
  .phase41-settings .action-chip-grid button:hover,
  .phase41-settings .permission-node:hover,
  .phase41-settings .group-coverage-card:hover {
    border-color: rgba(0, 127, 134, 0.22) !important;
    background: rgba(255, 255, 255, 0.78) !important;
    transform: translateY(-1px);
  }

  .phase41-settings .segmented-pills button.active,
  .phase41-settings .suggestion-pills button.active,
  .phase41-settings .action-chip-grid button.active,
  .phase41-settings .choice-card.active {
    border-color: rgba(0, 127, 134, 0.28) !important;
    background: var(--settings-accent-soft) !important;
    color: var(--settings-accent) !important;
  }

  .phase41-settings .toggle-pill.on {
    border-color: rgba(4, 150, 111, 0.22) !important;
    background: rgba(4, 150, 111, 0.1) !important;
    color: #047857 !important;
  }

  .phase41-settings .config-audit-panel {
    margin-top: 12px;
  }

  .phase41-settings .empty-version-state,
  .phase41-settings .diff-empty,
  .phase41-settings .rollback-note {
    border: 1px solid rgba(107, 127, 146, 0.14) !important;
    border-radius: 14px !important;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.64), rgba(248, 252, 253, 0.5)),
      rgba(255, 255, 255, 0.48) !important;
    color: var(--settings-muted) !important;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7);
    backdrop-filter: blur(12px) saturate(124%);
    -webkit-backdrop-filter: blur(12px) saturate(124%);
  }

  .phase41-settings .config-version-error {
    border: 1px solid rgba(221, 75, 62, 0.18) !important;
    border-radius: 14px !important;
    background:
      linear-gradient(90deg, rgba(221, 75, 62, 0.12), rgba(255, 255, 255, 0.66)),
      rgba(255, 255, 255, 0.54) !important;
    color: var(--settings-danger) !important;
  }

  .phase41-settings .diff-table,
  .phase41-settings :global(.summary-repos),
  .phase41-settings :global(.project-cards-container),
  .phase41-settings :global(.table-container),
  .phase41-settings :global(.webhook-result-table) {
    scrollbar-color: rgba(0, 127, 134, 0.38) rgba(107, 127, 146, 0.1) !important;
  }

  @media (max-width: 1120px) {
    .phase41-settings .settings-content-header {
      grid-template-columns: 1fr;
      align-items: start;
    }

    .phase41-settings .settings-content-facts {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (max-width: 720px) {
    .phase41-settings .settings-content-header,
    .phase41-settings .section-card,
    .phase41-settings .config-audit-panel {
      padding: 16px;
      border-radius: 18px;
    }

    .phase41-settings .settings-content-facts {
      grid-template-columns: 1fr;
    }
  }

  /* Phase 43 configuration workbench rebuild. */
  .phase41-settings {
    --settings-radius-panel: 18px;
    --settings-radius-control: 12px;
  }

  .phase41-settings .settings-content-shell {
    gap: 10px;
  }

  .phase41-settings .settings-content-header {
    grid-template-columns: minmax(0, 1fr) minmax(360px, 0.74fr);
    align-items: center;
    min-height: 96px;
    padding: 18px 20px;
    border-radius: var(--settings-radius-panel);
  }

  .phase41-settings .settings-content-header h1 {
    font-size: clamp(1.28rem, 0.72vw + 1rem, 1.72rem);
  }

  .phase41-settings .settings-content-header p {
    max-width: 600px;
    margin-top: 6px;
    font-size: 0.84rem;
    line-height: 1.5;
  }

  .phase41-settings .settings-content-facts {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .phase41-settings .settings-content-facts div {
    min-height: 52px;
    padding: 9px 10px;
    border-radius: 12px;
    background: rgba(255, 255, 255, 0.5);
  }

  .phase41-settings .settings-module-panel {
    padding: 0;
  }

  .phase41-settings .settings-workbench-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(320px, 390px);
    gap: 12px;
    align-items: start;
  }

  .phase41-settings .settings-workbench-grid.without-audit {
    grid-template-columns: minmax(0, 1fr);
  }

  .phase41-settings .settings-primary-pane,
  .phase41-settings .settings-audit-pane {
    min-width: 0;
  }

  .phase41-settings .settings-audit-pane {
    position: sticky;
    top: 12px;
  }

  .phase41-settings .section-card,
  .phase41-settings .config-audit-panel {
    padding: 18px;
    border-radius: var(--settings-radius-panel);
  }

  .phase41-settings .config-audit-panel {
    margin-top: 0;
  }

  .phase41-settings :global(.config-overview) {
    display: grid !important;
    gap: 16px !important;
    padding: 0 !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
    backdrop-filter: none !important;
    -webkit-backdrop-filter: none !important;
  }

  .phase41-settings :global(.overview-header) {
    display: grid !important;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center !important;
    gap: 14px !important;
    margin-bottom: 0 !important;
    padding-bottom: 14px !important;
    border-bottom: 1px solid var(--settings-line) !important;
  }

  .phase41-settings :global(.overview-kicker),
  .phase41-settings .audit-kicker {
    color: var(--settings-accent) !important;
    font-size: 0.68rem !important;
    font-weight: 860 !important;
    letter-spacing: 0.08em !important;
    text-transform: uppercase !important;
  }

  .phase41-settings :global(.overview-header h4) {
    margin-top: 4px !important;
    font-size: 1.08rem !important;
    line-height: 1.2 !important;
  }

  .phase41-settings :global(.overview-grid) {
    display: grid !important;
    grid-template-columns: repeat(3, minmax(0, 1fr)) !important;
    gap: 10px !important;
    margin-top: 0 !important;
    padding: 0 !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase41-settings :global(.overview-row) {
    display: grid !important;
    align-content: start !important;
    gap: 8px !important;
    min-height: 96px !important;
    padding: 13px 14px !important;
    border: 1px solid rgba(107, 127, 146, 0.14) !important;
    border-radius: 14px !important;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.68), rgba(248, 252, 253, 0.48)),
      rgba(255, 255, 255, 0.52) !important;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.78) !important;
  }

  .phase41-settings :global(.overview-row span) {
    font-size: 0.72rem !important;
    font-weight: 760 !important;
    line-height: 1.25 !important;
  }

  .phase41-settings :global(.overview-row strong) {
    display: block !important;
    max-height: 72px;
    overflow: auto;
    color: var(--settings-strong) !important;
    font-size: 0.84rem !important;
    font-weight: 780 !important;
    line-height: 1.45 !important;
    text-align: left !important;
    overflow-wrap: anywhere;
    scrollbar-width: thin;
  }

  .phase41-settings :global(.overview-row:has(strong.text-success)),
  .phase41-settings :global(.overview-row:last-child) {
    border-color: rgba(4, 150, 111, 0.18) !important;
    background:
      linear-gradient(180deg, rgba(242, 251, 248, 0.78), rgba(255, 255, 255, 0.48)),
      rgba(255, 255, 255, 0.52) !important;
  }

  .phase41-settings :global(.overview-actions) {
    display: flex !important;
    justify-content: flex-end !important;
    gap: 10px !important;
    margin-top: 0 !important;
    padding-top: 14px !important;
    border-top: 1px solid var(--settings-line) !important;
  }

  .phase41-settings .config-audit-header {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    align-items: start;
    gap: 10px;
    margin-bottom: 12px;
  }

  .phase41-settings .config-audit-header :global(.btn),
  .phase41-settings .config-audit-header button {
    justify-self: start;
  }

  .phase41-settings .settings-audit-pane .version-layout {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 10px;
  }

  .phase41-settings .settings-audit-pane .version-list {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
    max-height: 190px;
    padding-right: 2px;
    overflow: auto;
  }

  .phase41-settings .settings-audit-pane .version-item {
    min-height: 72px;
    padding: 10px;
    border-radius: 12px;
  }

  .phase41-settings .settings-audit-pane .version-detail {
    min-width: 0;
    padding: 12px;
    border-radius: 14px;
  }

  .phase41-settings .settings-audit-pane .version-detail-header {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 10px;
  }

  .phase41-settings .settings-audit-pane .diff-table {
    display: grid;
    gap: 8px;
    max-height: 240px;
    overflow: auto;
  }

  .phase41-settings .settings-audit-pane .diff-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 6px;
    padding: 9px;
    border: 1px solid rgba(107, 127, 146, 0.12);
    border-radius: 12px;
    background: rgba(255, 255, 255, 0.48);
  }

  .phase41-settings .settings-audit-pane .diff-arrow {
    display: none;
  }

  .phase41-settings .settings-audit-pane .diff-value {
    max-height: 68px;
    overflow: auto;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  @media (max-width: 1360px) {
    .phase41-settings .settings-workbench-grid {
      grid-template-columns: minmax(0, 1fr);
    }

    .phase41-settings .settings-audit-pane {
      position: static;
    }

    .phase41-settings .settings-audit-pane .version-layout {
      grid-template-columns: minmax(240px, 0.42fr) minmax(0, 0.58fr);
    }
  }

  @media (max-width: 1180px) {
    .phase41-settings .settings-content-header,
    .phase41-settings .settings-content-facts {
      grid-template-columns: 1fr;
    }

    .phase41-settings :global(.overview-grid) {
      grid-template-columns: repeat(2, minmax(0, 1fr)) !important;
    }
  }

  @media (max-width: 720px) {
    .phase41-settings :global(.overview-grid),
    .phase41-settings .settings-audit-pane .version-layout,
    .phase41-settings .settings-audit-pane .version-list {
      grid-template-columns: 1fr !important;
    }

    .phase41-settings :global(.overview-header),
    .phase41-settings :global(.overview-actions) {
      grid-template-columns: 1fr;
      justify-content: stretch !important;
    }
  }

  /* Phase 44 white-glass correction: keep hierarchy without gray masking. */
  .phase41-settings {
    --settings-glass: rgba(255, 255, 255, 0.9);
    --settings-glass-strong: rgba(255, 255, 255, 0.98);
    --settings-glass-soft: rgba(255, 255, 255, 0.88);
    --settings-edge: rgba(222, 234, 239, 0.86);
    --settings-line: rgba(212, 226, 233, 0.78);
    --settings-line-strong: rgba(184, 205, 216, 0.86);
    --settings-muted: #596d7d;
    --settings-subtle: #8294a1;
    --settings-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.96), 0 16px 34px rgba(42, 63, 82, 0.055);
    min-height: calc(100dvh - var(--wa-workspace-topbar-h, 68px));
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(252, 254, 254, 0.96)),
      #ffffff;
  }

  .phase41-settings .settings-main,
  .phase41-settings .settings-content-shell,
  .phase41-settings .settings-module-panel,
  .phase41-settings .settings-workbench-grid {
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase41-settings .settings-breadcrumb-bar,
  .phase41-settings .settings-content-header,
  .phase41-settings .section-card,
  .phase41-settings .config-audit-panel {
    border-color: var(--settings-edge) !important;
    background: rgba(255, 255, 255, 0.94) !important;
    box-shadow: var(--settings-shadow) !important;
  }

  .phase41-settings .settings-content-header {
    grid-template-columns: minmax(0, 1fr) minmax(360px, 0.62fr);
    min-height: 118px;
    padding: 20px 22px;
  }

  .phase41-settings .settings-content-facts div {
    border-color: rgba(214, 228, 235, 0.76) !important;
    background: rgba(255, 255, 255, 0.92) !important;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.88) !important;
  }

  .phase41-settings :global(.config-overview),
  .phase41-settings :global(.overview-grid),
  .phase41-settings :global(.form-container),
  .phase41-settings :global(.table-container),
  .phase41-settings :global(.table-responsive),
  .phase41-settings :global(.autoload-section),
  .phase41-settings :global(.repo-mapping-section),
  .phase41-settings :global(.add-repo-form),
  .phase41-settings :global(.summary-card),
  .phase41-settings :global(.webhook-automation-panel),
  .phase41-settings :global(.context-config-panel),
  .phase41-settings :global(.context-registry-panel),
  .phase41-settings :global(.pack-preview-panel),
  .phase41-settings :global(.context-health-grid) {
    background: transparent !important;
    box-shadow: none !important;
    backdrop-filter: none !important;
    -webkit-backdrop-filter: none !important;
  }

  .phase41-settings :global(.overview-row),
  .phase41-settings :global(.registry-fact-card),
  .phase41-settings :global(.project-card-item),
  .phase41-settings :global(.summary-repo-item),
  .phase41-settings :global(.webhook-result-table) {
    border-color: rgba(205, 220, 228, 0.84) !important;
    background: rgba(255, 255, 255, 0.94) !important;
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.9),
      0 8px 18px rgba(42, 63, 82, 0.035) !important;
    backdrop-filter: blur(18px) saturate(128%) !important;
    -webkit-backdrop-filter: blur(18px) saturate(128%) !important;
  }

  .phase41-settings :global(.overview-row:hover),
  .phase41-settings :global(.registry-fact-card:hover),
  .phase41-settings :global(.project-card-item:hover),
  .phase41-settings :global(.summary-repo-item:hover) {
    border-color: rgba(0, 127, 134, 0.22) !important;
    background: #ffffff !important;
    transform: translateY(-1px);
  }

  .phase41-settings :global(.overview-row:has(strong.text-success)),
  .phase41-settings :global(.overview-row:last-child) {
    border-color: rgba(0, 127, 134, 0.24) !important;
    background: rgba(252, 255, 254, 0.98) !important;
  }

  .phase41-settings :global(th) {
    background: rgba(255, 255, 255, 0.78) !important;
  }

  .phase41-settings :global(tbody tr:hover td) {
    background: rgba(241, 251, 251, 0.62) !important;
  }

  .phase41-settings .settings-audit-pane .version-detail,
  .phase41-settings .settings-audit-pane .diff-row {
    border-color: rgba(205, 220, 228, 0.8) !important;
    background: rgba(255, 255, 255, 0.94) !important;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.86) !important;
  }

  .phase41-settings .settings-audit-pane .version-item {
    background: rgba(255, 255, 255, 0.92);
  }

  .phase41-settings .settings-audit-pane .version-item.active {
    border-color: rgba(0, 127, 134, 0.24);
    background: rgba(235, 250, 250, 0.9);
    color: var(--settings-strong);
  }

  :global(.phase41-settings .settings-workbench-grid:has(.config-overview)) {
    align-items: stretch;
  }

  :global(.phase41-settings .settings-workbench-grid:has(.config-overview) .settings-audit-pane) {
    display: flex;
    align-self: stretch;
  }

  :global(.phase41-settings .settings-workbench-grid:has(.config-overview) .config-audit-panel) {
    width: 100%;
    min-height: 100%;
    display: flex;
    flex-direction: column;
  }

  :global(.phase41-settings .settings-workbench-grid:has(.project-config-container)) {
    align-items: stretch;
  }

  :global(.phase41-settings .settings-workbench-grid:has(.project-config-container) .settings-audit-pane) {
    display: flex;
    align-self: stretch;
  }

  :global(.phase41-settings .settings-workbench-grid:has(.project-config-container) .config-audit-panel) {
    width: 100%;
    min-height: 100%;
    display: flex;
    flex-direction: column;
  }

  /* Phase 45 configuration design convergence: one light glass system for every nested config panel. */
  .phase41-settings {
    --config-ink: #0d1722;
    --config-text: #293847;
    --config-muted: #667789;
    --config-subtle: #8a99aa;
    --config-line: rgba(203, 219, 227, 0.78);
    --config-line-strong: rgba(173, 195, 207, 0.82);
    --config-panel: rgba(255, 255, 255, 0.92);
    --config-card: rgba(255, 255, 255, 0.86);
    --config-soft: rgba(247, 252, 253, 0.82);
    --config-accent: #008f96;
    --config-accent-strong: #006f76;
    --config-accent-soft: rgba(0, 143, 150, 0.1);
    --config-success: #04966f;
    --config-warning: #a56500;
    --config-danger: #c94035;
    --config-info: #256bd8;
    --config-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.9), 0 14px 34px rgba(31, 55, 72, 0.055);
    --config-radius: 12px;
    --config-radius-lg: 16px;
    color: var(--config-text);
  }

  .phase41-settings,
  .phase41-settings * {
    letter-spacing: 0 !important;
  }

  .phase41-settings .settings-main::-webkit-scrollbar-track,
  .phase41-settings :global(.version-list::-webkit-scrollbar-track),
  .phase41-settings :global(.diff-table::-webkit-scrollbar-track),
  .phase41-settings :global(.diff-value::-webkit-scrollbar-track) {
    background: rgba(121, 139, 159, 0.1) !important;
  }

  .phase41-settings .settings-main::-webkit-scrollbar-thumb,
  .phase41-settings :global(.version-list::-webkit-scrollbar-thumb),
  .phase41-settings :global(.diff-table::-webkit-scrollbar-thumb),
  .phase41-settings :global(.diff-value::-webkit-scrollbar-thumb) {
    background: rgba(0, 143, 150, 0.38) !important;
  }

  .phase41-settings .section-card,
  .phase41-settings .config-audit-panel,
  .phase41-settings .policy-panel,
  .phase41-settings .branch-content,
  .phase41-settings .modal-card,
  .phase41-settings :global(.wizard),
  .phase41-settings :global(.project-config-container) {
    color: var(--config-text) !important;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(250, 253, 254, 0.88)),
      var(--config-panel) !important;
    border-color: var(--config-line) !important;
    box-shadow: var(--config-shadow) !important;
  }

  .phase41-settings .section-card,
  .phase41-settings .config-audit-panel {
    border-radius: var(--config-radius-lg) !important;
    padding: 18px !important;
  }

  .phase41-settings :global(.wizard),
  .phase41-settings :global(.project-config-container) {
    background: transparent !important;
    border: 0 !important;
    box-shadow: none !important;
  }

  .phase41-settings :global(.step-content),
  .phase41-settings :global(.form-section),
  .phase41-settings :global(.form-container),
  .phase41-settings :global(.registry-form-grid),
  .phase41-settings :global(.context-grid),
  .phase41-settings :global(.policy-builder-grid),
  .phase41-settings .policy-form-grid {
    gap: 14px !important;
  }

  .phase41-settings :global(.config-overview),
  .phase41-settings :global(.step-content),
  .phase41-settings :global(.success-screen),
  .phase41-settings :global(.form-container) {
    opacity: 1 !important;
    transform: none !important;
    animation: none !important;
  }

  .phase41-settings :global(.info-block),
  .phase41-settings :global(.credential-collapsed),
  .phase41-settings :global(.webhook-display),
  .phase41-settings :global(.webhook-automation-panel),
  .phase41-settings :global(.autoload-section),
  .phase41-settings :global(.repo-mapping-section),
  .phase41-settings :global(.add-repo-form),
  .phase41-settings :global(.summary-card),
  .phase41-settings :global(.bot-notice-box),
  .phase41-settings :global(.context-health-grid > *),
  .phase41-settings :global(.context-config-panel),
  .phase41-settings :global(.context-registry-panel),
  .phase41-settings :global(.pack-preview-panel),
  .phase41-settings :global(.registry-list),
  .phase41-settings :global(.registry-empty),
  .phase41-settings :global(.registry-fact-card),
  .phase41-settings :global(.pack-summary > *),
  .phase41-settings :global(.pack-item),
  .phase41-settings :global(.project-card-item),
  .phase41-settings :global(.summary-repo-item),
  .phase41-settings :global(.table-container),
  .phase41-settings :global(.table-responsive),
  .phase41-settings .policy-template-strip button,
  .phase41-settings .policy-panel,
  .phase41-settings .choice-card,
  .phase41-settings .segmented-pills,
  .phase41-settings .suggestion-pills button,
  .phase41-settings .action-chip-grid button,
  .phase41-settings .priority-stepper strong,
  .phase41-settings .priority-stepper button,
  .phase41-settings .toggle-pill,
  .phase41-settings .decision-card,
  .phase41-settings .decision-log-item,
  .phase41-settings .group-coverage-card,
  .phase41-settings .branch-content,
  .phase41-settings .branch-header,
  .phase41-settings .permission-node,
  .phase41-settings .permission-group-toggles button,
  .phase41-settings .delete-target-card,
  .phase41-settings .version-item,
  .phase41-settings .version-detail,
  .phase41-settings .diff-row,
  .phase41-settings .empty-version-state,
  .phase41-settings .diff-empty {
    border: 1px solid var(--config-line) !important;
    border-radius: var(--config-radius) !important;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.88), rgba(249, 253, 254, 0.76)),
      var(--config-card) !important;
    color: var(--config-text) !important;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.78) !important;
    backdrop-filter: blur(14px) saturate(120%) !important;
    -webkit-backdrop-filter: blur(14px) saturate(120%) !important;
  }

  .phase41-settings :global(.info-block) {
    border: 1px solid rgba(0, 143, 150, 0.22) !important;
  }

  .phase41-settings :global(.form-sub-section),
  .phase41-settings :global(.column-mapping-grid),
  .phase41-settings :global(.add-repo-inputs),
  .phase41-settings :global(.context-choice-grid),
  .phase41-settings .choice-row,
  .phase41-settings .permission-summary-strip {
    gap: 12px !important;
  }

  .phase41-settings :global(.form-sub-section > .input-group),
  .phase41-settings :global(.form-group),
  .phase41-settings :global(.form-group-custom),
  .phase41-settings :global(.context-field),
  .phase41-settings :global(.flex-1),
  .phase41-settings :global(.custom-select-wrapper) {
    min-width: 0;
    padding: 14px !important;
    border: 1px solid var(--config-line) !important;
    border-radius: var(--config-radius) !important;
    background: var(--config-soft) !important;
    box-shadow: none !important;
  }

  .phase41-settings :global(.overview-header),
  .phase41-settings .config-audit-header,
  .phase41-settings .policy-panel-header,
  .phase41-settings .card-header,
  .phase41-settings .modal-header,
  .phase41-settings .modal-footer,
  .phase41-settings .group-card-actions,
  .phase41-settings .branch-header,
  .phase41-settings :global(.registry-header),
  .phase41-settings :global(.registry-toolbar),
  .phase41-settings :global(.webhook-auto-header) {
    border-color: var(--config-line) !important;
    background: transparent !important;
    color: var(--config-text) !important;
  }

  .phase41-settings :global(.overview-header p) {
    color: #536a7d !important;
  }

  .phase41-settings :global(.overview-kicker),
  .phase41-settings .audit-kicker,
  .phase41-settings :global(.context-kicker),
  .phase41-settings :global(.branch-key),
  .phase41-settings :global(.permission-code),
  .phase41-settings :global(.diff-path),
  .phase41-settings :global(.proj-id) {
    color: var(--config-accent) !important;
    text-transform: none !important;
    font-weight: 820 !important;
  }

  .phase41-settings :global(h1),
  .phase41-settings :global(h2),
  .phase41-settings :global(h3),
  .phase41-settings :global(h4),
  .phase41-settings :global(h5),
  .phase41-settings :global(strong),
  .phase41-settings .version-title,
  .phase41-settings .actor-tag,
  .phase41-settings :global(.overview-header h4),
  .phase41-settings :global(.info-block h4),
  .phase41-settings :global(.success-title),
  .phase41-settings :global(.summary-value),
  .phase41-settings :global(.repo-name-cell),
  .phase41-settings :global(.proj-name),
  .phase41-settings :global(.registry-section-title),
  .phase41-settings :global(.permission-node-main strong),
  .phase41-settings :global(.branch-header h3),
  .phase41-settings :global(.choice-card strong),
  .phase41-settings :global(.decision-card strong),
  .phase41-settings :global(.decision-log-item strong),
  .phase41-settings :global(.group-coverage-card strong) {
    color: var(--config-ink) !important;
    background: none !important;
    -webkit-text-fill-color: currentColor !important;
    text-shadow: none !important;
  }

  .phase41-settings :global(p),
  .phase41-settings :global(small),
  .phase41-settings .version-meta,
  .phase41-settings .version-sections,
  .phase41-settings .branch-count,
  .phase41-settings .actor-sub,
  .phase41-settings .field-desc,
  .phase41-settings .empty-table-cell,
  .phase41-settings :global(.overview-header p),
  .phase41-settings :global(.overview-row span),
  .phase41-settings :global(.helper-text),
  .phase41-settings :global(.helper-text-custom),
  .phase41-settings :global(.summary-label),
  .phase41-settings :global(.success-desc),
  .phase41-settings :global(.webhook-help),
  .phase41-settings :global(.repo-path-cell),
  .phase41-settings :global(.proj-path),
  .phase41-settings :global(.empty-repos),
  .phase41-settings :global(.empty-projects-msg),
  .phase41-settings :global(.context-intro),
  .phase41-settings :global(.fact-meta),
  .phase41-settings :global(.permission-node-main p),
  .phase41-settings :global(.group-coverage-card p),
  .phase41-settings :global(.group-coverage-meta small),
  .phase41-settings :global(.pack-item p) {
    color: var(--config-muted) !important;
  }

  .phase41-settings :global(.overview-row) {
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(249, 253, 254, 0.92)),
      #ffffff !important;
    border-color: rgba(196, 214, 224, 0.92) !important;
  }

  .phase41-settings :global(.overview-row span) {
    color: #536a7d !important;
  }

  .phase41-settings :global(.overview-row strong) {
    color: var(--config-ink) !important;
    font-weight: 820 !important;
  }

  .phase41-settings :global(.switch-label),
  .phase41-settings :global(.form-label-custom),
  .phase41-settings :global(label) {
    color: var(--config-ink) !important;
    font-weight: 760 !important;
  }

  .phase41-settings :global(.helper-text-custom),
  .phase41-settings :global(.helper-text) {
    color: #536a7d !important;
    line-height: 1.55 !important;
  }

  .phase41-settings :global(.text-success),
  .phase41-settings .tone-success,
  .phase41-settings .permission-catalog-status span.live {
    color: var(--config-success) !important;
  }

  .phase41-settings :global(.text-muted),
  .phase41-settings .tone-neutral {
    color: var(--config-subtle) !important;
  }

  .phase41-settings .rollback-note,
  .phase41-settings .badge-action-group_permissions_update,
  .phase41-settings :global(.fact-status.status-draft) {
    border-color: rgba(165, 101, 0, 0.22) !important;
    background: rgba(216, 135, 0, 0.1) !important;
    color: var(--config-warning) !important;
  }

  .phase41-settings .config-version-error,
  .phase41-settings .error-banner,
  .phase41-settings :global(.error-msg),
  .phase41-settings :global(.error-msg-banner),
  .phase41-settings :global(.inline-error),
  .phase41-settings .decision-card.denied span,
  .phase41-settings .effect-deny,
  .phase41-settings .group-delete-btn,
  .phase41-settings .permission-group-toggles button.locked {
    border-color: rgba(201, 64, 53, 0.22) !important;
    background: rgba(221, 75, 62, 0.1) !important;
    color: var(--config-danger) !important;
  }

  .phase41-settings .success-banner,
  .phase41-settings :global(.inline-success),
  .phase41-settings .decision-card.allowed span,
  .phase41-settings .effect-allow,
  .phase41-settings .permission-group-toggles button.enabled,
  .phase41-settings .toggle-pill.on {
    border-color: rgba(4, 150, 111, 0.22) !important;
    background: rgba(4, 150, 111, 0.1) !important;
    color: var(--config-success) !important;
  }

  .phase41-settings :global(input),
  .phase41-settings :global(textarea),
  .phase41-settings :global(select),
  .phase41-settings .custom-input,
  .phase41-settings .custom-textarea,
  .phase41-settings .custom-select,
  .phase41-settings .dropdown-trigger-btn,
  .phase41-settings :global(.context-input),
  .phase41-settings :global(.context-textarea),
  .phase41-settings :global(.webhook-url-box),
  .phase41-settings :global(.details-pre),
  .phase41-settings :global(.jql-preview code),
  .phase41-settings :global(.pack-text) {
    border: 1px solid var(--config-line-strong) !important;
    border-radius: 10px !important;
    background: rgba(255, 255, 255, 0.92) !important;
    color: var(--config-ink) !important;
    box-shadow: none !important;
    text-shadow: none !important;
  }

  .phase41-settings :global(input:focus),
  .phase41-settings :global(textarea:focus),
  .phase41-settings :global(select:focus),
  .phase41-settings .custom-input:focus,
  .phase41-settings .custom-textarea:focus,
  .phase41-settings .custom-select:focus,
  .phase41-settings .dropdown-trigger-btn:focus,
  .phase41-settings :global(.context-input:focus),
  .phase41-settings :global(.context-textarea:focus) {
    border-color: var(--config-accent) !important;
    background: #ffffff !important;
    box-shadow: 0 0 0 3px rgba(0, 143, 150, 0.12) !important;
  }

  .phase41-settings :global(input::placeholder),
  .phase41-settings :global(textarea::placeholder) {
    color: var(--config-subtle) !important;
  }

  .phase41-settings :global(.segmented-control),
  .phase41-settings :global(.context-choice-grid),
  .phase41-settings .segmented-pills {
    border: 1px solid var(--config-line-strong) !important;
    border-radius: 12px !important;
    background: rgba(248, 252, 253, 0.9) !important;
    padding: 4px !important;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.86) !important;
  }

  .phase41-settings :global(.control-btn),
  .phase41-settings :global(.context-choice-grid button),
  .phase41-settings .segmented-pills button {
    min-height: 34px !important;
    border: 1px solid transparent !important;
    border-radius: 9px !important;
    background: transparent !important;
    color: var(--config-muted) !important;
    box-shadow: none !important;
    text-shadow: none !important;
  }

  .phase41-settings :global(.control-btn:hover),
  .phase41-settings :global(.context-choice-grid button:hover),
  .phase41-settings .segmented-pills button:hover {
    background: rgba(255, 255, 255, 0.86) !important;
    border-color: rgba(0, 143, 150, 0.16) !important;
    color: var(--config-accent-strong) !important;
    transform: none !important;
  }

  .phase41-settings :global(.control-btn.active),
  .phase41-settings :global(.context-choice-grid button.active),
  .phase41-settings .segmented-pills button.active {
    background: var(--config-accent) !important;
    border-color: var(--config-accent) !important;
    color: #ffffff !important;
    box-shadow: 0 8px 20px rgba(0, 143, 150, 0.16) !important;
    text-shadow: none !important;
  }

  .phase41-settings :global(table),
  .phase41-settings :global(.repo-table) {
    color: var(--config-text) !important;
    border-collapse: collapse;
  }

  .phase41-settings :global(th),
  .phase41-settings :global(td),
  .phase41-settings :global(.repo-table th),
  .phase41-settings :global(.repo-table td) {
    border-color: var(--config-line) !important;
    background: transparent !important;
    color: var(--config-text) !important;
    text-transform: none !important;
  }

  .phase41-settings :global(th),
  .phase41-settings :global(.repo-table th) {
    background: rgba(247, 252, 253, 0.78) !important;
    color: var(--config-muted) !important;
    font-size: 0.76rem !important;
    font-weight: 780 !important;
  }

  .phase41-settings :global(tr:hover td),
  .phase41-settings :global(.repo-table tr:hover td) {
    background: rgba(239, 249, 250, 0.64) !important;
  }

  .phase41-settings button:not(.btn):not(.close-modal-btn):not(.dropdown-backdrop-overlay),
  .phase41-settings :global(.copy-link),
  .phase41-settings :global(.gen-btn),
  .phase41-settings :global(.delete-btn),
  .phase41-settings :global(.control-btn) {
    min-height: 34px;
    border: 1px solid var(--config-line) !important;
    border-radius: 10px !important;
    background: rgba(255, 255, 255, 0.76) !important;
    color: var(--config-text) !important;
    box-shadow: none !important;
    transition: background 140ms var(--wa-ease, ease), border-color 140ms var(--wa-ease, ease), transform 140ms var(--wa-ease, ease);
  }

  .phase41-settings button:not(.btn):not(.close-modal-btn):not(.dropdown-backdrop-overlay):hover,
  .phase41-settings :global(.copy-link:hover),
  .phase41-settings :global(.gen-btn:hover),
  .phase41-settings :global(.control-btn:hover) {
    border-color: rgba(0, 143, 150, 0.28) !important;
    background: #ffffff !important;
    color: var(--config-accent-strong) !important;
    transform: translateY(-1px);
  }

  .phase41-settings :global(.control-btn.active),
  .phase41-settings .choice-card.active,
  .phase41-settings .segmented-pills button.active,
  .phase41-settings .suggestion-pills button.active,
  .phase41-settings .action-chip-grid button.active,
  .phase41-settings .version-item.active,
  .phase41-settings :global(.registry-fact-card.active),
  .phase41-settings :global(.context-choice-grid button.active) {
    border-color: rgba(0, 143, 150, 0.28) !important;
    background: var(--config-accent-soft) !important;
    color: var(--config-accent-strong) !important;
  }

  .phase41-settings :global(.steps-container) {
    border: 1px solid var(--config-line) !important;
    border-radius: var(--config-radius-lg) !important;
    background: rgba(255, 255, 255, 0.9) !important;
    box-shadow: none !important;
  }

  .phase41-settings :global(.step-indicator) {
    border-color: var(--config-line-strong) !important;
    background: #ffffff !important;
    color: var(--config-muted) !important;
    box-shadow: none !important;
  }

  .phase41-settings :global(.step-item.active .step-indicator),
  .phase41-settings :global(.step-item.completed .step-indicator) {
    border-color: var(--config-accent) !important;
    background: var(--config-accent) !important;
    color: #ffffff !important;
  }

  .phase41-settings :global(.step-line),
  .phase41-settings :global(.step-item .step-line) {
    background: var(--config-line) !important;
  }

  .phase41-settings :global(.step-item .step-line.completed-line) {
    background: var(--config-accent) !important;
  }

  .phase41-settings :global(.step-label) {
    background: #ffffff !important;
    color: var(--config-muted) !important;
    text-transform: none !important;
  }

  .phase41-settings .settings-content-shell {
    background: transparent !important;
    border-color: transparent !important;
    box-shadow: none !important;
  }

  .phase41-settings .settings-module-panel {
    background: transparent !important;
    border: 0 !important;
    box-shadow: none !important;
  }

  .phase41-settings .settings-content-header,
  .phase41-settings .section-card,
  .phase41-settings .config-audit-panel {
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.95), rgba(250, 254, 254, 0.86)),
      rgba(255, 255, 255, 0.88) !important;
    border: 1px solid rgba(205, 220, 228, 0.76) !important;
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.92),
      0 18px 42px rgba(35, 56, 72, 0.045) !important;
    backdrop-filter: blur(18px) saturate(122%) !important;
    -webkit-backdrop-filter: blur(18px) saturate(122%) !important;
  }

  .phase41-settings .settings-audit-pane .version-detail-header {
    grid-template-columns: minmax(0, 1fr) auto !important;
    align-items: start;
  }

  .phase41-settings .settings-audit-pane .version-detail-header :global(.btn) {
    width: auto !important;
    min-width: 108px;
    justify-self: end;
  }

  .phase41-settings .settings-audit-pane .version-detail-header :global(.btn-danger:disabled) {
    border-color: rgba(201, 64, 53, 0.16) !important;
    background: rgba(201, 64, 53, 0.055) !important;
    color: rgba(154, 58, 51, 0.78) !important;
  }

  .phase41-settings .settings-audit-pane .version-item {
    display: grid;
    align-content: center;
    gap: 5px;
  }

  .phase41-settings .settings-audit-pane .version-sections {
    color: var(--config-accent-strong) !important;
  }

  .phase41-settings .settings-audit-pane .diff-path {
    color: var(--config-accent-strong) !important;
  }

  .phase41-settings .settings-audit-pane .diff-value.before {
    border-color: rgba(201, 64, 53, 0.12) !important;
    background: rgba(201, 64, 53, 0.035) !important;
  }

  .phase41-settings .settings-audit-pane .diff-value.after {
    border-color: rgba(4, 150, 111, 0.13) !important;
    background: rgba(4, 150, 111, 0.035) !important;
  }

  .phase41-settings :global(.form-sub-section > .input-group),
  .phase41-settings :global(.form-group),
  .phase41-settings :global(.form-group-custom),
  .phase41-settings :global(.context-field),
  .phase41-settings :global(.flex-1),
  .phase41-settings :global(.custom-select-wrapper) {
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.82), rgba(248, 253, 254, 0.62)),
      rgba(255, 255, 255, 0.74) !important;
    backdrop-filter: blur(12px) saturate(118%) !important;
    -webkit-backdrop-filter: blur(12px) saturate(118%) !important;
  }

  .phase41-settings .modal-overlay {
    background: rgba(13, 23, 34, 0.22) !important;
    backdrop-filter: blur(12px) !important;
  }

  .phase41-settings .modal-card {
    background: rgba(255, 255, 255, 0.98) !important;
    color: var(--config-text) !important;
  }

  @media (max-width: 1180px) {
    .phase41-settings .settings-content-header {
      grid-template-columns: 1fr;
    }
  }

  /* Phase 46: flat configuration workbench. Glass separates regions, never fields. */
  .phase46-settings {
    --config-ink: #101923;
    --config-text: #31404e;
    --config-muted: #6b7b8b;
    --config-subtle: #92a0ad;
    --config-line: rgba(116, 139, 156, 0.18);
    --config-line-strong: rgba(91, 119, 137, 0.3);
    --config-accent: #008f96;
    --config-accent-strong: #006e74;
    --config-accent-soft: rgba(0, 143, 150, 0.08);
    --config-success: #087f61;
    --config-warning: #a36908;
    --config-danger: #c5473c;
    --config-glass: rgba(255, 255, 255, 0.78);
    --config-glass-strong: rgba(255, 255, 255, 0.9);
    --config-surface: rgba(249, 252, 253, 0.7);
    --config-shadow: 0 18px 44px rgba(41, 62, 78, 0.055);
    width: 100%;
    max-width: min(1720px, 100%);
    min-height: calc(100dvh - var(--wa-workspace-topbar-h, 68px));
    background: transparent !important;
    color: var(--config-text);
  }

  .phase46-settings,
  .phase46-settings * {
    letter-spacing: 0 !important;
    box-sizing: border-box;
  }

  .phase46-settings .settings-main {
    gap: 12px;
    padding-bottom: 20px;
  }

  .phase46-settings .settings-breadcrumb-bar,
  .phase46-settings .settings-content-header,
  .phase46-settings .settings-primary-pane.integration-surface,
  .phase46-settings .section-card,
  .phase46-settings .config-audit-panel {
    border: 1px solid var(--config-line) !important;
    background: var(--config-glass) !important;
    box-shadow: var(--config-shadow) !important;
    backdrop-filter: blur(22px) saturate(126%) !important;
    -webkit-backdrop-filter: blur(22px) saturate(126%) !important;
  }

  .phase46-settings .settings-breadcrumb-bar {
    min-height: 44px;
    padding: 0 14px;
    border-radius: 10px;
  }

  .phase46-settings .settings-breadcrumb-bar button {
    border: 0 !important;
    border-radius: 4px !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase46-settings .settings-content-shell,
  .phase46-settings .settings-module-panel {
    padding: 0 !important;
    border: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase46-settings .settings-content-header {
    min-height: 112px;
    padding: 20px 22px;
    border-radius: 14px;
    grid-template-columns: minmax(260px, 1fr) minmax(520px, 0.92fr);
    align-items: center;
    gap: 28px;
  }

  .phase46-settings .settings-kicker,
  .phase46-settings .audit-kicker,
  .phase46-settings :global(.overview-kicker),
  .phase46-settings :global(.context-kicker) {
    color: var(--config-accent-strong) !important;
    font-size: 11px !important;
    font-weight: 760 !important;
    text-transform: none !important;
  }

  .phase46-settings .settings-content-header h1 {
    margin-top: 4px;
    color: var(--config-ink) !important;
    font-size: clamp(22px, 2vw, 28px);
    line-height: 1.15;
  }

  .phase46-settings .settings-content-header p {
    margin-top: 6px;
    color: var(--config-muted) !important;
    font-size: 13px;
  }

  .phase46-settings .settings-content-facts {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 0;
    padding: 0;
    border: 0;
    background: transparent;
  }

  .phase46-settings .settings-content-facts div {
    min-width: 0;
    min-height: 48px;
    padding: 3px 12px;
    border: 0 !important;
    border-left: 1px solid var(--config-line) !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase46-settings .settings-content-facts span {
    color: var(--config-muted) !important;
    font-size: 11px;
  }

  .phase46-settings .settings-content-facts strong {
    margin-top: 6px;
    color: var(--config-ink) !important;
    font-size: 13px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .phase46-settings .settings-workbench-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(300px, 320px);
    align-items: start;
    gap: 14px;
    margin-top: 12px;
  }

  .phase46-settings .settings-workbench-grid.without-audit {
    grid-template-columns: minmax(0, 1fr);
  }

  .phase46-settings .settings-primary-pane.integration-surface,
  .phase46-settings .section-card,
  .phase46-settings .config-audit-panel {
    min-width: 0;
    border-radius: 14px !important;
  }

  .phase46-settings .settings-primary-pane.integration-surface,
  .phase46-settings .section-card {
    padding: 20px !important;
  }

  .phase46-settings .settings-audit-pane {
    min-width: 0;
    align-self: stretch;
  }

  .phase46-settings .config-audit-panel {
    height: 100%;
    min-height: 440px;
    padding: 18px !important;
    overflow: hidden;
  }

  .phase46-settings :global(.wizard),
  .phase46-settings :global(.project-config-container),
  .phase46-settings :global(.config-overview),
  .phase46-settings :global(.step-content),
  .phase46-settings :global(.success-screen),
  .phase46-settings :global(.form-container),
  .phase46-settings :global(.context-registry-panel),
  .phase46-settings :global(.pack-preview-panel) {
    width: 100%;
    margin: 0 !important;
    padding: 0 !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
    opacity: 1 !important;
    transform: none !important;
    animation: none !important;
    backdrop-filter: none !important;
  }

  .phase46-settings :global(.overview-header),
  .phase46-settings .config-audit-header,
  .phase46-settings .card-header,
  .phase46-settings :global(.registry-header) {
    min-height: 66px;
    margin: 0 !important;
    padding: 0 0 16px !important;
    border: 0 !important;
    border-bottom: 1px solid var(--config-line) !important;
    background: transparent !important;
  }

  .phase46-settings :global(.overview-header h4),
  .phase46-settings .config-audit-header h3,
  .phase46-settings .card-header h2,
  .phase46-settings :global(.registry-header h4) {
    margin: 3px 0 0 !important;
    color: var(--config-ink) !important;
    background: none !important;
    -webkit-text-fill-color: currentColor !important;
    font-size: 17px !important;
    line-height: 1.3 !important;
    text-shadow: none !important;
  }

  .phase46-settings :global(.overview-header p),
  .phase46-settings .card-header p,
  .phase46-settings :global(.registry-header p) {
    margin: 5px 0 0 !important;
    color: var(--config-muted) !important;
    font-size: 12px !important;
    line-height: 1.5 !important;
  }

  .phase46-settings :global(.switch-container) {
    margin: 0 !important;
    padding: 0 !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase46-settings :global(.switch-control) {
    width: 42px !important;
    height: 23px !important;
    border: 0 !important;
    background: #c8d1d8 !important;
    box-shadow: inset 0 0 0 1px rgba(72, 92, 108, 0.12) !important;
  }

  .phase46-settings :global(.switch-control.checked) {
    background: var(--config-accent) !important;
  }

  .phase46-settings :global(.switch-control:focus-visible) {
    outline: 0;
    box-shadow: 0 0 0 3px rgba(0, 143, 150, 0.14) !important;
  }

  .phase46-settings :global(.overview-grid) {
    display: grid !important;
    grid-template-columns: repeat(2, minmax(0, 1fr)) !important;
    gap: 0 28px !important;
    margin: 0 !important;
    padding: 6px 0 0 !important;
    border: 0 !important;
    background: transparent !important;
  }

  .phase46-settings :global(.overview-row) {
    min-width: 0;
    min-height: 66px !important;
    display: grid !important;
    align-content: center !important;
    gap: 5px !important;
    margin: 0 !important;
    padding: 12px 0 !important;
    border: 0 !important;
    border-bottom: 1px solid var(--config-line) !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase46-settings :global(.overview-row span),
  .phase46-settings :global(.summary-label) {
    color: var(--config-muted) !important;
    font-size: 11px !important;
    font-weight: 650 !important;
  }

  .phase46-settings :global(.overview-row strong),
  .phase46-settings :global(.summary-value) {
    min-width: 0;
    color: var(--config-ink) !important;
    font-size: 13px !important;
    font-weight: 720 !important;
    line-height: 1.45 !important;
    overflow-wrap: anywhere;
  }

  .phase46-settings :global(.jql-preview),
  .phase46-settings :global(.details-pre),
  .phase46-settings :global(.summary-card),
  .phase46-settings :global(.credential-collapsed),
  .phase46-settings :global(.webhook-display),
  .phase46-settings :global(.webhook-automation-panel),
  .phase46-settings :global(.autoload-section),
  .phase46-settings :global(.repo-mapping-section),
  .phase46-settings :global(.add-repo-form),
  .phase46-settings :global(.context-config-panel),
  .phase46-settings :global(.registry-list-column),
  .phase46-settings :global(.registry-editor-column),
  .phase46-settings :global(.pack-summary),
  .phase46-settings :global(.pack-items) {
    margin: 0 !important;
    padding: 16px 0 !important;
    border: 0 !important;
    border-radius: 0 !important;
    border-top: 1px solid var(--config-line) !important;
    background: transparent !important;
    box-shadow: none !important;
    backdrop-filter: none !important;
  }

  .phase46-settings :global(.jql-preview code),
  .phase46-settings :global(.details-pre),
  .phase46-settings :global(.webhook-url-box),
  .phase46-settings :global(.pack-text) {
    display: block;
    max-height: 152px;
    padding: 12px !important;
    border: 1px solid var(--config-line) !important;
    border-radius: 7px !important;
    background: rgba(246, 249, 250, 0.86) !important;
    color: var(--config-text) !important;
    box-shadow: none !important;
    overflow: auto;
  }

  .phase46-settings :global(.overview-actions),
  .phase46-settings :global(.actions),
  .phase46-settings :global(.form-actions),
  .phase46-settings :global(.success-actions),
  .phase46-settings :global(.registry-actions) {
    display: flex !important;
    justify-content: flex-end !important;
    align-items: center !important;
    gap: 8px !important;
    margin: 16px 0 0 !important;
    padding: 16px 0 0 !important;
    border-top: 1px solid var(--config-line) !important;
    background: transparent !important;
  }

  .phase46-settings :global(.steps-container) {
    margin: 0 0 18px !important;
    padding: 0 0 15px !important;
    border: 0 !important;
    border-bottom: 1px solid var(--config-line) !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase46-settings :global(.step-indicator) {
    border-color: var(--config-line-strong) !important;
    background: rgba(255, 255, 255, 0.9) !important;
    color: var(--config-muted) !important;
    box-shadow: none !important;
  }

  .phase46-settings :global(.step-item.active .step-indicator),
  .phase46-settings :global(.step-item.completed .step-indicator) {
    border-color: var(--config-accent) !important;
    background: var(--config-accent) !important;
    color: #fff !important;
  }

  .phase46-settings :global(.step-label) {
    background: transparent !important;
    color: var(--config-muted) !important;
  }

  .phase46-settings :global(.info-block) {
    margin: 0 0 18px !important;
    padding: 0 0 14px !important;
    border: 0 !important;
    border-bottom: 1px solid var(--config-line) !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase46-settings :global(.info-block h4) {
    margin: 0 0 5px !important;
    color: var(--config-ink) !important;
    font-size: 15px !important;
  }

  .phase46-settings :global(.info-block p),
  .phase46-settings :global(.helper-text),
  .phase46-settings :global(.helper-text-custom),
  .phase46-settings :global(.config-textarea-helper) {
    color: var(--config-muted) !important;
    font-size: 12px !important;
    line-height: 1.55 !important;
  }

  .phase46-settings :global(.form-section),
  .phase46-settings :global(.form-sub-section),
  .phase46-settings :global(.registry-form-grid),
  .phase46-settings .policy-builder-grid,
  .phase46-settings .policy-form-grid {
    display: grid !important;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px 20px !important;
    margin: 0 !important;
    padding: 0 !important;
    border: 0 !important;
    background: transparent !important;
  }

  .phase46-settings :global(.form-sub-section > .input-group),
  .phase46-settings :global(.input-group),
  .phase46-settings :global(.select-group),
  .phase46-settings :global(.form-group),
  .phase46-settings :global(.form-group-custom),
  .phase46-settings :global(.context-field),
  .phase46-settings :global(.flex-1),
  .phase46-settings :global(.custom-select-wrapper),
  .phase46-settings .policy-choice-block {
    min-width: 0;
    margin: 0 !important;
    padding: 0 !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
    backdrop-filter: none !important;
  }

  .phase46-settings :global(.config-textarea-field-wide),
  .phase46-settings :global(.form-group-custom.wide),
  .phase46-settings .policy-wide {
    grid-column: 1 / -1;
  }

  .phase46-settings :global(input),
  .phase46-settings :global(textarea),
  .phase46-settings :global(select),
  .phase46-settings .custom-input,
  .phase46-settings .custom-textarea,
  .phase46-settings .custom-select,
  .phase46-settings .dropdown-trigger-btn,
  .phase46-settings :global(.select-trigger),
  .phase46-settings :global(.context-input),
  .phase46-settings :global(.context-textarea) {
    min-height: 38px;
    border: 1px solid var(--config-line-strong) !important;
    border-radius: 7px !important;
    background: rgba(255, 255, 255, 0.8) !important;
    color: var(--config-ink) !important;
    box-shadow: none !important;
  }

  .phase46-settings :global(input:focus),
  .phase46-settings :global(textarea:focus),
  .phase46-settings :global(select:focus),
  .phase46-settings .custom-input:focus,
  .phase46-settings .custom-textarea:focus,
  .phase46-settings .dropdown-trigger-btn:focus,
  .phase46-settings :global(.select-trigger:focus),
  .phase46-settings :global(.context-input:focus),
  .phase46-settings :global(.context-textarea:focus) {
    outline: 0;
    border-color: var(--config-accent) !important;
    background: #fff !important;
    box-shadow: 0 0 0 3px rgba(0, 143, 150, 0.1) !important;
  }

  .phase46-settings :global(.credential-collapsed) {
    grid-column: 1 / -1;
    display: flex !important;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    border-bottom: 1px solid var(--config-line) !important;
  }

  .phase46-settings :global(.segmented-control),
  .phase46-settings :global(.context-choice-grid),
  .phase46-settings .segmented-pills {
    display: inline-flex !important;
    width: fit-content;
    max-width: 100%;
    gap: 2px !important;
    padding: 3px !important;
    border: 1px solid var(--config-line) !important;
    border-radius: 8px !important;
    background: rgba(240, 245, 247, 0.76) !important;
    box-shadow: none !important;
  }

  .phase46-settings :global(.control-btn),
  .phase46-settings :global(.context-choice-grid button),
  .phase46-settings .segmented-pills button {
    min-height: 32px !important;
    padding: 0 11px !important;
    border: 0 !important;
    border-radius: 6px !important;
    background: transparent !important;
    color: var(--config-muted) !important;
    box-shadow: none !important;
  }

  .phase46-settings :global(.control-btn.active),
  .phase46-settings :global(.context-choice-grid button.active),
  .phase46-settings .segmented-pills button.active {
    background: rgba(255, 255, 255, 0.96) !important;
    color: var(--config-accent-strong) !important;
    box-shadow: 0 1px 3px rgba(28, 54, 67, 0.1) !important;
  }

  .phase46-settings :global(.registry-layout) {
    display: grid !important;
    grid-template-columns: minmax(260px, 0.8fr) minmax(0, 1.2fr) !important;
    gap: 24px !important;
  }

  .phase46-settings :global(.registry-list-column),
  .phase46-settings :global(.registry-editor-column) {
    border-top: 0 !important;
  }

  .phase46-settings :global(.registry-editor-column) {
    padding-left: 24px !important;
    border-left: 1px solid var(--config-line) !important;
  }

  .phase46-settings :global(.registry-fact-card),
  .phase46-settings :global(.pack-item),
  .phase46-settings :global(.project-card-item),
  .phase46-settings :global(.summary-repo-item) {
    width: 100%;
    margin: 0 !important;
    padding: 12px 0 !important;
    border: 0 !important;
    border-bottom: 1px solid var(--config-line) !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase46-settings :global(.registry-fact-card.active) {
    padding-inline: 10px !important;
    background: var(--config-accent-soft) !important;
  }

  .phase46-settings :global(.table-responsive),
  .phase46-settings :global(.table-container),
  .phase46-settings :global(.webhook-result-table) {
    overflow: auto;
    border: 1px solid var(--config-line) !important;
    border-radius: 8px !important;
    background: rgba(255, 255, 255, 0.46) !important;
    box-shadow: none !important;
  }

  .phase46-settings :global(table) {
    width: 100%;
    border-collapse: collapse;
    color: var(--config-text) !important;
  }

  .phase46-settings :global(th) {
    padding: 10px 12px !important;
    border-bottom: 1px solid var(--config-line) !important;
    background: rgba(242, 247, 248, 0.76) !important;
    color: var(--config-muted) !important;
    font-size: 11px !important;
    text-transform: none !important;
  }

  .phase46-settings :global(td) {
    padding: 11px 12px !important;
    border-bottom: 1px solid var(--config-line) !important;
    background: transparent !important;
    color: var(--config-text) !important;
  }

  .phase46-settings :global(tr:hover td) {
    background: rgba(0, 143, 150, 0.035) !important;
  }

  .phase46-settings .config-audit-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 12px;
  }

  .phase46-settings .config-audit-header h3 {
    display: grid;
    gap: 2px;
  }

  .phase46-settings .audit-scope {
    color: var(--config-muted);
    font-size: 11px;
    font-weight: 620;
  }

  .phase46-settings .version-layout {
    display: grid !important;
    grid-template-rows: minmax(0, 190px) minmax(0, 1fr) !important;
    gap: 0 !important;
    height: calc(100% - 70px);
    min-height: 350px;
  }

  .phase46-settings .version-list {
    display: block !important;
    overflow: auto !important;
    padding: 6px 0 !important;
    border-bottom: 1px solid var(--config-line) !important;
  }

  .phase46-settings .version-item {
    width: 100%;
    min-height: 52px;
    display: grid !important;
    grid-template-columns: auto 1fr;
    gap: 3px 10px !important;
    align-items: center;
    margin: 0 !important;
    padding: 8px 9px !important;
    border: 0 !important;
    border-radius: 6px !important;
    background: transparent !important;
    color: var(--config-text) !important;
    box-shadow: none !important;
    text-align: left;
  }

  .phase46-settings .version-item:hover,
  .phase46-settings .version-item.active {
    background: var(--config-accent-soft) !important;
  }

  .phase46-settings .version-title {
    color: var(--config-ink) !important;
    font-size: 12px !important;
  }

  .phase46-settings .version-meta,
  .phase46-settings .version-sections {
    color: var(--config-muted) !important;
    font-size: 10px !important;
  }

  .phase46-settings .version-sections {
    grid-column: 2;
  }

  .phase46-settings .version-detail {
    min-width: 0;
    padding: 14px 0 0 !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
    overflow: hidden;
  }

  .phase46-settings .version-detail-header {
    display: grid !important;
    grid-template-columns: minmax(0, 1fr) auto !important;
    align-items: start;
    gap: 10px;
    margin-bottom: 10px;
    padding: 0 !important;
    border: 0 !important;
    background: transparent !important;
  }

  .phase46-settings .diff-table {
    max-height: 260px;
    overflow: auto;
    border-top: 1px solid var(--config-line);
  }

  .phase46-settings .diff-row {
    display: grid !important;
    grid-template-columns: minmax(0, 1fr) !important;
    gap: 6px !important;
    margin: 0 !important;
    padding: 9px 0 !important;
    border: 0 !important;
    border-bottom: 1px solid var(--config-line) !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase46-settings .diff-arrow {
    display: none !important;
  }

  .phase46-settings .diff-value {
    min-width: 0;
    max-height: 76px;
    overflow: auto;
    padding: 6px !important;
    border: 0 !important;
    border-radius: 4px !important;
    background: rgba(246, 249, 250, 0.8) !important;
    color: var(--config-text) !important;
    font-size: 10px !important;
  }

  .phase46-settings .diff-value.before {
    background: rgba(197, 71, 60, 0.045) !important;
  }

  .phase46-settings .diff-value.after {
    background: rgba(8, 127, 97, 0.05) !important;
  }

  .phase46-settings .empty-version-state,
  .phase46-settings .diff-empty,
  .phase46-settings :global(.registry-empty),
  .phase46-settings :global(.loading-state),
  .phase46-settings :global(.empty-state) {
    padding: 24px 12px !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    color: var(--config-muted) !important;
    box-shadow: none !important;
    text-align: center;
  }

  .phase46-settings :global(.text-success) {
    color: var(--config-success) !important;
  }

  .phase46-settings :global(.text-warning),
  .phase46-settings .rollback-note {
    color: var(--config-warning) !important;
  }

  .phase46-settings :global(.alert),
  .phase46-settings .error-banner,
  .phase46-settings .success-banner,
  .phase46-settings :global(.inline-error),
  .phase46-settings :global(.inline-success) {
    border-radius: 7px !important;
    box-shadow: none !important;
    backdrop-filter: none !important;
  }

  .phase46-settings .policy-panel,
  .phase46-settings .branch-content,
  .phase46-settings .permission-node,
  .phase46-settings .group-coverage-card,
  .phase46-settings .decision-card,
  .phase46-settings .decision-log-item {
    margin: 0 !important;
    padding: 16px 0 !important;
    border: 0 !important;
    border-bottom: 1px solid var(--config-line) !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase46-settings .choice-card,
  .phase46-settings .suggestion-pills button,
  .phase46-settings .action-chip-grid button,
  .phase46-settings .permission-group-toggles button,
  .phase46-settings .policy-template-strip button {
    border-color: var(--config-line) !important;
    border-radius: 7px !important;
    background: rgba(255, 255, 255, 0.56) !important;
    color: var(--config-text) !important;
    box-shadow: none !important;
  }

  .phase46-settings :global(.custom-select-dropdown),
  .phase46-settings :global(.select-dropdown),
  .phase46-settings .dropdown-options-list {
    border: 1px solid var(--config-line) !important;
    border-radius: 9px !important;
    background: rgba(255, 255, 255, 0.96) !important;
    color: var(--config-text) !important;
    box-shadow: 0 18px 44px rgba(34, 55, 70, 0.14) !important;
    backdrop-filter: blur(18px) saturate(124%) !important;
  }

  .phase46-settings .modal-overlay {
    background: rgba(18, 29, 39, 0.24) !important;
    backdrop-filter: blur(10px) !important;
  }

  .phase46-settings .modal-card {
    border: 1px solid var(--config-line) !important;
    border-radius: 12px !important;
    background: rgba(255, 255, 255, 0.94) !important;
    color: var(--config-text) !important;
    box-shadow: 0 28px 72px rgba(20, 38, 51, 0.18) !important;
  }

  @media (max-width: 1120px) {
    .phase46-settings .settings-content-header {
      grid-template-columns: 1fr;
      gap: 16px;
    }

    .phase46-settings .settings-content-facts div:first-child {
      border-left: 0 !important;
    }
  }

  @media (max-width: 1080px) {
    .phase46-settings .settings-workbench-grid {
      grid-template-columns: minmax(0, 1fr);
    }

    .phase46-settings .config-audit-panel {
      min-height: auto;
    }

    .phase46-settings .version-layout {
      height: auto;
      grid-template-columns: minmax(220px, 0.7fr) minmax(0, 1.3fr) !important;
      grid-template-rows: minmax(300px, auto) !important;
    }

    .phase46-settings .version-list {
      border-right: 1px solid var(--config-line) !important;
      border-bottom: 0 !important;
      padding-right: 10px !important;
    }

    .phase46-settings .version-detail {
      padding: 6px 0 0 14px !important;
    }
  }

  @media (max-width: 1380px) {
    .phase41-settings.phase46-settings .settings-workbench-grid:has(:global(.project-config-container)) {
      grid-template-columns: minmax(0, 1fr);
    }

    .phase41-settings.phase46-settings .settings-workbench-grid:has(:global(.project-config-container)) .config-audit-panel {
      min-height: auto;
    }

    .phase41-settings.phase46-settings .settings-workbench-grid:has(:global(.project-config-container)) .version-layout {
      height: auto;
      grid-template-columns: minmax(220px, 0.7fr) minmax(0, 1.3fr) !important;
      grid-template-rows: minmax(280px, auto) !important;
    }
  }

  @media (max-width: 760px) {
    .phase46-settings .settings-content-header,
    .phase46-settings .settings-primary-pane.integration-surface,
    .phase46-settings .section-card,
    .phase46-settings .config-audit-panel {
      padding: 16px !important;
      border-radius: 10px !important;
    }

    .phase46-settings .settings-content-facts {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .phase46-settings .settings-content-facts div:nth-child(odd) {
      border-left: 0 !important;
    }

    .phase46-settings :global(.overview-grid),
    .phase46-settings :global(.form-section),
    .phase46-settings :global(.form-sub-section),
    .phase46-settings :global(.registry-form-grid),
    .phase46-settings :global(.registry-layout),
    .phase46-settings .policy-builder-grid,
    .phase46-settings .policy-form-grid {
      grid-template-columns: minmax(0, 1fr) !important;
    }

    .phase46-settings :global(.registry-editor-column) {
      padding-left: 0 !important;
      border-left: 0 !important;
      border-top: 1px solid var(--config-line) !important;
    }

    .phase46-settings .version-layout {
      display: block !important;
    }

    .phase46-settings .version-list {
      max-height: 190px;
      border-right: 0 !important;
      border-bottom: 1px solid var(--config-line) !important;
      padding-right: 0 !important;
    }

    .phase46-settings .version-detail {
      padding: 14px 0 0 !important;
    }
  }

  /* Specificity lock while Phase 41 selectors remain for untouched Settings routes. */
  .phase41-settings.phase46-settings :global(.overview-row),
  .phase41-settings.phase46-settings :global(.summary-row) {
    border: 0 !important;
    border-bottom: 1px solid var(--config-line) !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
    backdrop-filter: none !important;
  }

  .phase41-settings.phase46-settings .settings-audit-pane .version-layout {
    grid-template-columns: minmax(0, 1fr) !important;
  }

  .phase41-settings.phase46-settings .settings-audit-pane .version-item,
  .phase41-settings.phase46-settings :global(.registry-fact-card),
  .phase41-settings.phase46-settings :global(.pack-item),
  .phase41-settings.phase46-settings :global(.project-card-item),
  .phase41-settings.phase46-settings :global(.summary-repo-item) {
    border: 0 !important;
    border-bottom: 1px solid var(--config-line) !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
    backdrop-filter: none !important;
  }

  .phase41-settings.phase46-settings .settings-audit-pane button.version-item:not(.btn) {
    border: 0 !important;
    border-bottom: 1px solid var(--config-line) !important;
    border-radius: 0 !important;
    background: transparent !important;
  }

  .phase41-settings.phase46-settings .settings-audit-pane .version-item:hover,
  .phase41-settings.phase46-settings .settings-audit-pane .version-item.active,
  .phase41-settings.phase46-settings :global(.registry-fact-card.active) {
    background: var(--config-accent-soft) !important;
  }

  .phase41-settings.phase46-settings .settings-audit-pane button.version-item.active:not(.btn),
  .phase41-settings.phase46-settings .settings-audit-pane button.version-item:hover:not(.btn) {
    background: var(--config-accent-soft) !important;
  }

  .phase41-settings.phase46-settings .settings-audit-pane .version-detail,
  .phase41-settings.phase46-settings .settings-audit-pane .diff-row {
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase41-settings.phase46-settings .settings-audit-pane .diff-row {
    border-bottom: 1px solid var(--config-line) !important;
  }

  .phase41-settings.phase46-settings :global(.context-choice-grid) {
    width: 100% !important;
    display: grid !important;
    grid-template-columns: repeat(auto-fit, minmax(84px, 1fr)) !important;
    gap: 6px !important;
    padding: 0 !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase41-settings.phase46-settings :global(.context-choice-grid button) {
    min-width: 0 !important;
    min-height: 34px !important;
    padding: 0 9px !important;
    border: 1px solid var(--config-line) !important;
    border-radius: 6px !important;
    background: rgba(255, 255, 255, 0.46) !important;
    color: var(--config-text) !important;
    white-space: nowrap;
  }

  .phase41-settings.phase46-settings :global(.context-choice-grid button.active) {
    border-color: rgba(0, 143, 150, 0.28) !important;
    background: var(--config-accent-soft) !important;
    color: var(--config-accent-strong) !important;
    box-shadow: none !important;
  }

  .phase41-settings.phase46-settings :global(.registry-metrics) {
    display: flex !important;
    align-items: center;
    gap: 0 !important;
  }

  .phase41-settings.phase46-settings :global(.registry-metrics span) {
    padding: 0 10px !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    color: var(--config-muted) !important;
    box-shadow: none !important;
  }

  .phase41-settings.phase46-settings :global(.registry-metrics span + span) {
    border-left: 1px solid var(--config-line) !important;
  }

  .phase41-settings.phase46-settings .permission-branch {
    display: block !important;
    margin: 0 !important;
    padding: 18px 0 0 !important;
    border-top: 1px solid var(--config-line) !important;
  }

  .phase41-settings.phase46-settings .branch-stem {
    display: none !important;
  }

  .phase41-settings.phase46-settings .branch-content,
  .phase41-settings.phase46-settings .branch-header {
    margin: 0 !important;
    padding: 0 0 14px !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase41-settings.phase46-settings .branch-header {
    border-bottom: 1px solid var(--config-line) !important;
  }

  .phase41-settings.phase46-settings .coverage-bar i {
    background: var(--config-accent) !important;
    box-shadow: none !important;
  }

  @media (max-width: 1080px) {
    .phase41-settings.phase46-settings .settings-audit-pane .version-layout {
      grid-template-columns: minmax(220px, 0.7fr) minmax(0, 1.3fr) !important;
    }
  }

  @media (max-width: 760px) {
    .phase41-settings.phase46-settings .settings-audit-pane .version-layout {
      display: block !important;
    }
  }

  /* Phase 49: compact page context and a height-synchronized audit inspector. */
  .phase41-settings.phase46-settings.phase49-settings .settings-breadcrumb-bar {
    min-height: 44px;
    border-radius: 8px !important;
  }

  .phase41-settings.phase46-settings.phase49-settings .settings-breadcrumb-bar li span {
    display: inline;
    padding: 0 !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .phase41-settings.phase46-settings.phase49-settings .settings-content-header {
    min-height: 86px;
    grid-template-columns: minmax(240px, 0.78fr) minmax(420px, 1.22fr);
    gap: 20px;
    padding: 14px 18px !important;
    border-radius: 8px !important;
  }

  .phase41-settings.phase46-settings.phase49-settings .settings-content-header h1 {
    font-size: 22px;
  }

  .phase41-settings.phase46-settings.phase49-settings .settings-content-facts {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .phase41-settings.phase46-settings.phase49-settings .settings-workbench-grid {
    grid-template-columns: minmax(0, 1fr) minmax(380px, 420px);
    align-items: stretch;
    gap: 14px;
  }

  .phase41-settings.phase46-settings.phase49-settings .settings-audit-pane {
    position: relative;
    align-self: stretch !important;
    min-width: 0;
    min-height: 0;
    height: auto;
    max-height: none;
    overflow: hidden;
  }

  .phase41-settings.phase46-settings.phase49-settings .config-audit-panel,
  :global(.phase41-settings.phase46-settings.phase49-settings .settings-workbench-grid:has(.config-overview) .config-audit-panel),
  :global(.phase41-settings.phase46-settings.phase49-settings .settings-workbench-grid:has(.project-config-container) .config-audit-panel) {
    width: 100%;
    height: 100% !important;
    min-height: 0 !important;
    max-height: none;
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    padding: 16px !important;
    border-radius: 8px !important;
    overflow: hidden;
  }

  .phase41-settings.phase46-settings.phase49-settings .config-audit-header {
    flex: 0 0 auto;
    min-height: 64px;
  }

  .phase41-settings.phase46-settings.phase49-settings .version-layout,
  .phase41-settings.phase46-settings.phase49-settings .settings-audit-pane .version-layout {
    flex: 1 1 auto;
    min-height: 0;
    height: auto !important;
    display: grid !important;
    grid-template-columns: minmax(0, 1fr) !important;
    grid-template-rows: minmax(112px, 210px) minmax(0, 1fr) !important;
    overflow: hidden;
  }

  .phase41-settings.phase46-settings.phase49-settings .version-list {
    min-height: 0;
    max-height: 210px;
    overflow-y: auto !important;
    overflow-x: hidden !important;
    scrollbar-gutter: stable;
    scrollbar-width: thin;
    scrollbar-color: rgba(0, 143, 150, 0.38) rgba(121, 139, 159, 0.1);
  }

  .phase41-settings.phase46-settings.phase49-settings .version-item {
    grid-template-columns: 48px minmax(0, 1fr) !important;
  }

  .phase41-settings.phase46-settings.phase49-settings .version-detail {
    min-height: 0;
    display: grid;
    grid-template-rows: auto auto minmax(0, 1fr);
    overflow: hidden !important;
  }

  .phase41-settings.phase46-settings.phase49-settings .version-detail-header {
    grid-template-columns: minmax(0, 1fr) !important;
  }

  .phase41-settings.phase46-settings.phase49-settings .version-detail-header :global(.btn) {
    width: fit-content !important;
    justify-self: start !important;
  }

  .phase41-settings.phase46-settings.phase49-settings .diff-table {
    min-height: 0;
    max-height: none;
    overflow-y: auto;
    overflow-x: hidden;
    scrollbar-gutter: stable;
    scrollbar-width: thin;
    scrollbar-color: rgba(0, 143, 150, 0.38) rgba(121, 139, 159, 0.1);
  }

  .phase41-settings.phase46-settings.phase49-settings .version-list::-webkit-scrollbar,
  .phase41-settings.phase46-settings.phase49-settings .diff-table::-webkit-scrollbar {
    width: 10px;
    height: 10px;
  }

  .phase41-settings.phase46-settings.phase49-settings .version-list::-webkit-scrollbar-track,
  .phase41-settings.phase46-settings.phase49-settings .diff-table::-webkit-scrollbar-track,
  .phase41-settings.phase46-settings.phase49-settings .version-list::-webkit-scrollbar-corner,
  .phase41-settings.phase46-settings.phase49-settings .diff-table::-webkit-scrollbar-corner {
    background: rgba(121, 139, 159, 0.08) !important;
  }

  .phase41-settings.phase46-settings.phase49-settings .version-list::-webkit-scrollbar-thumb,
  .phase41-settings.phase46-settings.phase49-settings .diff-table::-webkit-scrollbar-thumb {
    min-height: 44px;
    border: 2px solid rgba(255, 255, 255, 0.72);
    border-radius: 999px;
    background: rgba(0, 143, 150, 0.36) !important;
  }

  .phase41-settings.phase46-settings.phase49-settings .version-list::-webkit-scrollbar-thumb:hover,
  .phase41-settings.phase46-settings.phase49-settings .diff-table::-webkit-scrollbar-thumb:hover {
    background: rgba(0, 143, 150, 0.54) !important;
  }

  .phase41-settings.phase46-settings.phase49-settings .diff-row {
    gap: 8px !important;
    padding: 11px 0 !important;
  }

  .phase41-settings.phase46-settings.phase49-settings .diff-path {
    display: block;
    overflow-wrap: anywhere;
  }

  .phase41-settings.phase46-settings.phase49-settings .diff-value {
    max-height: none;
    overflow: visible;
    display: grid;
    gap: 5px;
    padding: 8px !important;
  }

  .phase41-settings.phase46-settings.phase49-settings .diff-value > span {
    color: var(--config-muted);
    font-size: 10px;
    font-weight: 760;
  }

  .phase41-settings.phase46-settings.phase49-settings .diff-value pre {
    max-width: 100%;
    margin: 0;
    color: var(--config-text);
    font-size: 10px;
    line-height: 1.45;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    word-break: break-word;
  }

  @media (max-width: 1280px) {
    .phase41-settings.phase46-settings.phase49-settings .settings-workbench-grid {
      grid-template-columns: minmax(0, 1fr);
    }

    .phase41-settings.phase46-settings.phase49-settings .settings-audit-pane {
      position: static;
      align-self: auto !important;
      width: 100%;
      height: min(640px, calc(100dvh - 110px));
      max-height: 640px;
      overflow: hidden;
    }

    .phase41-settings.phase46-settings.phase49-settings .config-audit-panel,
    :global(.phase41-settings.phase46-settings.phase49-settings .settings-workbench-grid:has(.config-overview) .config-audit-panel),
    :global(.phase41-settings.phase46-settings.phase49-settings .settings-workbench-grid:has(.project-config-container) .config-audit-panel) {
      position: relative;
      inset: auto;
    }
  }

  @media (max-width: 760px) {
    .phase41-settings.phase46-settings.phase49-settings .settings-content-header {
      grid-template-columns: minmax(0, 1fr);
    }

    .phase41-settings.phase46-settings.phase49-settings .settings-content-facts {
      grid-template-columns: minmax(0, 1fr);
    }

    .phase41-settings.phase46-settings.phase49-settings .settings-content-facts div {
      border-left: 0 !important;
      border-top: 1px solid var(--config-line) !important;
    }

    .phase41-settings.phase46-settings.phase49-settings .settings-audit-pane {
      height: 620px;
      max-height: 620px;
    }

    .phase41-settings.phase46-settings.phase49-settings .version-layout,
    .phase41-settings.phase46-settings.phase49-settings .settings-audit-pane .version-layout {
      display: grid !important;
      grid-template-rows: 190px minmax(0, 1fr) !important;
    }
  }

  /* Unified Settings workbench: the final rendered contract for every Settings route. */
  .settings-unified {
    --settings-line: rgba(83, 108, 130, 0.16);
    --settings-line-strong: rgba(59, 87, 111, 0.28);
    --settings-glass: rgba(248, 252, 253, 0.78);
    --settings-glass-strong: rgba(251, 253, 254, 0.9);
    --settings-flat: #f8fbfc;
    --settings-inset: #eef5f7;
    --settings-ink: var(--wa-text-strong, #0d1722);
    --settings-text: var(--wa-text-main, #293847);
    --settings-muted: var(--wa-text-muted, #667789);
    --settings-subtle: #738497;
    --settings-accent: var(--wa-accent, #008f96);
    --settings-accent-strong: var(--wa-accent-strong, #006f76);
    --settings-accent-soft: rgba(0, 143, 150, 0.09);
    width: 100%;
    min-width: 0;
    padding: 0 0 28px;
    color: var(--settings-text);
  }

  .settings-unified .settings-main,
  .settings-unified .settings-content-shell,
  .settings-unified .settings-module-panel {
    width: 100%;
    min-width: 0;
    height: auto;
    max-height: none;
    overflow: visible;
    padding: 0;
  }

  .settings-unified .settings-content-shell {
    display: grid;
    gap: 16px;
  }

  .settings-unified .settings-context-panel {
    position: relative;
    overflow: hidden;
    border: 1px solid rgba(255, 255, 255, 0.78);
    border-radius: 14px;
    background: var(--settings-glass);
    background-clip: padding-box;
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.86),
      0 10px 26px rgba(29, 54, 72, 0.07);
    -webkit-backdrop-filter: blur(20px) saturate(128%);
    backdrop-filter: blur(20px) saturate(128%);
  }

  .settings-unified .settings-context-panel::after {
    content: "";
    position: absolute;
    inset: 0;
    pointer-events: none;
    border: 1px solid var(--settings-line);
    border-radius: inherit;
  }

  .settings-unified .settings-context-topline {
    position: relative;
    z-index: 1;
    min-height: 42px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 0 18px;
    border-bottom: 1px solid var(--settings-line);
    background: rgba(242, 248, 249, 0.46);
  }

  .settings-unified .settings-breadcrumb-bar {
    min-width: 0;
    min-height: 0;
    padding: 0 !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
    backdrop-filter: none !important;
    -webkit-backdrop-filter: none !important;
  }

  .settings-unified .settings-breadcrumb-bar ol {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 7px;
    margin: 0;
    padding: 0;
    list-style: none;
    overflow: hidden;
  }

  .settings-unified .settings-breadcrumb-bar li {
    min-width: 0;
    display: inline-flex;
    align-items: center;
    gap: 7px;
    color: var(--settings-muted);
    font-size: 11px;
    font-weight: 680;
    white-space: nowrap;
  }

  .settings-unified .settings-breadcrumb-bar li + li::before {
    content: "/";
    color: rgba(102, 119, 137, 0.5);
    font-weight: 500;
  }

  .settings-unified .settings-breadcrumb-bar li.current span {
    overflow: hidden;
    color: var(--settings-ink);
    font-weight: 780;
    text-overflow: ellipsis;
  }

  .settings-unified .breadcrumb-meta {
    flex: none;
    display: flex;
    align-items: center;
    gap: 9px;
    color: var(--settings-muted);
    font-size: 11px;
  }

  .settings-unified .settings-content-header {
    position: relative;
    z-index: 1;
    min-height: 96px;
    display: grid;
    grid-template-columns: minmax(240px, 0.82fr) minmax(420px, 1.18fr);
    align-items: center;
    gap: 26px;
    padding: 16px 18px !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
    backdrop-filter: none !important;
  }

  .settings-unified .settings-title-copy {
    min-width: 0;
  }

  .settings-unified .settings-kicker,
  .settings-unified .audit-kicker {
    display: block;
    margin: 0;
    color: var(--settings-accent-strong);
    font-size: 11px;
    font-weight: 780;
    letter-spacing: 0;
  }

  .settings-unified .settings-content-header h1 {
    margin: 3px 0 0;
    color: var(--settings-ink);
    font-size: 22px;
    font-weight: 780;
    line-height: 1.2;
    letter-spacing: -0.02em;
    text-wrap: balance;
  }

  .settings-unified .settings-content-header h1:focus-visible {
    outline: 2px solid rgba(0, 143, 150, 0.28);
    outline-offset: 4px;
    border-radius: 4px;
  }

  .settings-unified .settings-content-header p {
    max-width: 58ch;
    margin: 5px 0 0;
    color: var(--settings-muted);
    font-size: 12px;
    line-height: 1.5;
    text-wrap: pretty;
  }

  .settings-unified .settings-content-facts {
    min-width: 0;
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0;
    margin: 0;
  }

  .settings-unified .settings-content-facts div {
    min-width: 0;
    min-height: 54px;
    display: grid;
    align-content: center;
    gap: 5px;
    padding: 0 16px;
    border: 0 !important;
    border-left: 1px solid var(--settings-line) !important;
  }

  .settings-unified .settings-content-facts dt,
  .settings-unified .settings-content-facts dd {
    min-width: 0;
    margin: 0;
  }

  .settings-unified .settings-content-facts dt {
    color: var(--settings-muted);
    font-size: 10px;
    font-weight: 700;
  }

  .settings-unified .settings-content-facts dd {
    overflow: hidden;
    color: var(--settings-ink);
    font-size: 13px;
    font-weight: 760;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .settings-unified .settings-module-panel {
    container-type: inline-size;
  }

  .settings-unified .settings-workbench-grid {
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(360px, 392px) !important;
    align-items: start !important;
    gap: 16px !important;
  }

  .settings-unified .settings-workbench-grid.without-audit {
    grid-template-columns: minmax(0, 1fr) !important;
  }

  .settings-unified .settings-primary-pane,
  .settings-unified .settings-audit-pane {
    min-width: 0;
    border: 1px solid rgba(255, 255, 255, 0.78);
    border-radius: 14px;
    background: var(--settings-glass-strong);
    background-clip: padding-box;
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.88),
      0 14px 30px rgba(29, 54, 72, 0.065);
    -webkit-backdrop-filter: blur(18px) saturate(124%);
    backdrop-filter: blur(18px) saturate(124%);
  }

  .settings-unified .settings-primary-pane {
    padding: 20px !important;
    overflow: visible;
  }

  .settings-unified .settings-audit-pane {
    position: sticky !important;
    inset: auto !important;
    top: 12px;
    align-self: start !important;
    width: auto;
    height: auto !important;
    min-height: 0;
    max-height: calc(100dvh - 112px) !important;
    overflow: hidden !important;
  }

  /* Phase 64: one authoritative owner for panel geometry above legacy visual skins. */
  #settings-unified-root.settings-unified .settings-workbench-grid {
    grid-template-columns: minmax(0, 1fr) minmax(360px, 392px) !important;
    align-items: start !important;
    gap: 16px !important;
  }

  #settings-unified-root.settings-unified .settings-workbench-grid.without-audit {
    grid-template-columns: minmax(0, 1fr) !important;
  }

  #settings-unified-root.settings-unified .settings-primary-pane {
    position: relative !important;
    inset: auto !important;
    align-self: start !important;
    height: auto !important;
    min-height: 0 !important;
    overflow: visible !important;
  }

  #settings-unified-root.settings-unified .settings-audit-pane {
    position: sticky !important;
    inset: auto !important;
    top: 12px !important;
    align-self: start !important;
    width: auto !important;
    height: auto !important;
    min-height: 0 !important;
    max-height: calc(100dvh - 112px) !important;
    overflow: hidden !important;
  }

  #settings-unified-root.settings-unified .config-audit-panel {
    position: relative !important;
    inset: auto !important;
    width: 100% !important;
    height: auto !important;
    min-height: 0 !important;
    max-height: inherit !important;
  }

  .settings-unified .section-card,
  .settings-unified .config-audit-panel,
  .settings-unified :global(.scw-workbench),
  .settings-unified :global(.gitlab-workbench) {
    min-width: 0;
    margin: 0 !important;
    padding: 0 !important;
    border: 0 !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
    backdrop-filter: none !important;
    -webkit-backdrop-filter: none !important;
  }

  .settings-unified .section-card {
    display: grid;
    gap: 18px;
  }

  .settings-unified .card-header,
  .settings-unified .config-audit-header,
  .settings-unified :global(.scw-header),
  .settings-unified :global(.overview-header) {
    min-height: 66px;
    margin: 0 !important;
    padding: 0 0 16px !important;
    border: 0 !important;
    border-bottom: 1px solid var(--settings-line) !important;
    background: transparent !important;
  }

  .settings-unified .card-header h2,
  .settings-unified .config-audit-header h3,
  .settings-unified :global(.scw-header h3),
  .settings-unified :global(.scw-header h4),
  .settings-unified :global(.overview-header h4) {
    margin: 3px 0 0 !important;
    color: var(--settings-ink) !important;
    font-size: 16px !important;
    font-weight: 760 !important;
    line-height: 1.35 !important;
    letter-spacing: -0.01em !important;
  }

  .settings-unified .card-header p,
  .settings-unified :global(.scw-header p),
  .settings-unified :global(.overview-header p) {
    margin: 5px 0 0 !important;
    color: var(--settings-muted) !important;
    font-size: 12px !important;
    line-height: 1.5 !important;
  }

  .settings-unified .config-audit-panel {
    position: relative !important;
    inset: auto !important;
    width: 100%;
    height: auto !important;
    min-height: 0 !important;
    max-height: inherit !important;
    display: flex;
    flex-direction: column;
    padding: 18px !important;
    overflow: hidden !important;
  }

  .settings-unified .config-audit-header {
    flex: 0 0 auto;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }

  .settings-unified .audit-scope {
    display: block;
    margin-top: 3px;
    color: var(--settings-muted);
    font-size: 11px;
    font-weight: 640;
  }

  .settings-unified .version-layout,
  .settings-unified .settings-audit-pane .version-layout {
    flex: 1 1 auto;
    min-height: 0;
    height: auto !important;
    display: grid !important;
    grid-template-columns: minmax(0, 1fr) !important;
    grid-template-rows: minmax(96px, 184px) minmax(0, 1fr) !important;
    gap: 0;
    overflow: hidden;
  }

  .settings-unified .version-list {
    min-height: 0;
    max-height: 184px;
    overflow: auto;
    padding: 6px 0 8px !important;
    border: 0 !important;
    border-bottom: 1px solid var(--settings-line) !important;
  }

  .settings-unified .version-item {
    width: 100%;
    min-height: 52px;
    display: grid;
    grid-template-columns: 44px minmax(0, 1fr) !important;
    align-items: center;
    gap: 3px 10px;
    padding: 8px 10px !important;
    border: 0 !important;
    border-radius: 8px !important;
    background: transparent !important;
    color: var(--settings-text) !important;
    text-align: left;
    cursor: pointer;
  }

  .settings-unified .version-item:hover,
  .settings-unified .version-item.active {
    background: var(--settings-accent-soft) !important;
  }

  .settings-unified .version-item.active {
    box-shadow: inset 0 0 0 1px rgba(0, 143, 150, 0.16) !important;
  }

  .settings-unified .version-title {
    color: var(--settings-ink);
    font-size: 12px;
    font-weight: 800;
  }

  .settings-unified .version-meta,
  .settings-unified .version-sections {
    color: var(--settings-muted);
    font-size: 10px;
  }

  .settings-unified .version-detail {
    min-height: 0;
    display: grid;
    grid-template-rows: auto auto minmax(0, 1fr);
    padding: 14px 0 0 !important;
    overflow: hidden !important;
  }

  .settings-unified .version-detail-header {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 10px;
  }

  .settings-unified .version-detail-header :global(.btn) {
    width: fit-content;
  }

  .settings-unified .diff-table {
    min-height: 0;
    max-height: none;
    overflow: auto;
    scrollbar-gutter: stable;
  }

  .settings-unified .diff-row {
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 7px !important;
    padding: 12px 0 !important;
    border: 0 !important;
    border-bottom: 1px solid var(--settings-line) !important;
  }

  .settings-unified .diff-arrow {
    display: none;
  }

  .settings-unified .diff-value {
    min-width: 0;
    max-height: none;
    display: grid;
    gap: 5px;
    padding: 9px !important;
    border: 1px solid var(--settings-line) !important;
    border-radius: 8px !important;
    background: rgba(239, 246, 248, 0.62) !important;
  }

  .settings-unified .empty-version-state,
  .settings-unified .diff-empty,
  .settings-unified .config-version-error,
  .settings-unified .error-banner,
  .settings-unified .success-banner {
    min-height: 72px;
    display: grid;
    place-items: center;
    margin: 12px 0 0;
    padding: 16px;
    border: 1px dashed var(--settings-line-strong);
    border-radius: 10px;
    background: rgba(239, 246, 248, 0.58);
    color: var(--settings-muted);
    font-size: 12px;
    line-height: 1.5;
    text-align: center;
  }

  .settings-unified .config-version-error,
  .settings-unified .error-banner {
    border-style: solid;
    border-color: rgba(200, 66, 54, 0.26);
    background: rgba(200, 66, 54, 0.075);
    color: #9c352d;
  }

  .settings-unified .success-banner {
    border-style: solid;
    border-color: rgba(4, 150, 111, 0.24);
    background: rgba(4, 150, 111, 0.07);
    color: #047a5d;
  }

  .settings-unified .table-responsive,
  .settings-unified :global(.table-container),
  .settings-unified :global(.scw-table-wrap) {
    width: 100%;
    max-width: 100%;
    overflow: auto;
    border: 1px solid var(--settings-line) !important;
    border-radius: 10px !important;
    background: rgba(248, 251, 252, 0.72) !important;
    box-shadow: none !important;
  }

  .settings-unified table,
  .settings-unified :global(table) {
    width: 100%;
    border-collapse: collapse;
    color: var(--settings-text);
  }

  .settings-unified th,
  .settings-unified :global(th) {
    background: rgba(235, 243, 246, 0.82) !important;
    color: var(--settings-muted) !important;
    font-size: 11px !important;
    font-weight: 780 !important;
  }

  .settings-unified td,
  .settings-unified th,
  .settings-unified :global(td),
  .settings-unified :global(th) {
    padding: 11px 12px !important;
    border: 0 !important;
    border-bottom: 1px solid var(--settings-line) !important;
    vertical-align: middle;
  }

  .settings-unified tbody tr:last-child td,
  .settings-unified :global(tbody tr:last-child td) {
    border-bottom: 0 !important;
  }

  .settings-unified tbody tr:hover td,
  .settings-unified :global(tbody tr:hover td) {
    background: rgba(0, 143, 150, 0.035) !important;
  }

  .settings-unified .policy-workbench,
  .settings-unified .policy-grid {
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    align-items: start;
    gap: 18px;
    margin: 0;
  }

  .settings-unified .policy-workbench.refined {
    grid-template-columns: minmax(0, 1fr);
  }

  .settings-unified .policy-task-toolbar {
    min-width: 0;
    min-height: 40px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding-bottom: 12px;
    border-bottom: 1px solid var(--settings-line);
  }

  .settings-unified .policy-workspace-summary {
    min-width: 0;
    display: flex;
    align-items: baseline;
    gap: 10px;
    flex-wrap: wrap;
    color: var(--settings-muted);
    font-size: 11px;
  }

  .settings-unified .policy-workspace-summary .audit-kicker {
    color: var(--settings-accent-strong);
  }

  .settings-unified .policy-template-strip {
    margin: 0;
  }

  .settings-unified .policy-table-wrap,
  .settings-unified .decision-log-list {
    max-height: min(58vh, 560px);
    overflow: auto;
    scrollbar-gutter: stable;
  }

  #settings-unified-root.settings-unified .settings-data-scroll {
    max-height: min(68dvh, 720px);
    overflow: auto;
    overscroll-behavior: contain;
    scrollbar-gutter: stable;
  }

  #settings-unified-root.settings-unified .permission-tree-scroll {
    padding: 0 14px;
    border: 1px solid var(--settings-line);
    border-radius: 10px;
    background: rgba(248, 251, 252, 0.54);
  }

  #settings-unified-root.settings-unified .settings-data-scroll thead th {
    position: sticky;
    top: 0;
    z-index: 2;
  }

  .settings-unified .action-chip-grid {
    max-height: 220px;
    overflow: auto;
    padding: 4px;
    border: 1px solid var(--settings-line);
    border-radius: 10px;
    background: rgba(239, 246, 248, 0.44);
    scrollbar-gutter: stable;
  }

  .settings-unified .action-chip-grid.compact {
    max-height: 180px;
  }

  #settings-unified-root.settings-unified .policy-panel,
  .settings-unified .permission-branch,
  .settings-unified .group-coverage-card {
    min-width: 0;
    margin: 0 !important;
    padding: 16px 0 !important;
    border: 0 !important;
    border-bottom: 1px solid var(--settings-line) !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  #settings-unified-root.settings-unified .policy-panel {
    padding-top: 4px !important;
    border-bottom: 0 !important;
  }

  #settings-unified-root.settings-unified .decision-card,
  #settings-unified-root.settings-unified .decision-log-item {
    min-width: 0;
    margin: 0 !important;
    padding: 13px 14px !important;
    border: 1px solid var(--settings-line) !important;
    border-radius: 10px !important;
    box-shadow: none !important;
  }

  #settings-unified-root.settings-unified .decision-card.allowed,
  #settings-unified-root.settings-unified .decision-log-item.allowed {
    border-color: rgba(4, 150, 111, 0.24) !important;
    background: rgba(4, 150, 111, 0.07) !important;
  }

  #settings-unified-root.settings-unified .decision-card.denied,
  #settings-unified-root.settings-unified .decision-log-item.denied {
    border-color: rgba(200, 66, 54, 0.24) !important;
    background: rgba(200, 66, 54, 0.07) !important;
  }

  .settings-unified .policy-builder-grid,
  .settings-unified .policy-form-grid {
    gap: 14px;
  }

  #settings-unified-root.settings-unified .choice-card,
  #settings-unified-root.settings-unified .segmented-pills button,
  #settings-unified-root.settings-unified .suggestion-pills button,
  #settings-unified-root.settings-unified .action-chip-grid button,
  .settings-unified .permission-group-toggles button,
  #settings-unified-root.settings-unified .policy-template-strip button,
  #settings-unified-root.settings-unified .toggle-pill,
  #settings-unified-root.settings-unified .priority-stepper button,
  #settings-unified-root.settings-unified .priority-stepper strong {
    border: 1px solid var(--settings-line) !important;
    border-radius: 8px !important;
    background: rgba(248, 251, 252, 0.68) !important;
    color: var(--settings-text) !important;
    box-shadow: none !important;
  }

  #settings-unified-root.settings-unified .choice-card:hover,
  #settings-unified-root.settings-unified .segmented-pills button:hover,
  #settings-unified-root.settings-unified .suggestion-pills button:hover,
  #settings-unified-root.settings-unified .action-chip-grid button:hover,
  .settings-unified .permission-group-toggles button:hover,
  #settings-unified-root.settings-unified .policy-template-strip button:hover {
    border-color: rgba(0, 143, 150, 0.26) !important;
    background: var(--settings-accent-soft) !important;
    transform: none !important;
  }

  #settings-unified-root.settings-unified .choice-card.active,
  #settings-unified-root.settings-unified .segmented-pills button.active,
  #settings-unified-root.settings-unified .suggestion-pills button.active,
  #settings-unified-root.settings-unified .action-chip-grid button.active {
    border-color: rgba(0, 143, 150, 0.3) !important;
    background: var(--settings-accent-soft) !important;
    color: var(--settings-accent-strong) !important;
  }

  #settings-unified-root.settings-unified .choice-card.effect-allow.active {
    border-color: rgba(4, 150, 111, 0.34) !important;
    background: rgba(4, 150, 111, 0.09) !important;
    color: #047a5d !important;
  }

  #settings-unified-root.settings-unified .choice-card.effect-deny.active {
    border-color: rgba(200, 66, 54, 0.32) !important;
    background: rgba(200, 66, 54, 0.085) !important;
    color: #9c352d !important;
  }

  .settings-unified input,
  .settings-unified .custom-input,
  .settings-unified .custom-textarea,
  .settings-unified .dropdown-trigger-btn,
  .settings-unified :global(input),
  .settings-unified :global(textarea),
  .settings-unified :global(select) {
    min-height: 38px;
    border: 1px solid var(--settings-line-strong) !important;
    border-radius: 8px !important;
    background: rgba(250, 253, 254, 0.84) !important;
    color: var(--settings-ink) !important;
    box-shadow: none !important;
  }

  .settings-unified input::placeholder,
  .settings-unified :global(input::placeholder),
  .settings-unified :global(textarea::placeholder) {
    color: #65778a !important;
    opacity: 1 !important;
  }

  .settings-unified input:focus,
  .settings-unified .custom-input:focus,
  .settings-unified .custom-textarea:focus,
  .settings-unified :global(input:focus),
  .settings-unified :global(textarea:focus),
  .settings-unified :global(select:focus) {
    outline: none !important;
    border-color: rgba(0, 143, 150, 0.66) !important;
    box-shadow: 0 0 0 3px rgba(0, 143, 150, 0.11) !important;
  }

  .settings-unified button:focus-visible,
  .settings-unified :global(button:focus-visible),
  .settings-unified [tabindex]:focus-visible {
    outline: 2px solid rgba(0, 143, 150, 0.28) !important;
    outline-offset: 2px;
  }

  .settings-unified .user-avatar {
    width: 34px;
    height: 34px;
    border: 1px solid var(--settings-line);
    background: var(--settings-inset);
  }

  .settings-unified .membership-badge,
  .settings-unified .action-badge,
  .settings-unified .actor-tag,
  .settings-unified .group-title-label {
    border-radius: 999px !important;
    box-shadow: none !important;
  }

  .settings-unified :global(.gitlab-workbench) {
    display: grid;
    gap: 20px;
    color: var(--settings-text) !important;
  }

  .settings-unified :global(.gitlab-workbench .state-banner) {
    border: 1px solid var(--settings-line) !important;
    border-radius: 10px !important;
    box-shadow: none !important;
  }

  .settings-unified :global(.gitlab-workbench .overview-grid),
  .settings-unified :global(.scw-read-grid) {
    align-items: stretch;
  }

  .settings-unified :global(.gitlab-workbench .overview-row),
  .settings-unified :global(.scw-read-item) {
    min-height: 66px;
    align-content: center;
  }

  .modal-overlay {
    background: rgba(18, 31, 43, 0.28) !important;
    backdrop-filter: blur(10px) saturate(112%) !important;
    -webkit-backdrop-filter: blur(10px) saturate(112%) !important;
  }

  .modal-card {
    border: 1px solid rgba(255, 255, 255, 0.78) !important;
    border-radius: 14px !important;
    background: rgba(249, 252, 253, 0.94) !important;
    color: var(--wa-text-main, #293847) !important;
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.88),
      0 24px 58px rgba(20, 38, 51, 0.18) !important;
    backdrop-filter: blur(22px) saturate(124%);
    -webkit-backdrop-filter: blur(22px) saturate(124%);
  }

  .modal-header,
  .modal-footer {
    background: rgba(242, 248, 249, 0.58) !important;
    border-color: rgba(83, 108, 130, 0.16) !important;
  }

  @container (max-width: 1120px) {
    #settings-unified-root.settings-unified .settings-workbench-grid {
      grid-template-columns: minmax(0, 1fr) !important;
    }

    #settings-unified-root.settings-unified .settings-audit-pane {
      position: relative !important;
      top: auto !important;
      height: min(680px, calc(100dvh - 96px)) !important;
      max-height: 680px !important;
      overflow: hidden !important;
    }

    #settings-unified-root.settings-unified .config-audit-panel {
      height: 100% !important;
      max-height: none !important;
      overflow: hidden !important;
    }

    .settings-unified .version-layout,
    .settings-unified .settings-audit-pane .version-layout {
      grid-template-columns: minmax(220px, 0.68fr) minmax(0, 1.32fr) !important;
      grid-template-rows: minmax(0, 1fr) !important;
      height: 100% !important;
    }

    .settings-unified .version-list {
      max-height: 420px;
      border-right: 1px solid var(--settings-line) !important;
      border-bottom: 0 !important;
      padding: 6px 10px 6px 0 !important;
    }

    .settings-unified .version-detail {
      padding: 10px 0 0 16px !important;
    }

    .settings-unified .diff-table {
      max-height: 420px;
    }
  }

  @media (max-width: 1120px) {
    .settings-unified .settings-content-header {
      grid-template-columns: minmax(0, 1fr);
      gap: 14px;
    }

    .settings-unified .settings-content-facts div:first-child {
      border-left: 0 !important;
    }
  }

  @media (max-width: 860px) {
    #settings-unified-root.settings-unified .settings-workbench-grid {
      grid-template-columns: minmax(0, 1fr) !important;
    }

    #settings-unified-root.settings-unified .settings-audit-pane {
      position: relative !important;
      top: auto !important;
      height: min(700px, calc(100dvh - 80px)) !important;
      max-height: 700px !important;
      overflow: hidden !important;
    }

    #settings-unified-root.settings-unified .config-audit-panel {
      height: 100% !important;
      max-height: none !important;
      overflow: hidden !important;
    }

    .settings-unified .policy-workbench,
    .settings-unified .policy-workbench.refined,
    .settings-unified .policy-grid {
      grid-template-columns: minmax(0, 1fr);
    }
  }

  @media (max-width: 760px) {
    .settings-unified {
      padding-bottom: 18px;
    }

    .settings-unified .settings-content-shell {
      gap: 12px;
    }

    .settings-unified .settings-context-panel,
    .settings-unified .settings-primary-pane,
    .settings-unified .settings-audit-pane {
      border-radius: 12px;
    }

    .settings-unified .settings-context-topline {
      min-height: 44px;
      align-items: flex-start;
      flex-direction: column;
      gap: 6px;
      padding: 9px 14px;
    }

    .settings-unified .settings-breadcrumb-bar {
      width: 100%;
    }

    .settings-unified .breadcrumb-meta {
      width: 100%;
      justify-content: space-between;
    }

    .settings-unified .settings-content-header {
      min-height: 0;
      padding: 14px !important;
    }

    .settings-unified .settings-content-header h1 {
      font-size: 20px;
    }

    .settings-unified .settings-content-facts {
      grid-template-columns: minmax(0, 1fr);
    }

    .settings-unified .settings-content-facts div,
    .settings-unified .settings-content-facts div:first-child {
      min-height: 48px;
      padding: 9px 0;
      border-left: 0 !important;
      border-top: 1px solid var(--settings-line) !important;
    }

    .settings-unified .settings-primary-pane,
    .settings-unified .config-audit-panel {
      padding: 14px !important;
    }

    .settings-unified .version-layout,
    .settings-unified .settings-audit-pane .version-layout {
      display: grid !important;
      grid-template-columns: minmax(0, 1fr) !important;
      grid-template-rows: auto auto !important;
      overflow: visible;
    }

    .settings-unified .version-list {
      max-height: 220px;
      border-right: 0 !important;
      border-bottom: 1px solid var(--settings-line) !important;
      padding: 6px 0 8px !important;
    }

    .settings-unified .version-detail {
      padding: 14px 0 0 !important;
      overflow: visible !important;
    }

    .settings-unified .diff-table {
      max-height: 420px;
    }

    .settings-unified .card-header,
    .settings-unified .config-audit-header,
    .settings-unified :global(.scw-header),
    .settings-unified :global(.overview-header) {
      display: grid !important;
      grid-template-columns: minmax(0, 1fr) !important;
      gap: 10px;
    }

    .settings-unified .policy-builder-grid,
    .settings-unified .policy-form-grid,
    .settings-unified .policy-template-strip,
    .settings-unified .choice-row {
      grid-template-columns: minmax(0, 1fr) !important;
    }

    .settings-unified .policy-task-toolbar {
      align-items: stretch;
      flex-direction: column;
    }

    .settings-unified .priority-stepper strong {
      min-height: 44px;
      display: inline-grid;
      place-items: center;
    }

    #settings-unified-root.settings-unified .settings-task-tabs button {
      min-height: 44px !important;
    }

    .settings-unified button:not([role='switch']),
    .settings-unified input,
    .settings-unified .custom-input,
    .settings-unified .dropdown-trigger-btn,
    .settings-unified :global(button:not([role='switch'])),
    .settings-unified :global(input),
    .settings-unified :global(select) {
      min-height: 44px;
    }

    .settings-unified :global(button[role='switch']) {
      min-height: 24px;
    }

    .settings-unified .policy-actions,
    .settings-unified .actions-cell,
    .settings-unified :global(.scw-actions),
    .settings-unified :global(.scw-section-actions) {
      width: 100%;
      align-items: stretch;
      flex-direction: column;
    }

    .settings-unified .policy-actions :global(.btn),
    .settings-unified :global(.scw-actions .btn),
    .settings-unified :global(.scw-section-actions .btn) {
      width: 100%;
    }
  }

  /* The page may scroll for long forms; audit data stays bounded inside the aligned card. */
  @media (min-width: 1281px) {
    #settings-unified-root.settings-unified .settings-workbench-grid {
      align-items: stretch !important;
    }

    #settings-unified-root.settings-unified .settings-primary-pane {
      align-self: stretch !important;
      height: auto !important;
    }

    #settings-unified-root.settings-unified .settings-audit-pane {
      position: relative !important;
      top: auto !important;
      align-self: stretch !important;
      height: auto !important;
      max-height: none !important;
      overflow: hidden !important;
    }

    #settings-unified-root.settings-unified .config-audit-panel {
      height: 100% !important;
      max-height: none !important;
      overflow: hidden !important;
    }

    .settings-unified .version-layout,
    .settings-unified .settings-audit-pane .version-layout {
      height: 100% !important;
      min-height: 0;
      grid-template-rows: minmax(96px, 184px) minmax(0, 1fr) !important;
      overflow: hidden !important;
    }

    .settings-unified .version-list {
      min-height: 0;
      max-height: 184px !important;
      overflow: auto !important;
      overscroll-behavior: contain;
      scrollbar-gutter: stable;
    }

    .settings-unified .version-detail {
      min-height: 0;
      max-height: none !important;
      overflow: hidden !important;
    }

    .settings-unified .diff-table {
      min-height: 0;
      max-height: none !important;
      overflow: auto !important;
      overscroll-behavior: contain;
      scrollbar-gutter: stable;
    }
  }

  @media (prefers-reduced-transparency: reduce) {
    .settings-unified .settings-context-panel,
    .settings-unified .settings-primary-pane,
    .settings-unified .settings-audit-pane,
    .modal-card {
      background: #f8fbfc !important;
      backdrop-filter: none !important;
      -webkit-backdrop-filter: none !important;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .settings-unified *,
    .settings-unified *::before,
    .settings-unified *::after,
    .modal-overlay *,
    .modal-overlay *::before,
    .modal-overlay *::after {
      scroll-behavior: auto !important;
      transition-duration: 0.01ms !important;
      animation-duration: 0.01ms !important;
      animation-iteration-count: 1 !important;
    }
  }
</style>
