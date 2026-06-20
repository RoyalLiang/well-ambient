<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Button from './shared/Button.svelte';
  import Alert from './shared/Alert.svelte';
  import GitLabConfig from './config/GitLabConfig.svelte';
  import FeishuConfig from './config/FeishuConfig.svelte';
  import JiraConfig from './config/JiraConfig.svelte';
  import AIConfig from './config/AIConfig.svelte';

  export let currentUserRole = 'member';
  export let currentUserEmail = '';
  export let currentUserPermissions: string[] = [];

  let serverStatus: 'online' | 'offline' | 'warning' = 'online';
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
  $: gitlabStatus = globalConfig.gitlab.base_url ? serverStatus : 'offline';
  $: feishuStatus = !globalConfig.feishu.app_id 
    ? 'offline' 
    : ((globalConfig.feishu.bot?.enabled || globalConfig.feishu.bitable?.enabled) ? 'online' : 'warning');
  $: jiraStatus = globalConfig.jira?.enabled ? 'online' : 'offline';
  $: aiStatus = globalConfig.ai?.enabled ? 'online' : 'offline';

  let activeSection: 'gitlab' | 'feishu' | 'jira' | 'ai' | 'users' | 'matrix' | 'policies' | 'audit' = 'gitlab';

  interface GlobalConfig {
    server: { host: string; port: number };
    gitlab: {
      base_url: string;
      secret_token: string;
      repos: Array<{ name: string; path: string; project_id: string }>;
    };
    feishu: {
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
      endpoint_type: 'completions',
      api_token: '',
      model: '',
      project_architecture: '',
      delivery_workflow: '',
      implemented_features: '',
      estimation_guidelines: '',
      default_work_hours_per_day: 8
    },
    jira: { enabled: false, base_url: '', username: '', api_token: '', sync_projects: [], sync_users: [], sync_statuses: [], custom_jql: '' }
  };

  let saving = false;
  let saveError = '';
  let saveSuccess = false;
  let saveSuccessKey: string | null = null;
  let configVersions: ConfigVersion[] = [];
  let selectedConfigVersionID: number | null = null;
  let configVersionError = '';
  let rollbackLoadingID: number | null = null;

  $: selectedConfigVersion = configVersions.find(v => v.id === selectedConfigVersionID) || configVersions[0] || null;
  $: isIntegrationSection = ['gitlab', 'feishu', 'jira', 'ai'].includes(activeSection);

  function switchSection(section: typeof activeSection) {
    activeSection = section;
    saveError = '';
    saveSuccess = false;
    saveSuccessKey = null;
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

  let users: User[] = [];
  let groups: Group[] = [];
  let auditLogs: AuditLog[] = [];
  let authorizationPolicies: AuthorizationPolicy[] = [];
  let authorizationAuditLogs: AuthorizationAuditLog[] = [];
  let authorizationPolicyError = '';
  let authorizationPolicySuccess = '';
  let authorizationPolicySaving = false;
  let authorizationDecision: AuthorizationDecision | null = null;
  let authorizationExplainError = '';

  // Atomic permissions metadata
  const permissionMeta = [
    { code: 'config:read', name: '查看集成配置', desc: '查看第三方系统配置密钥及连通状态' },
    { code: 'config:write', name: '修改集成配置', desc: '修改 GitLab、飞书、Jira 以及 AI 配置参数' },
    { code: 'users:read', name: '查看权限与日志', desc: '查看注册用户列表、用户组权限及安全审计痕迹' },
    { code: 'users:write', name: '权限组及成员分配', desc: '创建组、调整权限组的权限集、为用户指定角色和 Scope' },
    { code: 'users:transfer_super_admin', name: '转让超级管理员', desc: '将系统最高管理权转让给他人' },
    { code: 'dashboard:read', name: '查看协同看板', desc: '有权查看主界面协同看板、AI 需求解构日志与全部任务看板' },
    { code: 'demands:read', name: '查看需求看板', desc: '有权查看需求看板泳道及其排期卡片' },
    { code: 'decision:read', name: '查看决策大屏', desc: '有权查看红区卡点诊断盘与决策会议大屏' },
    { code: 'ai_context:read', name: '查看上下文事实', desc: '查看 AI 需求解构使用的结构化上下文事实' },
    { code: 'ai_context:write', name: '管理上下文事实', desc: '新增、更新、停用 AI 上下文事实' },
    { code: 'ai_context:preview', name: '预览上下文包', desc: '按需求文本预览上下文包选择结果' },
    { code: 'policies:read', name: '查看授权策略', desc: '查看策略化 allow/deny 权限规则' },
    { code: 'policies:write', name: '管理授权策略', desc: '新增或更新策略化权限规则' },
    { code: 'authorization_audit:read', name: '授权决策审计', desc: '查看拒绝和高风险授权决策日志' }
  ];

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
      const res = await fetch('/api/config/versions?limit=12');
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
    const version = configVersions.find(v => (v.changed_sections || []).includes(section) || v.source === 'bootstrap-file');
    return version?.created_at || '';
  }

  function formatDateTime(value: string) {
    if (!value) return '暂无版本记录';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString();
  }

  function formatDiffValue(value: any) {
    if (value === undefined || value === null || value === '') return '空';
    if (typeof value === 'string') return value;
    return JSON.stringify(value);
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

  async function handleSaveConfig(event: CustomEvent<{ key: string; data: any }>) {
    const { key, data } = event.detail;
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
        saveSuccess = true;
        saveSuccessKey = key;
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
      if (res.ok) groups = await res.json();
    } catch (e) {
      console.error(e);
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

  function openAddMembership(username: string) {
    membershipTargetUser = username;
    membershipSelectedGroup = groups[0]?.name || '';
    membershipScope = 'global';
    membershipScopeID = '';
    membershipError = '';
    membershipSuccess = '';
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

  async function removeMembership(username: string, membership: UserMembership) {
    if (!confirm(`确定要移除该用户属于「${membership.group_display_name} (${membership.scope === 'global' ? '全局' : '仓库:' + membership.scope_id})」的权限角色吗？`)) return;
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
      if (res.ok) {
        fetchUsers();
      } else {
        const data = await res.json();
        alert(data.message || '移除失败');
      }
    } catch (e: any) {
      alert('请求失败: ' + e.message);
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
    const targetUser = users.find(u => u.username === transferTargetUsername);
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

  async function createCustomGroup() {
    createGroupError = '';
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
        showCreateGroupModal = false;
        newGroupName = '';
        newGroupDisplayName = '';
        newGroupDescription = '';
        fetchGroups();
      } else {
        createGroupError = data.message || '创建用户组失败';
      }
    } catch (e: any) {
      createGroupError = '请求错误: ' + e.message;
    }
  }

  onMount(() => {
    fetchConfig();
    fetchConfigVersions();
    fetchStatus();
    fetchUsers();
    fetchGroups();
    fetchAuditLogs();
    fetchAuthorizationPolicies();
    fetchAuthorizationAuditLogs();

    statusIntervalId = setInterval(fetchStatus, 5000);

    const handleFocus = (e: any) => {
      if (e.detail && ['gitlab', 'feishu', 'jira', 'ai'].includes(e.detail)) {
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
</script>

<div class="settings-container">
  <!-- Left Category Navigation Sidebar -->
  <aside class="settings-sidebar font-mono">
    <div class="sidebar-header">
      <h3>⚙️ 系统设置</h3>
      <span class="version-label">Category settings · {currentUserRole}</span>
    </div>
    <nav class="sidebar-nav">
      <div class="nav-group">
        <span class="group-title">集成设置</span>
        <button class="nav-item {activeSection === 'gitlab' ? 'active' : ''}" on:click={() => switchSection('gitlab')}>
          <span>🔌 GitLab 仓库</span>
          <span class="status-indicator indicator-{gitlabStatus}"></span>
        </button>
        <button class="nav-item {activeSection === 'feishu' ? 'active' : ''}" on:click={() => switchSection('feishu')}>
          <span>🤖 飞书消息同步</span>
          <span class="status-indicator indicator-{feishuStatus}"></span>
        </button>
        <button class="nav-item {activeSection === 'jira' ? 'active' : ''}" on:click={() => switchSection('jira')}>
          <span>📝 Jira 服务关联</span>
          <span class="status-indicator indicator-{jiraStatus}"></span>
        </button>
        <button class="nav-item {activeSection === 'ai' ? 'active' : ''}" on:click={() => switchSection('ai')}>
          <span>🧠 需求解构引擎</span>
          <span class="status-indicator indicator-{aiStatus}"></span>
        </button>
      </div>

      {#if currentUserPermissions.includes('users:read')}
        <div class="nav-group">
          <span class="group-title">安全与授权</span>
          <button class="nav-item {activeSection === 'users' ? 'active' : ''}" on:click={() => switchSection('users')}>
            👥 成员角色管理
          </button>
          <button class="nav-item {activeSection === 'matrix' ? 'active' : ''}" on:click={() => switchSection('matrix')}>
            🔒 权限矩阵矩阵
          </button>
          <button class="nav-item {activeSection === 'policies' ? 'active' : ''}" on:click={() => switchSection('policies')}>
            🧭 策略化授权
          </button>
          <button class="nav-item {activeSection === 'audit' ? 'active' : ''}" on:click={() => switchSection('audit')}>
            📋 审计安全日志
          </button>
        </div>
      {/if}
    </nav>
  </aside>

  <!-- Right Section Panel -->
  <main class="settings-main">
    {#if activeSection === 'gitlab'}
      <div class="section-card">
        <GitLabConfig config={globalConfig.gitlab} lastUpdated={sectionLastUpdated('gitlab')} on:save={handleSaveConfig} on:close={handleConfigClose} {saveError} {saving} saveSuccess={saveSuccess && saveSuccessKey === 'gitlab'} />
      </div>
    {:else if activeSection === 'feishu'}
      <div class="section-card">
        <FeishuConfig config={globalConfig.feishu} lastUpdated={sectionLastUpdated('feishu')} on:save={handleSaveConfig} on:close={handleConfigClose} {saveError} {saving} saveSuccess={saveSuccess && saveSuccessKey === 'feishu'} />
      </div>
    {:else if activeSection === 'jira'}
      <div class="section-card">
        <JiraConfig config={globalConfig.jira} lastUpdated={sectionLastUpdated('jira')} on:save={handleSaveConfig} on:close={handleConfigClose} {saveError} {saving} saveSuccess={saveSuccess && saveSuccessKey === 'jira'} />
      </div>
    {:else if activeSection === 'ai'}
      <div class="section-card">
        <AIConfig config={globalConfig.ai} lastUpdated={sectionLastUpdated('ai')} on:save={handleSaveConfig} on:close={handleConfigClose} {saveError} {saving} saveSuccess={saveSuccess && saveSuccessKey === 'ai'} />
      </div>
    {:else if activeSection === 'users'}
      <div class="section-card">
        <div class="card-header">
          <h2>👥 成员角色与项目隔离 (Scope) 管理</h2>
          <p>查看并管理注册成员所属的权限组，在此可配置细粒度的 GitLab 仓库作用域隔离（Scope）。</p>
        </div>

        <div class="table-responsive">
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
                      {#each user.memberships as membership}
                        <div class="membership-badge badge-{membership.group_name}">
                          <span class="badge-role">{membership.group_display_name}</span>
                          <span class="badge-scope">
                            {membership.scope === 'global' ? '🌎 全局' : `📂 ${membership.scope_id}`}
                          </span>
                          {#if currentUserPermissions.includes('users:write') && membership.group_name !== 'super_admin'}
                            <button class="remove-membership-btn" on:click={() => removeMembership(user.username, membership)} title="移除组身份">&times;</button>
                          {/if}
                        </div>
                      {:else}
                        <span class="text-muted text-xs font-mono">⚠️ 暂未分配用户组（无管理权限）</span>
                      {/each}
                    </div>
                  </td>
                  <td style="text-align: right;">
                    <div class="actions-cell">
                      {#if currentUserPermissions.includes('users:write')}
                        <Button size="small" variant="ghost" on:click={() => openAddMembership(user.username)}>
                          ➕ 分配组
                        </Button>
                      {/if}
                      {#if currentUserPermissions.includes('users:transfer_super_admin') && user.username !== currentUserEmail}
                        <Button size="small" variant="danger" on:click={() => openTransferAdmin(user.username)}>
                          👑 转让超管
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
            <h2>🔒 可视化权限矩阵配置</h2>
            <p>管理系统中不同用户组对应的原子权限绑定关系。系统权限实时刷新。</p>
          </div>
          {#if currentUserPermissions.includes('users:write')}
            <Button variant="ghost" on:click={() => showCreateGroupModal = true}>
              🛠️ 创建自定义组
            </Button>
          {/if}
        </div>

        <div class="table-responsive">
          <table class="matrix-table">
            <thead>
              <tr>
                <th>用户组及描述</th>
                {#each permissionMeta as perm}
                  <th class="rotate-header" title={perm.desc}>{perm.name}</th>
                {/each}
              </tr>
            </thead>
            <tbody>
              {#each groups as group}
                <tr>
                  <td>
                    <div class="group-info-cell">
                      <span class="group-title-label font-bold badge-{group.name}">{group.displayName}</span>
                      <p class="group-desc-para">{group.description}</p>
                    </div>
                  </td>
                  {#each permissionMeta as perm}
                    <td style="text-align: center;">
                      {#if group.name === 'super_admin'}
                        <input type="checkbox" checked disabled class="matrix-checkbox-disabled" />
                      {:else}
                        <input 
                          type="checkbox" 
                          checked={group.permissions.includes(perm.code)} 
                          disabled={!currentUserPermissions.includes('users:write')} 
                          on:change={() => togglePermissionInMatrix(group, perm.code)}
                          class="matrix-checkbox" 
                        />
                      {/if}
                    </td>
                  {/each}
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {:else if activeSection === 'policies'}
      <div class="section-card">
        <div class="card-header flex-header">
          <div>
            <h2>🧭 策略化授权控制台</h2>
            <p>在原有用户组权限矩阵之上追加 allow/deny 策略，按主体、动作、资源与作用域解释最终授权结果。</p>
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

        <div class="policy-workbench">
          <section class="policy-panel">
            <div class="policy-panel-header">
              <span class="audit-kicker font-mono">Create Policy</span>
              <h3>新增授权策略</h3>
            </div>
            <div class="policy-form-grid">
              <div class="field-item">
                <label for="policy-effect">Effect</label>
                <select id="policy-effect" bind:value={policyForm.effect} class="custom-select font-mono">
                  <option value="deny">deny</option>
                  <option value="allow">allow</option>
                </select>
              </div>
              <div class="field-item">
                <label for="policy-subject-type">Subject Type</label>
                <select id="policy-subject-type" bind:value={policyForm.subject_type} class="custom-select font-mono">
                  <option value="group">group</option>
                  <option value="user">user</option>
                  <option value="any">any</option>
                </select>
              </div>
              <div class="field-item">
                <label for="policy-subject-id">Subject ID</label>
                <input id="policy-subject-id" bind:value={policyForm.subject_id} class="custom-input font-mono" placeholder="member / admin / user@example.com" />
              </div>
              <div class="field-item">
                <label for="policy-action">Action</label>
                <input id="policy-action" bind:value={policyForm.action} class="custom-input font-mono" placeholder="config:write" />
              </div>
              <div class="field-item">
                <label for="policy-resource-type">Resource Type</label>
                <input id="policy-resource-type" bind:value={policyForm.resource_type} class="custom-input font-mono" placeholder="config / repo / demand" />
              </div>
              <div class="field-item">
                <label for="policy-resource-id">Resource ID</label>
                <input id="policy-resource-id" bind:value={policyForm.resource_id} class="custom-input font-mono" placeholder="可留空或填写具体资源" />
              </div>
              <div class="field-item">
                <label for="policy-scope">Scope</label>
                <select id="policy-scope" bind:value={policyForm.scope} class="custom-select font-mono">
                  <option value="global">global</option>
                  <option value="repo">repo</option>
                </select>
              </div>
              <div class="field-item">
                <label for="policy-scope-id">Scope ID</label>
                <input id="policy-scope-id" bind:value={policyForm.scope_id} class="custom-input font-mono" placeholder="repo 名称，可留空" />
              </div>
              <div class="field-item">
                <label for="policy-priority">Priority</label>
                <input id="policy-priority" type="number" bind:value={policyForm.priority} class="custom-input font-mono" />
              </div>
              <label class="policy-toggle">
                <input type="checkbox" bind:checked={policyForm.enabled} />
                <span>启用策略</span>
              </label>
              <div class="field-item policy-wide">
                <label for="policy-reason">Reason</label>
                <input id="policy-reason" bind:value={policyForm.reason} class="custom-input" placeholder="写清为什么允许或拒绝，便于审计解释" />
              </div>
            </div>
            <div class="policy-actions">
              <Button variant="primary" loading={authorizationPolicySaving} on:click={saveAuthorizationPolicy}>
                保存策略
              </Button>
            </div>
          </section>

          <section class="policy-panel">
            <div class="policy-panel-header">
              <span class="audit-kicker font-mono">Explain Decision</span>
              <h3>授权解释器</h3>
            </div>
            {#if authorizationExplainError}
              <div class="error-banner">{authorizationExplainError}</div>
            {/if}
            <div class="policy-form-grid">
              <div class="field-item policy-wide">
                <label for="explain-username">Username</label>
                <input id="explain-username" bind:value={explainForm.username} class="custom-input font-mono" placeholder="user@example.com" />
              </div>
              <div class="field-item">
                <label for="explain-action">Action</label>
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
                <label for="explain-scope">Scope</label>
                <select id="explain-scope" bind:value={explainForm.scope} class="custom-select font-mono">
                  <option value="global">global</option>
                  <option value="repo">repo</option>
                </select>
              </div>
              <div class="field-item">
                <label for="explain-scope-id">Scope ID</label>
                <input id="explain-scope-id" bind:value={explainForm.scope_id} class="custom-input font-mono" />
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
          </section>
        </div>

        <div class="policy-grid">
          <section class="policy-panel">
            <div class="policy-panel-header">
              <span class="audit-kicker font-mono">Policies</span>
              <h3>策略列表</h3>
            </div>
            <div class="table-responsive policy-table-wrap">
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
          </section>

          <section class="policy-panel">
            <div class="policy-panel-header">
              <span class="audit-kicker font-mono">Decision Audit</span>
              <h3>授权决策审计</h3>
            </div>
            <div class="decision-log-list">
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
          </section>
        </div>
      </div>
    {:else if activeSection === 'audit'}
      <div class="section-card">
        <div class="card-header">
          <h2>📋 安全审计日志痕迹 (Audit Logs)</h2>
          <p>系统自动记录所有敏感配置修改、鉴权登录及用户权限矩阵分配的操作行迹，以便溯源审计。</p>
        </div>

        <div class="table-responsive">
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
                  <td colspan="5" style="text-align: center; color: #64748b; padding: 24px;">📭 暂无敏感操作审计记录</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {/if}

    {#if isIntegrationSection}
      <section class="config-audit-panel">
        <div class="config-audit-header">
          <div>
            <span class="audit-kicker font-mono">Versioned Config</span>
            <h3>配置版本审计与回滚</h3>
          </div>
          <Button size="small" variant="ghost" on:click={fetchConfigVersions}>刷新记录</Button>
        </div>

        {#if configVersionError}
          <div class="config-version-error">{configVersionError}</div>
        {/if}

        {#if configVersions.length === 0}
          <div class="empty-version-state">暂无数据库配置版本。首次保存后会自动生成可审计快照。</div>
        {:else}
          <div class="version-layout">
            <div class="version-list" role="list" aria-label="配置版本">
              {#each configVersions as version}
                <button
                  type="button"
                  class="version-item {selectedConfigVersion?.id === version.id ? 'active' : ''}"
                  on:click={() => selectedConfigVersionID = version.id}
                >
                  <span class="version-title">v{version.version}</span>
                  <span class="version-meta">{formatDateTime(version.created_at)}</span>
                  <span class="version-sections">{(version.changed_sections || []).join(' / ') || '无差异'}</span>
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
                  {#each (selectedConfigVersion.diff || []).slice(0, 8) as diff}
                    <div class="diff-row">
                      <span class="diff-path font-mono">{diff.path}</span>
                      <span class="diff-value before font-mono">{formatDiffValue(diff.before)}</span>
                      <span class="diff-arrow">→</span>
                      <span class="diff-value after font-mono">{formatDiffValue(diff.after)}</span>
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
    {/if}
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
          <div class="error-banner">❌ {membershipError}</div>
        {/if}
        {#if membershipSuccess}
          <div class="success-banner">✅ {membershipSuccess}</div>
        {/if}
        
        <div class="field-item">
          <label for="membership-target-user">目标成员账户</label>
          <input id="membership-target-user" type="text" value={membershipTargetUser} disabled class="input-disabled font-mono" />
        </div>
        <div class="field-item">
          <label for="membership-selected-group">分配目标用户组</label>
          <select id="membership-selected-group" bind:value={membershipSelectedGroup} class="custom-select font-mono">
            {#each groups as g}
              {#if g.name !== 'super_admin'}
                <option value={g.name}>{g.displayName} ({g.name})</option>
              {/if}
            {/each}
          </select>
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

<!-- Modal 2: Transfer Super Admin -->
{#if showTransferModal}
  <div class="modal-overlay">
    <div class="modal-card">
      <div class="modal-header">
        <h3 style="color: #ef4444;">⚠️ 警告：超级管理员控制权转让</h3>
        <button class="close-modal-btn" on:click={() => showTransferModal = false}>&times;</button>
      </div>
      <div class="modal-body">
        {#if transferError}
          <div class="error-banner">❌ {transferError}</div>
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
        <h3>🛠️ 创建自定义组 (Role Group)</h3>
        <button class="close-modal-btn" on:click={() => showCreateGroupModal = false}>&times;</button>
      </div>
      <div class="modal-body">
        {#if createGroupError}
          <div class="error-banner">❌ {createGroupError}</div>
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

<style>
  .settings-container {
    display: flex;
    gap: 32px;
    margin-bottom: 48px;
    min-height: 70vh;
  }

  .settings-sidebar {
    width: 250px;
    flex-shrink: 0;
    background: #0b1329;
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 12px;
    padding: 20px;
    box-shadow: 0 4px 20px -2px rgba(0, 0, 0, 0.5);
    align-self: flex-start;
  }

  .sidebar-header {
    border-bottom: 1px solid rgba(51, 65, 85, 0.6);
    padding-bottom: 12px;
    margin-bottom: 16px;
  }

  .sidebar-header h3 {
    margin: 0 0 4px 0;
    font-size: 1.1rem;
    color: #e2e8f0;
  }

  .version-label {
    font-size: 0.7rem;
    color: #64748b;
  }

  .nav-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: 20px;
  }

  .group-title {
    font-size: 0.75rem;
    color: #475569;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-bottom: 4px;
    padding-left: 8px;
  }

  .nav-item {
    background: transparent;
    border: none;
    color: #94a3b8;
    text-align: left;
    padding: 8px 12px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.85rem;
    transition: all 0.2s ease;
  }

  .nav-item:hover {
    background: rgba(30, 41, 59, 0.5);
    color: #e2e8f0;
    padding-left: 16px;
  }

  .nav-item.active {
    background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
    color: #ffffff;
    font-weight: 700;
    box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
  }

  .settings-main {
    flex-grow: 1;
    min-width: 0;
  }

  .section-card {
    background: #0f172a;
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 12px;
    padding: 24px;
    box-shadow: 0 4px 20px -2px rgba(0, 0, 0, 0.3);
  }

  .config-audit-panel {
    margin-top: 18px;
    background: #0b1329;
    border: 1px solid rgba(51, 65, 85, 0.45);
    border-radius: 10px;
    padding: 18px;
    box-shadow: 0 4px 18px rgba(0, 0, 0, 0.22);
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

  .policy-form-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }

  .policy-wide {
    grid-column: 1 / -1;
  }

  .policy-toggle {
    display: flex;
    align-items: center;
    gap: 8px;
    color: #cbd5e1;
    font-size: 0.85rem;
    align-self: center;
  }

  .policy-toggle input {
    width: 16px;
    height: 16px;
    accent-color: #6366f1;
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
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.42);
    padding-bottom: 14px;
    margin-bottom: 14px;
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
    font-size: 1rem;
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
    grid-template-columns: 240px minmax(0, 1fr);
    gap: 14px;
  }

  .version-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: 360px;
    overflow-y: auto;
    padding-right: 4px;
  }

  .version-list::-webkit-scrollbar {
    width: 8px;
  }

  .version-list::-webkit-scrollbar-track {
    background: rgba(15, 23, 42, 0.55);
    border-radius: 999px;
  }

  .version-list::-webkit-scrollbar-thumb {
    background: rgba(100, 116, 139, 0.65);
    border-radius: 999px;
  }

  .version-item {
    border: 1px solid rgba(51, 65, 85, 0.48);
    background: rgba(15, 23, 42, 0.56);
    color: #cbd5e1;
    border-radius: 8px;
    padding: 10px;
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
    font-size: 0.9rem;
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
    border: 1px solid rgba(51, 65, 85, 0.48);
    background: rgba(15, 23, 42, 0.38);
    border-radius: 8px;
    padding: 12px;
  }

  .version-detail-header {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    align-items: flex-start;
    margin-bottom: 10px;
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
    gap: 8px;
  }

  .diff-row {
    display: grid;
    grid-template-columns: 160px minmax(0, 1fr) 18px minmax(0, 1fr);
    gap: 8px;
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
    max-height: 80px;
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

  /* Matrix table custom styles */
  .rotate-header {
    text-align: center;
    font-size: 0.75rem;
    max-width: 110px;
  }

  .group-info-cell {
    padding: 4px 0;
  }

  .group-title-label {
    display: inline-block;
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

  .matrix-checkbox, .matrix-checkbox-disabled {
    appearance: none;
    -webkit-appearance: none;
    width: 18px;
    height: 18px;
    border: 2px solid #475569;
    border-radius: 4px;
    background: #1e293b;
    position: relative;
    cursor: pointer;
    outline: none;
    transition: all 0.2s ease;
  }

  .matrix-checkbox:checked, .matrix-checkbox-disabled:checked {
    background: #6366f1;
    border-color: #6366f1;
  }

  .matrix-checkbox:checked::after, .matrix-checkbox-disabled:checked::after {
    content: '';
    position: absolute;
    left: 4px;
    top: 0px;
    width: 5px;
    height: 10px;
    border: solid white;
    border-width: 0 2px 2px 0;
    transform: rotate(45deg);
  }

  .matrix-checkbox:hover:not(:disabled) {
    border-color: #6366f1;
    box-shadow: 0 0 8px rgba(99, 102, 241, 0.4);
  }

  .matrix-checkbox-disabled {
    opacity: 0.5;
    cursor: not-allowed;
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

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.4);
    background: #0b1329;
  }

  .modal-header h3 {
    margin: 0;
    font-size: 1.1rem;
    color: #f8fafc;
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

  .status-indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    display: inline-block;
    transition: all 0.3s ease;
  }

  .indicator-online {
    background-color: #10b981;
    box-shadow: 0 0 8px #10b981;
  }

  .indicator-offline {
    background-color: #ef4444;
    box-shadow: 0 0 8px #ef4444;
  }

  .indicator-warning {
    background-color: #f59e0b;
    box-shadow: 0 0 8px #f59e0b;
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
</style>
