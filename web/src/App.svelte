<script lang="ts">
  import { onMount } from 'svelte';
  import CryptoJS from 'crypto-js';
  import TaskKanban from './components/TaskKanban.svelte';
  import Deconstructor from './components/Deconstructor.svelte';
  import ProjectHealthTelemetry from './components/ProjectHealthTelemetry.svelte';
  import DecisionDashboard from './components/DecisionDashboard.svelte';
  import SettingsPanel from './components/SettingsPanel.svelte';
  import DemandKanban from './components/DemandKanban.svelte';
  import ProfilePanel from './components/ProfilePanel.svelte';

  let activeTab = 'decision'; // 'decision' | 'schedule' | 'evidence' | 'settings'

  interface Alert {
    id: number;
    type: string; // delay, git_push, mr_event, ai_review, semantic_linker
    task_id: string;
    title: string;
    assignee: string;
    delay_days: number;
    severity: string;
    status: string;
    message: string;
    link: string;
    created_at: string;
  }

  let showNotifications = false;
  let alerts: Alert[] = [];
  let eventSource: EventSource | null = null;
  let coreMembers = new Set<string>();
  let shakeActive = false;
  let previousUnreadCount = 0;

  // JWT auth states
  let jwtToken = localStorage.getItem('jwt_token') || '';
  let currentUserName = localStorage.getItem('current_user_name') || '';
  let currentUserEmail = localStorage.getItem('current_user_email') || '';
  let currentUserAvatar = localStorage.getItem('current_user_avatar') || '';
  let currentUserRole = localStorage.getItem('current_user_role') || 'member';
  let currentUserDepartment = localStorage.getItem('current_user_department') || '';
  let authDegraded = localStorage.getItem('auth_degraded') === 'true';
  let authDegradedMessage = localStorage.getItem('auth_degraded_message') || '';
  let currentUserPermissions: string[] = [];
  try {
    currentUserPermissions = JSON.parse(localStorage.getItem('current_user_permissions') || '[]');
  } catch (e) {
    currentUserPermissions = [];
  }
  let showUserDropdown = false;

  function normalizeDepartment(department: unknown): string {
    if (typeof department !== 'string') return '';
    const trimmed = department.trim();
    if (!trimmed || trimmed === '未分配' || trimmed === '无部门') return '';
    return trimmed;
  }
  
  // Login form states
  let loginUsernamePrefix = '';
  let loginPassword = '';
  let loginError = '';
  let loginNotice = '';
  let loggingIn = false;

  function hasPermission(p: string): boolean {
    return currentUserPermissions.includes(p);
  }

  // Redirect to first available tab based on permissions
  function autoRedirectTab() {
    if (!jwtToken) return;
    
    // Define tabs and their corresponding required permissions
    const tabPermissions: Record<string, string> = {
      'decision': 'decision:read',
      'schedule': 'demands:read',
      'evidence': 'dashboard:read',
      'settings': 'config:read'
    };

    const requiredPerm = tabPermissions[activeTab];
    if (activeTab === 'settings') {
      if (hasPermission('config:read') || hasPermission('users:read') || hasPermission('kpi:read')) {
        return;
      }
    } else if (requiredPerm && hasPermission(requiredPerm)) {
      return;
    }
    
    // Otherwise look for first allowed tab
    const orderedTabs = ['decision', 'schedule', 'evidence', 'settings'];
    for (const tab of orderedTabs) {
      if (tab === 'settings') {
        if (hasPermission('config:read') || hasPermission('users:read') || hasPermission('kpi:read')) {
          activeTab = tab;
          return;
        }
      } else {
        const perm = tabPermissions[tab];
        if (hasPermission(perm)) {
          activeTab = tab;
          return;
        }
      }
    }
    
    // If no permission for any page
    activeTab = 'no_permission';
  }

  // Save original fetch
  const originalFetch = window.fetch;
  
  // Override window.fetch globally to inject JWT token
  window.fetch = async function(resource, init) {
    const urlStr = typeof resource === 'string' ? resource : (resource as Request).url;
    if (urlStr.includes('/api/login')) {
      return originalFetch(resource, init);
    }
    
    init = init || {};

    if (jwtToken) {
      const headers = new Headers(init.headers);
      headers.set('Authorization', `Bearer ${jwtToken}`);
      init.headers = headers;
    }
    
    try {
      const response = await originalFetch(resource, init);
      if (response.status === 401 && !urlStr.includes('/api/login')) {
        logout();
      }
      return response;
    } catch (error) {
      throw error;
    }
  };

  function getAlertKey(a: Alert): string {
    if (a.id) return a.type + "_" + a.id;
    return a.type + "_" + a.task_id;
  }

  $: activeAlerts = alerts.filter(a => isCoreMember(a.assignee));

  // Trigger shake animation when new unread notifications arrive
  $: {
    const unreadCount = activeAlerts.length;
    if (unreadCount > previousUnreadCount) {
      shakeActive = true;
      setTimeout(() => {
        shakeActive = false;
      }, 800);
    }
    previousUnreadCount = unreadCount;
  }

  function toggleNotifications() {
    showNotifications = !showNotifications;
  }

  function toggleUserDropdown() {
    const willShow = !showUserDropdown;
    showUserDropdown = willShow;
    if (willShow) {
      showNotifications = false;
      refreshCurrentUserProfile();
    }
  }

  function logout() {
    jwtToken = '';
    currentUserName = '';
    currentUserEmail = '';
    currentUserAvatar = '';
    currentUserRole = 'member';
    currentUserDepartment = '';
    currentUserPermissions = [];
    localStorage.removeItem('jwt_token');
    localStorage.removeItem('current_user_name');
    localStorage.removeItem('current_user_email');
    localStorage.removeItem('current_user_avatar');
    localStorage.removeItem('current_user_role');
    localStorage.removeItem('current_user_permissions');
    localStorage.removeItem('current_user_department');
    localStorage.removeItem('auth_degraded');
    localStorage.removeItem('auth_degraded_message');
    authDegraded = false;
    authDegradedMessage = '';
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
    alerts = [];
    showUserDropdown = false;
    activeTab = 'dashboard';
  }

  async function handleLoginSubmit(e: Event) {
    e.preventDefault();
    if (!loginUsernamePrefix || !loginPassword) {
      loginError = '账号和密码不能为空';
      return;
    }
    
    loggingIn = true;
    loginError = '';
    loginNotice = '';
    
    try {
      const fullEmail = loginUsernamePrefix.trim() + '@westwell-lab.com';
      const encryptionKey = 'django-insecure-()$+l&t333b4ncc0hrw!u!^yd_oja&0qc4n#&xnfcm)5r^n$5k';
      const encryptedPassword = CryptoJS.AES.encrypt(loginPassword, encryptionKey).toString();

      const res = await originalFetch('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: fullEmail,
          encrypted_password: encryptedPassword
        })
      });
      
      const data = await res.json();
      if (res.ok && data.status === 'success') {
        jwtToken = data.token;
        currentUserName = data.user.name;
        currentUserEmail = data.user.username;
        currentUserAvatar = data.user.avatar || '';
        currentUserRole = data.user.role || 'member';
        currentUserDepartment = normalizeDepartment(data.user.department);
        currentUserPermissions = data.user.permissions || [];
        authDegraded = data.degraded === true;
        authDegradedMessage = data.degraded ? (data.message || 'WellOS 维护中，已使用本地临时会话登录') : '';
        loginNotice = data.degraded ? (data.message || 'WellOS 维护中，已使用本地临时会话登录') : '';
        
        localStorage.setItem('jwt_token', jwtToken);
        localStorage.setItem('current_user_name', currentUserName);
        localStorage.setItem('current_user_email', currentUserEmail);
        localStorage.setItem('current_user_avatar', currentUserAvatar);
        localStorage.setItem('current_user_role', currentUserRole);
        localStorage.setItem('current_user_department', currentUserDepartment);
        localStorage.setItem('current_user_permissions', JSON.stringify(currentUserPermissions));
        localStorage.setItem('auth_degraded', authDegraded ? 'true' : 'false');
        localStorage.setItem('auth_degraded_message', authDegradedMessage);
        
        await refreshCurrentUserProfile();
        autoRedirectTab();
        
        loginUsernamePrefix = '';
        loginPassword = '';
        
        loadConfig();
        connectSSE();
        
        window.dispatchEvent(new CustomEvent('user-logged-in'));
      } else {
        loginError = data.message || '登录失败，请检查账号密码';
        loginNotice = '';
      }
    } catch (err) {
      console.error('Login error:', err);
      loginError = '登录请求失败，请确保网络通畅';
      loginNotice = '';
    } finally {
      loggingIn = false;
    }
  }

  async function dismissAlert(alert: Alert) {
    const key = getAlertKey(alert);
    try {
      const res = await fetch('/api/notifications/read', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          user_id: currentUserEmail,
          keys: [key],
          status: 'dismissed'
        })
      });
      if (!res.ok) {
        console.error('Failed to dismiss alert in backend');
      }
    } catch (err) {
      console.error('Error reporting dismiss state to backend:', err);
    }
  }

  async function clearAllAlerts() {
    const keys = activeAlerts.map(a => getAlertKey(a));
    if (keys.length === 0) return;
    try {
      const res = await fetch('/api/notifications/read', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          user_id: currentUserEmail,
          keys: keys,
          status: 'dismissed'
        })
      });
      if (!res.ok) {
        console.error('Failed to clear all alerts in backend');
      }
    } catch (err) {
      console.error('Error reporting clear all state to backend:', err);
    }
  }

  async function loadConfig() {
    try {
      const res = await fetch('/api/config');
      if (res.ok) {
        const config = await res.json();
        let users: string[] = [];
        if (config.jira) {
          if (config.jira.sync_users && config.jira.sync_users.length > 0) {
            users = [...config.jira.sync_users];
          } else if (config.jira.custom_jql) {
            const match = config.jira.custom_jql.match(/assignee\s+in\s*\(([^)]+)\)/i);
            if (match && match[1]) {
              users = match[1].split(',').map((name: string) => name.trim().replace(/['"]/g, ''));
            }
          }
        }
        if (users.length > 0) {
          users = users.filter(name => name !== '未指派' && name !== '-');
          coreMembers = new Set(users);
        } else {
          coreMembers = new Set();
        }
        return config;
      }
    } catch (e) {
      console.error('Failed to load config for notifications:', e);
      coreMembers = new Set();
    }
    return null;
  }

  function isCoreMember(name: string): boolean {
    if (!name) return false;
    const activeMembers = coreMembers.size > 0 ? coreMembers : new Set([currentUserName].filter(Boolean));
    return activeMembers.has(name) || activeMembers.has(name.split(' ')[0]);
  }

  function handleNavigateToSettings(event: CustomEvent<{ section: string }>) {
    activeTab = 'settings';
    setTimeout(() => {
      window.dispatchEvent(new CustomEvent('focus-settings-section', { detail: event.detail.section }));
    }, 50);
  }

  function formatTimeAgo(timeStr: string): string {
    if (!timeStr) return '';
    const date = new Date(timeStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    if (isNaN(diffMs)) return timeStr;
    const diffSecs = Math.floor(diffMs / 1000);
    const diffMins = Math.floor(diffSecs / 60);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    if (diffSecs < 60) {
      return '刚刚';
    } else if (diffMins < 60) {
      return `${diffMins}分钟前`;
    } else if (diffHours < 24) {
      return `${diffHours}小时前`;
    } else if (diffDays < 7) {
      return `${diffDays}天前`;
    } else {
      return date.toLocaleDateString();
    }
  }

  async function refreshCurrentUserProfile() {
    if (!jwtToken) return;
    try {
      const res = await fetch('/api/me');
      if (!res.ok) return;
      const data = await res.json();
      const user = data.user || {};

      currentUserName = user.name || currentUserName;
      currentUserEmail = user.username || user.email || currentUserEmail;
      currentUserAvatar = user.avatar || currentUserAvatar;
      currentUserRole = user.role || currentUserRole;
      currentUserDepartment = normalizeDepartment(user.department);
      if (Array.isArray(user.permissions)) {
        currentUserPermissions = user.permissions;
      }

      localStorage.setItem('current_user_name', currentUserName);
      localStorage.setItem('current_user_email', currentUserEmail);
      localStorage.setItem('current_user_avatar', currentUserAvatar);
      localStorage.setItem('current_user_role', currentUserRole);
      localStorage.setItem('current_user_department', currentUserDepartment);
      localStorage.setItem('current_user_permissions', JSON.stringify(currentUserPermissions));
      autoRedirectTab();
    } catch (e) {
      console.error('Failed to refresh current user profile:', e);
    }
  }

  function connectSSE() {
    if (!jwtToken) return;
    if (eventSource) {
      eventSource.close();
    }
    eventSource = new EventSource(`/api/notifications/sse?token=${encodeURIComponent(jwtToken)}`);
    
    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        alerts = data || [];
      } catch (e) {
        console.error('SSE message parse error:', e);
      }
    };

    eventSource.addEventListener('config-updated', async (event) => {
      try {
        const config = await loadConfig();
        if (config) {
          window.dispatchEvent(new CustomEvent('config-updated', {
            detail: config
          }));
        }
      } catch (e) {
        console.error('SSE config update refresh error:', e);
      }
    });

    eventSource.onerror = (err) => {
      console.warn('SSE connection disrupted, automatic reconnection active.');
    };
  }

  function parseJwt(token: string) {
    try {
      const base64Url = token.split('.')[1];
      const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
      const jsonPayload = decodeURIComponent(atob(base64).split('').map(function(c) {
        return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2);
      }).join(''));
      return JSON.parse(jsonPayload);
    } catch (e) {
      return null;
    }
  }

  onMount(() => {
    if (jwtToken) {
      const claims = parseJwt(jwtToken);
      if (claims) {
        if (claims.avatar && !currentUserAvatar) {
          currentUserAvatar = claims.avatar;
          localStorage.setItem('current_user_avatar', currentUserAvatar);
        }
        if (claims.name && !currentUserName) {
          currentUserName = claims.name;
          localStorage.setItem('current_user_name', currentUserName);
        }
        if (claims.user_id && !currentUserEmail) {
          currentUserEmail = claims.user_id;
          localStorage.setItem('current_user_email', currentUserEmail);
        }
        const claimDepartment = normalizeDepartment(claims.department);
        if (claimDepartment && currentUserDepartment !== claimDepartment) {
          currentUserDepartment = claimDepartment;
          localStorage.setItem('current_user_department', currentUserDepartment);
        }
        if (Array.isArray(claims.permissions) && currentUserPermissions.length === 0) {
          currentUserPermissions = claims.permissions;
          localStorage.setItem('current_user_permissions', JSON.stringify(currentUserPermissions));
        }
        if (Array.isArray(claims.groups) && currentUserRole === 'member') {
          if (claims.groups.includes('super_admin')) {
            currentUserRole = 'super_admin';
          } else if (claims.groups.includes('admin')) {
            currentUserRole = 'admin';
          }
          localStorage.setItem('current_user_role', currentUserRole);
        }
      }
      refreshCurrentUserProfile();
      loadConfig();
      connectSSE();
      autoRedirectTab();
    }

    const handleOutsideClick = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      if (showNotifications && !target.closest('.message-center')) {
        showNotifications = false;
      }
      if (showUserDropdown && !target.closest('.user-selector-container')) {
        showUserDropdown = false;
      }
    };
    
    document.addEventListener('click', handleOutsideClick);

    return () => {
      document.removeEventListener('click', handleOutsideClick);
      if (eventSource) {
        eventSource.close();
      }
    };
  });

  function getNotificationTypeLabel(type: string): string {
    switch (type) {
      case 'delay': return '延期预警';
      case 'git_push': return '代码推送';
      case 'mr_event': return 'MR 事件';
      case 'ai_review': return 'AI 评审';
      case 'semantic_linker': return 'AI 关联';
      default: return '遥测通知';
    }
  }
</script>

{#if !jwtToken}
  <div class="login-overlay">
    <div class="login-card">
      <div class="login-header">
        <div class="logo-circle">
          <span class="logo-symbol">⇅</span>
        </div>
        <h2>well-ambient</h2>
      </div>
      
      <form on:submit={handleLoginSubmit} class="login-form">
        {#if loginError}
          <div class="login-error-alert">
            ❌ {loginError}
          </div>
        {/if}
        
        <div class="login-field">
          <label for="username">账号 (邮箱前缀)</label>
          <div class="email-input-wrapper">
            <input 
              type="text" 
              id="username" 
              placeholder="请输入邮箱前缀" 
              bind:value={loginUsernamePrefix}
              required
              disabled={loggingIn}
            />
            <span class="email-suffix">@westwell-lab.com</span>
          </div>
        </div>
        
        <div class="login-field">
          <label for="password">密码</label>
          <input 
            type="password" 
            id="password" 
            placeholder="••••••••" 
            bind:value={loginPassword}
            required
            disabled={loggingIn}
          />
        </div>
        
        <button type="submit" class="login-submit-btn" disabled={loggingIn}>
          {#if loggingIn}
            <span class="spinner-mini"></span> 正在鉴权中...
          {:else}
            🚀 立即登陆
          {/if}
        </button>
      </form>
    </div>
  </div>
{:else}
<main class="app-container {activeTab === 'settings' ? 'settings-mode' : ''}">
  <!-- Header -->
  <header class="app-header {authDegraded ? 'has-maintenance-banner' : ''}">
    <div class="brand">
      <div class="logo-circle">
        <span class="logo-symbol">⇅</span>
      </div>
      <div class="brand-text">
        <h1 class="brand-name">well-ambient</h1>
      </div>
    </div>
    
    <div class="header-right-actions">
      <!-- Message Center Bell -->
      <div class="message-center">
        <button 
          class="bell-btn {activeAlerts.length > 0 ? 'has-unread' : ''} {shakeActive ? 'shake' : ''}" 
          on:click={toggleNotifications} 
          aria-label="Notifications center"
        >
          <svg class="bell-svg-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"></path>
            <path d="M13.73 21a2 2 0 0 1-3.46 0"></path>
          </svg>
          {#if activeAlerts.length > 0}
            <span class="bell-badge">{activeAlerts.length > 99 ? '99+' : activeAlerts.length}</span>
          {/if}
        </button>
        
        {#if showNotifications}
          <div class="notifications-dropdown">
            <div class="dropdown-header">
              <h3>⚡ 协同遥测中心 ({activeAlerts.length > 99 ? '99+' : activeAlerts.length})</h3>
              {#if activeAlerts.length > 0}
                <button class="clear-all-btn" on:click={clearAllAlerts}>全部忽略</button>
              {/if}
            </div>
            <div class="dropdown-body">
              {#if activeAlerts.length === 0}
                <div class="empty-alerts">
                  <span>🎉 暂无未读遥测通知</span>
                </div>
              {:else}
                {#each activeAlerts as alert}
                  {#if alert.link}
                    <a href={alert.link} target="_blank" rel="noopener noreferrer" class="alert-item alert-{alert.type} has-link" on:click|stopPropagation>
                      <div class="alert-title">
                        <div class="title-meta-left">
                          <span class="alert-type-badge type-{alert.type}">{getNotificationTypeLabel(alert.type)}</span>
                          <span class="alert-task-id">{alert.task_id}</span>
                          <span class="alert-assignee">👤 {alert.assignee}</span>
                        </div>
                        <div class="alert-title-right">
                          <span class="alert-time-ago font-mono">{formatTimeAgo(alert.created_at)}</span>
                          <button class="dismiss-single-btn" on:click|preventDefault={() => dismissAlert(alert)} title="忽略">✕</button>
                        </div>
                      </div>
                      <p class="alert-message">{alert.title}</p>
                      <span class="alert-desc">{alert.message}</span>
                      <span class="alert-action-hint">点击跳转 GitLab 🔗</span>
                    </a>
                  {:else}
                    <div class="alert-item alert-{alert.type}">
                      <div class="alert-title">
                        <div class="title-meta-left">
                          <span class="alert-type-badge type-{alert.type}">{getNotificationTypeLabel(alert.type)}</span>
                          <span class="alert-task-id">{alert.task_id}</span>
                          <span class="alert-assignee">👤 {alert.assignee}</span>
                        </div>
                        <div class="alert-title-right">
                          <span class="alert-time-ago font-mono">{formatTimeAgo(alert.created_at)}</span>
                          <button class="dismiss-single-btn" on:click={() => dismissAlert(alert)} title="忽略">✕</button>
                        </div>
                      </div>
                      <p class="alert-message">{alert.title}</p>
                      {#if alert.type === 'delay'}
                        <span class="alert-delay">⚠️ 已延期 {alert.delay_days} 天</span>
                      {:else}
                        <span class="alert-desc">{alert.message}</span>
                      {/if}
                    </div>
                  {/if}
                {/each}
              {/if}
            </div>
          </div>
        {/if}
      </div>

      <!-- Authenticated User Profile (Moved to the right of Bell) -->
      {#if jwtToken}
        <div class="user-selector-container">
          <button class="user-avatar-btn" on:click={toggleUserDropdown} aria-label="User Profile">
            <div class="avatar-circle">
              {#if currentUserAvatar}
                <img src={currentUserAvatar} alt="avatar" class="avatar-img" />
              {:else}
                {currentUserName ? currentUserName.charAt(0) : (currentUserEmail ? currentUserEmail.charAt(0).toUpperCase() : 'U')}
              {/if}
            </div>
            <span class="user-status-dot-overlay"></span>
          </button>
          {#if showUserDropdown}
            <div class="user-selector-dropdown">
              <ProfilePanel
                compact
                currentUserName={currentUserName}
                currentUserEmail={currentUserEmail}
                currentUserAvatar={currentUserAvatar}
                currentUserDepartment={currentUserDepartment}
                currentUserRole={currentUserRole}
                currentUserPermissions={currentUserPermissions}
                onRefresh={refreshCurrentUserProfile}
              />
              <button 
                class="user-option-btn logout-btn" 
                on:click={logout}
              >
                🚪 退出登录
              </button>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </header>

  {#if authDegraded}
    <div class="auth-degraded-banner">
      <span class="auth-degraded-kicker font-mono">OS MAINTENANCE</span>
      <span>{authDegradedMessage || 'WellOS 维护中，当前为本地临时会话，资料同步将在 OS 恢复后更新。'}</span>
    </div>
  {/if}

  <!-- Tab Navigation -->
  <div class="tabs-navigation font-mono">
    {#if hasPermission('decision:read')}
      <button class="tab-btn {activeTab === 'decision' ? 'active' : ''}" on:click={() => activeTab = 'decision'}>
        ⚡ 决策队列
      </button>
    {/if}
    {#if hasPermission('demands:read')}
      <button class="tab-btn {activeTab === 'schedule' ? 'active' : ''}" on:click={() => activeTab = 'schedule'}>
        🗓️ 排期治理台
      </button>
    {/if}
    {#if hasPermission('dashboard:read')}
      <button class="tab-btn {activeTab === 'evidence' ? 'active' : ''}" on:click={() => activeTab = 'evidence'}>
        🔍 证据观测台
      </button>
    {/if}
    {#if hasPermission('config:read') || hasPermission('users:read') || hasPermission('kpi:read')}
      <button class="tab-btn {activeTab === 'settings' ? 'active' : ''}" on:click={() => activeTab = 'settings'}>
        ⚙️ 系统配置
      </button>
    {/if}
  </div>

  <!-- Core Dashboard Layout -->
  <section class="app-content-shell">
    {#if activeTab === 'decision'}
      <DecisionDashboard currentUser={currentUserName} />
    {:else if activeTab === 'schedule'}
      <DemandKanban
        currentUserPermissions={currentUserPermissions}
        currentUserName={currentUserName}
        currentUserEmail={currentUserEmail}
        currentUserDepartment={currentUserDepartment}
      />
    {:else if activeTab === 'evidence'}
      <div class="dashboard-content">
        <!-- Project Health Telemetry -->
        <ProjectHealthTelemetry />

        <!-- AI Deconstructor -->
        <Deconstructor />

        <!-- Kanban -->
        <TaskKanban />
      </div>
    {:else if activeTab === 'settings'}
      <SettingsPanel
        currentUserRole={currentUserRole}
        currentUserEmail={currentUserEmail}
        currentUserPermissions={currentUserPermissions}
      />
    {:else if activeTab === 'no_permission'}
      <div class="no-permission-warning glass-panel">
        <div class="warning-icon">🔒</div>
        <h2>访问受限</h2>
        <p>您当前没有访问该系统的任何页面权限。</p>
        <p class="sub-text">请联系系统管理员分配权限组或配置查看作用域。</p>
        <button class="logout-btn font-mono" on:click={logout}>退出登录</button>
      </div>
    {/if}
  </section>

  <!-- Footer -->
  {#if activeTab !== 'settings'}
    <footer class="app-footer">
      <p>© 2026 well-ambient Sync System. All rights reserved.</p>
      <p class="font-mono text-muted">v0.1.0-alpha | Embedded Svelte Dashboard</p>
    </footer>
  {/if}
</main>
{/if}

<style>
  :global(body) {
    background-color: #020617;
    background-image: 
      radial-gradient(at 0% 0%, rgba(99, 102, 241, 0.08) 0px, transparent 50%),
      radial-gradient(at 100% 100%, rgba(124, 58, 237, 0.08) 0px, transparent 50%);
    color: #f1f5f9;
    font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif;
    margin: 0;
    padding: 0;
    min-height: 100vh;
  }

  .app-container {
    width: 90%;
    max-width: 100%;
    margin: 0 auto;
    padding: 24px;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    min-height: 100vh;
  }

  @media (max-width: 768px) {
    .app-container {
      width: 95%;
      padding: 16px 12px;
    }
  }

  .app-container.settings-mode {
    min-height: 100vh;
    min-height: 100dvh;
    overflow: visible;
  }

  .app-content-shell {
    min-width: 0;
  }

  .settings-mode .app-content-shell {
    flex: 1;
    min-height: 0;
    overflow: visible;
  }

  @media (min-width: 1101px) {
    .settings-mode .app-content-shell {
      display: flex;
      flex: 1 1 auto;
      height: auto;
      min-height: auto;
    }
  }

  .app-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid rgba(51, 65, 85, 0.3);
    padding-bottom: 20px;
    margin-bottom: 32px;
    flex-wrap: wrap;
    gap: 16px;
  }

  .app-header.has-maintenance-banner {
    margin-bottom: 14px;
  }

  .auth-degraded-banner {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 22px;
    border: 1px solid rgba(245, 158, 11, 0.34);
    background: rgba(120, 53, 15, 0.18);
    color: #fde68a;
    border-radius: 10px;
    padding: 10px 12px;
    font-size: 0.82rem;
    line-height: 1.45;
  }

  .auth-degraded-kicker {
    color: #f59e0b;
    font-size: 0.62rem;
    font-weight: 900;
    letter-spacing: 0.06em;
    white-space: nowrap;
  }

  .user-selector-container {
    position: relative;
    display: inline-block;
  }

  .user-selector-btn {
    background: rgba(15, 23, 42, 0.45);
    border: 1px solid rgba(56, 189, 248, 0.25);
    color: #f1f5f9;
    padding: 6px 12px;
    border-radius: 8px;
    font-size: 0.8rem;
    font-weight: 500;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 8px;
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
  }

  .user-avatar-btn {
    background: transparent;
    border: none;
    outline: none;
    position: relative;
    padding: 0;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    transition: transform 0.2s cubic-bezier(0.16, 1, 0.3, 1), box-shadow 0.2s;
    margin-left: 10px;
  }

  .user-avatar-btn:hover {
    transform: scale(1.05);
  }

  .user-avatar-btn:active {
    transform: scale(0.95);
  }

  .avatar-circle {
    width: 36px;
    height: 36px;
    background: linear-gradient(135deg, rgba(56, 189, 248, 0.25) 0%, rgba(3, 105, 161, 0.4) 100%);
    border: 1px solid rgba(56, 189, 248, 0.4);
    border-radius: 50%;
    color: #38bdf8;
    font-size: 0.95rem;
    font-weight: 700;
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: 0 4px 12px rgba(56, 189, 248, 0.15), inset 0 0 8px rgba(56, 189, 248, 0.1);
    text-shadow: 0 0 6px rgba(56, 189, 248, 0.5);
  }

  .avatar-img {
    width: 100%;
    height: 100%;
    border-radius: 50%;
    object-fit: cover;
  }

  .user-avatar-btn:hover .avatar-circle {
    border-color: rgba(56, 189, 248, 0.7);
    box-shadow: 0 4px 20px rgba(56, 189, 248, 0.35), inset 0 0 10px rgba(56, 189, 248, 0.2);
  }

  .user-status-dot-overlay {
    position: absolute;
    bottom: 0px;
    right: 0px;
    width: 8px;
    height: 8px;
    background-color: #10b981;
    border: 2px solid #0b1329;
    border-radius: 50%;
    box-shadow: 0 0 8px #10b981;
    animation: pulse 2s infinite;
  }

  .user-selector-dropdown {
    position: absolute;
    top: calc(100% + 12px);
    right: 0;
    width: min(360px, calc(100vw - 32px));
    background: rgba(13, 21, 39, 0.85);
    border: 1px solid rgba(56, 189, 248, 0.25);
    border-radius: 12px;
    box-shadow: 0 20px 40px -15px rgba(0, 0, 0, 0.75), 0 0 20px rgba(56, 189, 248, 0.05);
    z-index: 1000;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
  }

  .user-option-btn {
    width: 100%;
    background: transparent;
    border: none;
    outline: none;
    color: #94a3b8;
    font-size: 0.75rem;
    padding: 6px 10px;
    text-align: left;
    border-radius: 4px;
    cursor: pointer;
    transition: all 0.15s;
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .user-option-btn:hover {
    background: rgba(56, 189, 248, 0.08);
    color: #f1f5f9;
  }

  .user-option-btn.active {
    background: rgba(56, 189, 248, 0.15);
    color: #38bdf8;
    font-weight: 600;
  }

  .header-right-actions {
    display: flex;
    align-items: center;
    gap: 16px;
  }

  /* Message Center Styling */
  .message-center {
    position: relative;
    display: inline-block;
  }

  .bell-btn {
    background: rgba(30, 41, 59, 0.5);
    border: 1px solid rgba(51, 65, 85, 0.4);
    width: 36px;
    height: 36px;
    border-radius: 50%;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
    transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
    outline: none;
  }

  .bell-btn:hover {
    background: rgba(51, 65, 85, 0.5);
    border-color: rgba(99, 102, 241, 0.5);
    box-shadow: 0 0 12px rgba(99, 102, 241, 0.2);
  }

  .bell-svg-icon {
    width: 18px;
    height: 18px;
    color: #94a3b8;
    transition: all 0.2s;
  }

  .bell-btn:hover .bell-svg-icon {
    color: #818cf8;
  }

  .bell-btn.has-unread .bell-svg-icon {
    color: #ef4444;
    animation: bell-pulse 2s infinite;
  }

  .bell-btn.shake .bell-svg-icon {
    animation: bell-shake 0.6s ease-in-out;
  }

  @keyframes bell-pulse {
    0%, 100% { transform: scale(1); }
    50% { transform: scale(1.1); }
  }

  @keyframes bell-shake {
    0%, 100% { transform: rotate(0); }
    15% { transform: rotate(-15deg); }
    30% { transform: rotate(12deg); }
    45% { transform: rotate(-10deg); }
    60% { transform: rotate(8deg); }
    75% { transform: rotate(-4deg); }
  }

  .bell-badge {
    position: absolute;
    top: -4px;
    right: -4px;
    background: #ef4444;
    color: #ffffff;
    font-size: 0.6rem;
    font-weight: 800;
    height: 16px;
    min-width: 16px;
    padding: 0 4px;
    border-radius: 9999px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1.5px solid #020617;
    box-shadow: 0 0 8px rgba(239, 68, 68, 0.6);
    box-sizing: border-box;
    white-space: nowrap;
  }

  .notifications-dropdown {
    position: absolute;
    top: 46px;
    right: 0;
    width: 360px;
    background: rgba(15, 23, 42, 0.95);
    border: 1px solid rgba(129, 140, 248, 0.2);
    border-radius: 12px;
    box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.6), 0 0 20px rgba(99, 102, 241, 0.15);
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    z-index: 100;
    overflow: hidden;
    animation: slideDown 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .dropdown-header {
    padding: 12px 16px;
    background: rgba(30, 41, 59, 0.5);
    border-bottom: 1px solid rgba(51, 65, 85, 0.3);
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .dropdown-header h3 {
    margin: 0;
    font-size: 0.85rem;
    font-weight: 700;
    color: #cbd5e1;
  }

  .clear-all-btn {
    background: transparent;
    border: none;
    color: #818cf8;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
    padding: 2px 6px;
    border-radius: 4px;
    transition: all 0.2s;
  }

  .clear-all-btn:hover {
    background: rgba(99, 102, 241, 0.15);
    color: #a5b4fc;
  }

  .dropdown-body {
    max-height: 320px;
    overflow-y: auto;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    scrollbar-width: thin;
    scrollbar-color: rgba(99, 102, 241, 0.25) transparent;
  }

  .dropdown-body::-webkit-scrollbar {
    width: 4px;
  }

  .dropdown-body::-webkit-scrollbar-track {
    background: transparent;
  }

  .dropdown-body::-webkit-scrollbar-thumb {
    background: rgba(99, 102, 241, 0.25);
    border-radius: 4px;
  }

  .dropdown-body::-webkit-scrollbar-thumb:hover {
    background: rgba(99, 102, 241, 0.5);
  }

  .alert-title-right {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .dismiss-single-btn {
    background: transparent;
    border: none;
    color: #64748b;
    font-size: 0.75rem;
    cursor: pointer;
    padding: 2px 4px;
    border-radius: 4px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    opacity: 0;
    transition: all 0.2s;
  }

  .alert-item:hover .dismiss-single-btn {
    opacity: 0.6;
  }

  .alert-item:hover .dismiss-single-btn:hover {
    opacity: 1;
    color: #ef4444;
    background: rgba(239, 68, 68, 0.1);
  }

  .empty-alerts {
    padding: 24px;
    text-align: center;
    color: #64748b;
    font-size: 0.8rem;
  }

  /* Custom Type Borders and Hover Animations */
  .alert-item {
    background: rgba(2, 6, 23, 0.6);
    border: 1px solid rgba(51, 65, 85, 0.45);
    padding: 12px;
    border-radius: 10px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    text-align: left;
    text-decoration: none;
    color: inherit;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.35);
    transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .alert-item.has-link:hover {
    transform: translateY(-2px);
    background: rgba(30, 41, 59, 0.4);
    border-color: rgba(129, 140, 248, 0.5);
    box-shadow: 0 6px 16px rgba(99, 102, 241, 0.15);
  }

  .alert-item.alert-delay {
    border-left: 4px solid #ef4444;
  }

  .alert-item.alert-git_push {
    border-left: 4px solid #10b981;
  }

  .alert-item.alert-mr_event {
    border-left: 4px solid #6366f1;
  }

  .alert-item.alert-ai_review {
    border-left: 4px solid #a855f7;
  }

  .alert-item.alert-semantic_linker {
    border-left: 4px solid #06b6d4;
  }

  .alert-title {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .title-meta-left {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .alert-type-badge {
    font-size: 0.6rem;
    font-weight: 700;
    padding: 1px 5px;
    border-radius: 3px;
  }

  .alert-type-badge.type-delay { background: rgba(239, 68, 68, 0.15); color: #f87171; }
  .alert-type-badge.type-git_push { background: rgba(16, 185, 129, 0.15); color: #34d399; }
  .alert-type-badge.type-mr_event { background: rgba(99, 102, 241, 0.15); color: #818cf8; }
  .alert-type-badge.type-ai_review { background: rgba(168, 85, 247, 0.15); color: #c084fc; }
  .alert-type-badge.type-semantic_linker { background: rgba(6, 182, 212, 0.15); color: #22d3ee; }

  .alert-task-id {
    font-size: 0.7rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-weight: 600;
    color: #64748b;
  }

  .alert-assignee {
    font-size: 0.65rem;
    color: #94a3b8;
    background: rgba(148, 163, 184, 0.08);
    border: 1px solid rgba(148, 163, 184, 0.15);
    padding: 1px 4px;
    border-radius: 3px;
    margin-left: 4px;
    display: inline-flex;
    align-items: center;
  }

  .alert-time-ago {
    font-size: 0.7rem;
    color: #64748b;
  }

  .alert-message {
    margin: 0;
    font-size: 0.8rem;
    font-weight: 600;
    color: #f1f5f9;
    line-height: 1.4;
    word-break: break-word;
  }

  .alert-desc {
    font-size: 0.75rem;
    color: #94a3b8;
    line-height: 1.4;
    word-break: break-word;
  }

  .alert-git_push .alert-desc, 
  .alert-ai_review .alert-desc, 
  .alert-mr_event .alert-desc {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    background: rgba(2, 6, 23, 0.4);
    padding: 6px 8px;
    border-radius: 6px;
    border: 1px solid rgba(51, 65, 85, 0.25);
    margin-top: 4px;
    white-space: pre-wrap;
    word-break: break-all;
  }

  .alert-action-hint {
    font-size: 0.65rem;
    color: #818cf8;
    align-self: flex-end;
    margin-top: 2px;
    opacity: 0.8;
  }

  .alert-item:hover .alert-action-hint {
    opacity: 1;
    text-decoration: underline;
  }

  .alert-delay {
    font-size: 0.7rem;
    font-weight: 700;
    color: #f87171;
  }
  .alert-warning .alert-delay { color: #fbbf24; }
  .alert-critical .alert-delay { color: #f87171; }

  @keyframes slideDown {
    from { transform: translateY(-10px); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .logo-circle {
    width: 36px;
    height: 36px;
    border-radius: 8px;
    background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: 0 4px 14px rgba(79, 70, 229, 0.4);
  }

  .logo-symbol {
    font-size: 1.15rem;
    color: #ffffff;
    font-weight: 700;
  }

  .brand-name {
    font-size: 1.25rem;
    font-weight: 800;
    background: linear-gradient(135deg, #ffffff 0%, #cbd5e1 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    margin: 0;
    letter-spacing: -0.025em;
  }

  .brand-tagline {
    font-size: 0.825rem;
    color: #64748b;
    margin: 0;
  }

  .header-status {
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(30, 41, 59, 0.5);
    border: 1px solid rgba(51, 65, 85, 0.4);
    padding: 6px 14px;
    border-radius: 9999px;
  }

  .pulse-indicator {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background-color: #34d399;
    box-shadow: 0 0 8px #34d399;
    animation: pulse 2s infinite;
  }

  .status-text {
    font-size: 0.65rem;
    font-weight: 700;
    color: #34d399;
    letter-spacing: 0.05em;
  }

  .tabs-navigation {
    display: flex;
    gap: 12px;
    margin-bottom: 24px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.3);
    padding-bottom: 12px;
  }

  .tab-btn {
    background: transparent;
    border: 1px solid transparent;
    color: #94a3b8;
    padding: 8px 16px;
    border-radius: 8px;
    font-size: 0.85rem;
    font-weight: 700;
    cursor: pointer;
    transition: all 0.2s;
  }

  .tab-btn:hover {
    color: #cbd5e1;
    background: rgba(30, 41, 59, 0.4);
    border-color: rgba(51, 65, 85, 0.4);
  }

  .tab-btn.active {
    color: #818cf8;
    background: rgba(99, 102, 241, 0.1);
    border-color: rgba(99, 102, 241, 0.3);
    box-shadow: 0 0 10px rgba(99, 102, 241, 0.1);
  }

  .dashboard-content {
    flex-grow: 1;
    display: flex;
    flex-direction: column;
  }

  .app-footer {
    border-top: 1px solid rgba(51, 65, 85, 0.3);
    padding-top: 20px;
    margin-top: 48px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 12px;
  }

  .app-footer p {
    font-size: 0.75rem;
    color: #475569;
    margin: 0;
  }

  .font-mono {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  .text-muted {
    color: #334155;
  }

  @keyframes pulse {
    0%, 100% { transform: scale(1); opacity: 1; }
    50% { transform: scale(1.1); opacity: 0.6; }
  }

  .login-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: url('/login-bg-bottom-left.81e7ef70.png') no-repeat left center, radial-gradient(circle at center, #0f172a 0%, #020617 100%);
    background-size: min(90%, 780px) auto, cover;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    z-index: 99999;
    padding: 20px 10%;
  }

  .email-input-wrapper {
    display: flex;
    align-items: center;
    border: 1px solid rgba(56, 189, 248, 0.2);
    background: rgba(15, 23, 42, 0.6);
    border-radius: 8px;
    padding-right: 12px;
    transition: border-color 0.2s, box-shadow 0.2s;
    width: 100%;
  }
  .email-input-wrapper:focus-within {
    border-color: rgba(56, 189, 248, 0.6);
    box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.15);
  }
  .email-input-wrapper input {
    flex: 1;
    width: 0;
    border: none !important;
    background: transparent !important;
    outline: none !important;
    box-shadow: none !important;
    padding: 10px 12px;
    color: #f1f5f9;
  }
  .email-input-wrapper input:-webkit-autofill,
  .email-input-wrapper input:-webkit-autofill:hover, 
  .email-input-wrapper input:-webkit-autofill:focus, 
  .email-input-wrapper input:-webkit-autofill:active {
    -webkit-box-shadow: 0 0 0 1000px #0a0f1d inset !important;
    box-shadow: 0 0 0 1000px #0a0f1d inset !important;
    -webkit-text-fill-color: #f1f5f9 !important;
  }
  .email-suffix {
    color: #64748b;
    font-size: 0.9rem;
    user-select: none;
    white-space: nowrap;
  }

  .login-card {
    background: rgba(15, 23, 42, 0.45);
    border: 1px solid rgba(56, 189, 248, 0.25);
    padding: 36px;
    border-radius: 16px;
    width: 100%;
    max-width: 440px;
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    box-shadow: 0 20px 50px -10px rgba(0, 0, 0, 0.7), 0 0 30px rgba(56, 189, 248, 0.05);
    border-top: 1px solid rgba(56, 189, 248, 0.4);
  }

  .login-header {
    text-align: center;
    margin-bottom: 28px;
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .login-header h2 {
    font-size: 1.5rem;
    font-weight: 700;
    color: #f1f5f9;
    margin: 16px 0 6px 0;
  }

  .login-header p {
    font-size: 0.8rem;
    color: #64748b;
    margin: 0;
  }

  .login-form {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .login-error-alert {
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #f87171;
    padding: 10px 14px;
    border-radius: 8px;
    font-size: 0.78rem;
    line-height: 1.4;
  }

  .login-field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .login-field label {
    font-size: 0.75rem;
    font-weight: 600;
    color: #94a3b8;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .login-field input {
    background: rgba(2, 6, 23, 0.5);
    border: 1px solid rgba(51, 65, 85, 0.5);
    color: #f1f5f9;
    padding: 11px 14px;
    border-radius: 8px;
    font-size: 0.9rem;
    outline: none;
    transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .login-field input:focus {
    border-color: rgba(56, 189, 248, 0.6);
    box-shadow: 0 0 12px rgba(56, 189, 248, 0.15);
    background: rgba(2, 6, 23, 0.7);
  }

  .login-submit-btn {
    background: linear-gradient(135deg, #0284c7 0%, #0369a1 100%);
    border: 1px solid rgba(56, 189, 248, 0.3);
    color: #ffffff;
    padding: 12px;
    border-radius: 8px;
    font-size: 0.9rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    box-shadow: 0 4px 14px rgba(2, 132, 199, 0.3);
  }

  .login-submit-btn:hover:not(:disabled) {
    background: linear-gradient(135deg, #0ea5e9 0%, #0284c7 100%);
    box-shadow: 0 6px 20px rgba(2, 132, 199, 0.45);
    transform: translateY(-1px);
  }

  .login-submit-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .spinner-mini {
    width: 14px;
    height: 14px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-radius: 50%;
    border-top-color: #ffffff;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .logout-btn {
    color: #f87171;
  }
  .logout-btn:hover {
    background: rgba(239, 68, 68, 0.08) !important;
    color: #fca5a5;
  }

  .no-permission-warning {
    max-width: 460px;
    margin: 80px auto;
    padding: 40px;
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    border: 1px solid rgba(239, 68, 68, 0.2);
  }

  .no-permission-warning .warning-icon {
    font-size: 3rem;
    filter: drop-shadow(0 0 10px rgba(239, 68, 68, 0.2));
  }

  .no-permission-warning h2 {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 700;
    color: #f87171;
  }

  .no-permission-warning p {
    margin: 0;
    font-size: 0.88rem;
    color: #cbd5e1;
    line-height: 1.5;
  }

  .no-permission-warning p.sub-text {
    font-size: 0.78rem;
    color: #64748b;
  }

  .no-permission-warning .logout-btn {
    margin-top: 12px;
    background: rgba(30, 41, 59, 0.5);
    border: 1px solid rgba(239, 68, 68, 0.3) !important;
    border-radius: 6px;
    padding: 8px 24px;
    font-size: 0.8rem;
    font-weight: 700;
    cursor: pointer;
    transition: all 0.2s;
  }
</style>
