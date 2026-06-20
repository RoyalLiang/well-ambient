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

  let activeSection: 'gitlab' | 'feishu' | 'jira' | 'ai' | 'users' | 'matrix' | 'audit' = 'gitlab';

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

  let users: User[] = [];
  let groups: Group[] = [];
  let auditLogs: AuditLog[] = [];

  // Atomic permissions metadata
  const permissionMeta = [
    { code: 'config:read', name: '查看集成配置', desc: '查看第三方系统配置密钥及连通状态' },
    { code: 'config:write', name: '修改集成配置', desc: '修改 GitLab、飞书、Jira 以及 AI 配置参数' },
    { code: 'users:read', name: '查看权限与日志', desc: '查看注册用户列表、用户组权限及安全审计痕迹' },
    { code: 'users:write', name: '权限组及成员分配', desc: '创建组、调整权限组的权限集、为用户指定角色和 Scope' },
    { code: 'users:transfer_super_admin', name: '转让超级管理员', desc: '将系统最高管理权转让给他人' },
    { code: 'dashboard:read', name: '查看协同看板', desc: '有权查看主界面协同看板、AI 需求解构日志与全部任务看板' },
    { code: 'demands:read', name: '查看需求看板', desc: '有权查看需求看板泳道及其排期卡片' },
    { code: 'decision:read', name: '查看决策大屏', desc: '有权查看红区卡点诊断盘与决策会议大屏' }
  ];

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
    fetchStatus();
    fetchUsers();
    fetchGroups();
    fetchAuditLogs();

    statusIntervalId = setInterval(fetchStatus, 5000);

    const handleFocus = (e: any) => {
      if (e.detail && ['gitlab', 'feishu', 'jira', 'ai'].includes(e.detail)) {
        switchSection(e.detail);
      }
    };
    window.addEventListener('focus-settings-section', handleFocus);
    return () => {
      window.removeEventListener('focus-settings-section', handleFocus);
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
      <span class="version-label">Category settings</span>
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
        <GitLabConfig config={globalConfig.gitlab} on:save={handleSaveConfig} on:close={handleConfigClose} {saveError} {saving} saveSuccess={saveSuccess && saveSuccessKey === 'gitlab'} />
      </div>
    {:else if activeSection === 'feishu'}
      <div class="section-card">
        <FeishuConfig config={globalConfig.feishu} on:save={handleSaveConfig} on:close={handleConfigClose} {saveError} {saving} saveSuccess={saveSuccess && saveSuccessKey === 'feishu'} />
      </div>
    {:else if activeSection === 'jira'}
      <div class="section-card">
        <JiraConfig config={globalConfig.jira} on:save={handleSaveConfig} on:close={handleConfigClose} {saveError} {saving} saveSuccess={saveSuccess && saveSuccessKey === 'jira'} />
      </div>
    {:else if activeSection === 'ai'}
      <div class="section-card">
        <AIConfig config={globalConfig.ai} on:save={handleSaveConfig} on:close={handleConfigClose} {saveError} {saving} saveSuccess={saveSuccess && saveSuccessKey === 'ai'} />
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
          <label>目标成员账户</label>
          <input type="text" value={membershipTargetUser} disabled class="input-disabled font-mono" />
        </div>
        <div class="field-item">
          <label>分配目标用户组</label>
          <select bind:value={membershipSelectedGroup} class="custom-select font-mono">
            {#each groups as g}
              {#if g.name !== 'super_admin'}
                <option value={g.name}>{g.displayName} ({g.name})</option>
              {/if}
            {/each}
          </select>
        </div>
        <div class="field-item">
          <label>分配作用域等级 (Scope)</label>
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
            <label>特定作用域仓库标识 (ProjectID/RepoName)</label>
            <input type="text" bind:value={membershipScopeID} placeholder="输入关联的代码仓名称，如 frontend-dashboard" class="custom-input font-mono" />
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
          <label>目标接收者账户</label>
          <input type="text" value={transferTargetUsername} disabled class="input-disabled font-mono" />
        </div>
        <div class="field-item">
          <label>请输入目标用户的真实姓名以确认安全转让</label>
          <input 
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
          <label>组代码名称 (英文小写下划线，作为系统唯一 Key)</label>
          <input type="text" bind:value={newGroupName} placeholder="如 developer_lead" class="custom-input font-mono" />
        </div>
        <div class="field-item">
          <label>显示名称</label>
          <input type="text" bind:value={newGroupDisplayName} placeholder="如 研发主导组" class="custom-input" />
        </div>
        <div class="field-item">
          <label>组描述说明</label>
          <textarea bind:value={newGroupDescription} placeholder="请输入该组的使用场景和权限划分意图" class="custom-textarea" rows="3"></textarea>
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
