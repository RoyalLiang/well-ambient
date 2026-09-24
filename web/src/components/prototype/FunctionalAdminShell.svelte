<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import { SETTINGS_SECTION_DEFINITIONS } from '../../lib/settings-sections';
  import ProjectPreferences from '../ProjectPreferences.svelte';
  import ToastHost from '../shared/ToastHost.svelte';

  interface ConsoleAlert {
    id: number;
    type: string;
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

  interface NavItem {
    route: string;
    label: string;
    subtitle: string;
    icon: string;
    tone: 'blue' | 'green' | 'amber' | 'rose' | 'violet' | 'slate';
    children?: NavSubItem[];
  }

  interface NavSubItem {
    section: string;
    group: string;
    label: string;
    subtitle: string;
    permissions: string[];
	roles?: string[];
  }

  interface GlobalSearchResult {
    id: string;
    title: string;
    owner: string;
    status: string;
    type: string;
    route: 'schedule' | 'tasks';
  }

  type ScheduleView = 'board' | 'schedule' | 'releases' | 'projects';
  type TaskView = 'status' | 'execution' | 'review';
  type DecisionView = 'agenda' | 'daily_jira';
	type KPIView = 'overview' | 'calculation';

  const decisionSubnav: NavSubItem[] = [
    { section: 'agenda', group: '决策看板', label: '决策事项', subtitle: '议程与调停', permissions: ['decision:read'] },
    { section: 'daily_jira', group: '决策看板', label: '每日 Jira', subtitle: '早会审计', permissions: ['decision:read'] }
  ];

  const scheduleSubnav: NavSubItem[] = [
    { section: 'schedule', group: '排期治理', label: '排期看板', subtitle: '治理总表', permissions: ['demands:read'] },
    { section: 'releases', group: '排期治理', label: '版本计划', subtitle: '范围与承诺', permissions: ['demands:read'] },
    { section: 'board', group: '排期治理', label: '流转看板', subtitle: '需求流转', permissions: ['demands:read'] },
    { section: 'projects', group: '排期治理', label: '项目看板', subtitle: '项目进展', permissions: ['demands:read'] }
  ];

  const taskSubnav: NavSubItem[] = [
    { section: 'status', group: '任务跟踪', label: '任务表', subtitle: '责任与状态', permissions: ['dashboard:read'] },
    { section: 'execution', group: '任务跟踪', label: '执行追踪', subtitle: '代码与 MR', permissions: ['dashboard:read'] },
    { section: 'review', group: '任务跟踪', label: '代码评审', subtitle: '知识与代码证据', permissions: ['dashboard:read'] }
  ];

	const kpiSubnav: NavSubItem[] = [
		{ section: 'overview', group: '度量洞察', label: '度量概览', subtitle: '绩效事实', permissions: ['kpi:read'] },
		{ section: 'calculation', group: '度量洞察', label: '计算说明', subtitle: '口径与审计', permissions: ['kpi:read'], roles: ['super_admin'] }
	];

  const settingsSubnav: NavSubItem[] = SETTINGS_SECTION_DEFINITIONS.map((section) => ({
    section: section.id,
    group: section.group,
    label: section.label,
    subtitle: section.domain,
    permissions: section.permissions
  }));

  const navItems: NavItem[] = [
    { route: 'decision', label: '决策看板', subtitle: '会议与阻塞', icon: 'grid', tone: 'blue', children: decisionSubnav },
    { route: 'schedule', label: '排期治理', subtitle: '需求与风险', icon: 'calendar', tone: 'green', children: scheduleSubnav },
    { route: 'solutions', label: '方案中心', subtitle: '方案与标准', icon: 'library', tone: 'blue' },
    { route: 'evidence', label: '证据链', subtitle: '健康与解构', icon: 'network', tone: 'amber' },
    { route: 'tasks', label: '任务跟踪', subtitle: '执行闭环', icon: 'checklist', tone: 'rose', children: taskSubnav },
    { route: 'kpi', label: '度量洞察', subtitle: '绩效事实', icon: 'analytics', tone: 'violet', children: kpiSubnav },
    { route: 'settings', label: '配置中心', subtitle: '规则与用户', icon: 'settings', tone: 'slate', children: settingsSubnav }
  ];

  export let activeRoute = 'decision';
  export let activeDecisionView: DecisionView = 'agenda';
  export let activeSettingsSection = 'gitlab';
  export let activeScheduleView: ScheduleView = 'schedule';
  export let activeTaskView: TaskView = 'status';
	export let activeKPIView: KPIView = 'overview';
  export let availableRoutes: string[] = [];
  export let currentUserName = '';
  export let currentUserEmail = '';
  export let currentUserAvatar = '';
  export let currentUserRole = 'member';
  export let currentUserDepartment = '';
  export let currentUserPermissions: string[] = [];
  export let alertCount = 0;
  export let alerts: ConsoleAlert[] = [];
  export let authDegraded = false;
  export let authDegradedMessage = '';
  export let onNavigate: (route: string) => void = () => {};
  export let onDecisionNavigate: (view: DecisionView) => void = () => {};
  export let onSettingsNavigate: (section: string) => void = () => {};
  export let onScheduleNavigate: (view: ScheduleView) => void = () => {};
  export let onTaskNavigate: (view: TaskView) => void = () => {};
	export let onKPINavigate: (view: KPIView) => void = () => {};
  export let onLogout: () => void = () => {};
  export let onClearAlerts: () => void | Promise<void> = () => {};
  export let onDismissAlert: (alert: ConsoleAlert) => void | Promise<void> = () => {};
  export let onRefreshProfile: () => void | Promise<void> = () => {};
  export let onProjectPreferencesChange: (preference: { mode: string; project_keys: string[] }) => void = () => {};

  let showAlerts = false;
  let showProfile = false;
  let railCollapsed = false;
  let mobileRailOpen = false;
  let notificationWrapEl: HTMLDivElement;
  let globalSearchEl: HTMLFormElement;
  let workspaceFrameEl: HTMLElement | null = null;
  let mainContentTop = 68;
  let lastWorkspaceKey = '';
  let globalSearchQuery = '';
  let globalSearchResults: GlobalSearchResult[] = [];
  let globalSearchLoading = false;
  let globalSearchError = '';
  let showGlobalSearchResults = false;
  let globalSearchTimer: ReturnType<typeof setTimeout> | null = null;
  let globalSearchRequestId = 0;

  $: visibleNav = navItems.filter((item) => availableRoutes.includes(item.route));
  $: activeItem = navItems.find((item) => item.route === activeRoute) || visibleNav[0] || navItems[0];
  $: workspaceKey = `${activeRoute}:${activeRoute === 'decision' ? activeDecisionView : activeRoute === 'schedule' ? activeScheduleView : activeRoute === 'tasks' ? activeTaskView : activeRoute === 'kpi' ? activeKPIView : activeRoute === 'settings' ? activeSettingsSection : ''}`;
  $: displayName = currentUserName || currentUserEmail || 'well user';
  $: avatarMark = displayName.slice(0, 1).toUpperCase();
  $: roleLabel = currentUserRole === 'super_admin'
    ? '超级管理员'
    : currentUserRole === 'admin'
      ? '系统管理员'
      : '成员';
  $: departmentLabel = currentUserDepartment || '未分配部门';
  $: unreadLabel = alertCount > 99 ? '99+' : String(alertCount);
  $: if (workspaceFrameEl && workspaceKey !== lastWorkspaceKey) {
    const nextWorkspaceKey = workspaceKey;
    lastWorkspaceKey = nextWorkspaceKey;
    tick().then(() => {
      if (workspaceKey === nextWorkspaceKey) {
        workspaceFrameEl?.scrollTo({ top: 0, left: 0, behavior: 'auto' });
      }
    });
  }

  function navigate(route: string) {
    onNavigate(route);
    showAlerts = false;
    showProfile = false;
    mobileRailOpen = false;
  }

  function toggleRail() {
    if (window.matchMedia('(max-width: 860px)').matches) {
      mobileRailOpen = !mobileRailOpen;
      return;
    }
    railCollapsed = !railCollapsed;
  }

  function trackMainContentTop(node: HTMLElement) {
    const update = () => {
      mainContentTop = Math.max(0, node.getBoundingClientRect().top);
    };
    const observer = new ResizeObserver(update);
    const shellColumn = node.parentElement;

    observer.observe(node);
    if (shellColumn) {
      observer.observe(shellColumn);
    }
    window.addEventListener('resize', update);
    update();

    return {
      destroy() {
        observer.disconnect();
        window.removeEventListener('resize', update);
      }
    };
  }

  function canAccessSubItem(item: NavSubItem) {
		const roleAllowed = !item.roles || item.roles.includes(currentUserRole);
		return roleAllowed && item.permissions.some((permission) => currentUserPermissions.includes(permission));
  }

  function visibleChildren(item: NavItem) {
    return item.children?.filter(canAccessSubItem) || [];
  }

  function navigateSettingsSection(section: string) {
    onSettingsNavigate(section);
    showAlerts = false;
    showProfile = false;
    mobileRailOpen = false;
  }

  function isSubnavActive(route: string, section: string) {
    if (route === 'decision') return activeDecisionView === section;
    if (route === 'settings') return activeSettingsSection === section;
    if (route === 'schedule') return activeScheduleView === section;
    if (route === 'tasks') return activeTaskView === section;
		if (route === 'kpi') return activeKPIView === section;
    return false;
  }

  function navigateSubItem(route: string, section: string) {
    if (route === 'decision' && (section === 'agenda' || section === 'daily_jira')) {
      onDecisionNavigate(section);
      showAlerts = false;
      showProfile = false;
      mobileRailOpen = false;
    } else if (route === 'settings') {
      navigateSettingsSection(section);
    } else if (route === 'schedule' && (section === 'board' || section === 'schedule' || section === 'releases' || section === 'projects')) {
      onScheduleNavigate(section);
      showAlerts = false;
      showProfile = false;
      mobileRailOpen = false;
    } else if (route === 'tasks' && (section === 'status' || section === 'execution' || section === 'review')) {
      onTaskNavigate(section);
      showAlerts = false;
      showProfile = false;
      mobileRailOpen = false;
		} else if (route === 'kpi' && (section === 'overview' || section === 'calculation')) {
			onKPINavigate(section);
			showAlerts = false;
			showProfile = false;
			mobileRailOpen = false;
    }
  }

  function toggleAlerts() {
    showAlerts = !showAlerts;
    if (showAlerts) showProfile = false;
  }

  function toggleProfile() {
    showProfile = !showProfile;
    if (showProfile) {
      showAlerts = false;
      onRefreshProfile();
    }
  }

  function formatTimeAgo(timeStr: string): string {
    if (!timeStr) return '';
    const date = new Date(timeStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    if (Number.isNaN(diffMs)) return timeStr;
    const minutes = Math.floor(diffMs / 60000);
    if (minutes < 1) return '刚刚';
    if (minutes < 60) return `${minutes} 分钟前`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours} 小时前`;
    const days = Math.floor(hours / 24);
    if (days < 7) return `${days} 天前`;
    return date.toLocaleDateString();
  }

  function getAlertTypeLabel(type: string): string {
    switch (type) {
      case 'delay': return '延期预警';
      case 'git_push': return '代码推送';
      case 'mr_event': return 'MR 事件';
      case 'ai_review': return 'AI 评审';
      case 'semantic_linker': return 'AI 关联';
      default: return '遥测通知';
    }
  }

  function dismiss(alert: ConsoleAlert) {
    onDismissAlert(alert);
  }

  function clearAlerts() {
    onClearAlerts();
  }

  function normalizeSearchValue(value: unknown): string {
    return String(value || '').trim().toLocaleLowerCase('zh-CN');
  }

  async function runGlobalSearch(): Promise<GlobalSearchResult[]> {
    const query = globalSearchQuery.trim();
    if (globalSearchTimer) {
      clearTimeout(globalSearchTimer);
      globalSearchTimer = null;
    }
    if (query.length < 2) {
      globalSearchResults = [];
      globalSearchError = query ? '至少输入 2 个字符' : '';
      showGlobalSearchResults = Boolean(query);
      return [];
    }

    const requestId = ++globalSearchRequestId;
    globalSearchLoading = true;
    globalSearchError = '';
    showGlobalSearchResults = true;

    try {
      const response = await fetch(`/api/work-items?search=${encodeURIComponent(query)}&limit=8`, {
        cache: 'no-store'
      });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const payload: {
        items?: Array<{ work_item?: Record<string, unknown> }>;
      } = await response.json();
      const tasks = (payload.items || [])
        .map((snapshot) => snapshot.work_item || {})
        .filter((task) => Boolean(task.task_id));
      const normalizedQuery = normalizeSearchValue(query);

      const nextResults = tasks
        .sort((a, b) => {
          const aExact = normalizeSearchValue(a.task_id) === normalizedQuery ? 0 : 1;
          const bExact = normalizeSearchValue(b.task_id) === normalizedQuery ? 0 : 1;
          if (aExact !== bExact) return aExact - bExact;
          const aTitleExact = normalizeSearchValue(a.title) === normalizedQuery ? 0 : 1;
          const bTitleExact = normalizeSearchValue(b.title) === normalizedQuery ? 0 : 1;
          return aTitleExact - bTitleExact;
        })
        .slice(0, 8)
        .map((task) => {
          const type = String(task.issue_type || 'requirement');
          return {
            id: String(task.task_id || ''),
            title: String(task.title || task.task_id || '未命名事项'),
            owner: String(task.assignee || '未指派'),
            status: String(task.status || '未知状态'),
            type,
            route: 'tasks'
          } satisfies GlobalSearchResult;
        });

      if (requestId !== globalSearchRequestId) return [];
      globalSearchResults = nextResults;
      if (nextResults.length === 0) {
        globalSearchError = '没有匹配的需求或 Bug';
      }
      return nextResults;
    } catch (error) {
      if (requestId !== globalSearchRequestId) return [];
      console.error('Global search failed:', error);
      globalSearchResults = [];
      globalSearchError = '搜索暂时不可用，请稍后重试';
      return [];
    } finally {
      if (requestId === globalSearchRequestId) globalSearchLoading = false;
    }
  }

  async function submitGlobalSearch() {
    const submittedQuery = globalSearchQuery.trim();
    const results = await runGlobalSearch();
    if (!submittedQuery || submittedQuery !== globalSearchQuery.trim() || results.length === 0) return;
    const normalizedQuery = normalizeSearchValue(submittedQuery);
    const result = results.find((candidate) => normalizeSearchValue(candidate.id) === normalizedQuery)
      || results.find((candidate) => normalizeSearchValue(candidate.title) === normalizedQuery)
      || results[0];
    await selectGlobalSearchResult(result);
  }

  function queueGlobalSearch() {
    if (globalSearchTimer) clearTimeout(globalSearchTimer);
    if (!globalSearchQuery.trim()) {
      globalSearchResults = [];
      globalSearchError = '';
      showGlobalSearchResults = false;
      window.dispatchEvent(new CustomEvent('well-ambient:global-search-clear'));
      return;
    }
    globalSearchTimer = setTimeout(runGlobalSearch, 220);
  }

  async function selectGlobalSearchResult(result: GlobalSearchResult) {
    showGlobalSearchResults = false;
    if (result.route === 'schedule') {
      onScheduleNavigate('schedule');
    } else {
      onTaskNavigate('status');
    }
    await tick();
    window.dispatchEvent(new CustomEvent('well-ambient:global-search-select', {
      detail: { id: result.id, route: result.route }
    }));
  }

  function handleWindowClick(event: MouseEvent) {
    if (showAlerts && notificationWrapEl && !notificationWrapEl.contains(event.target as Node)) {
      showAlerts = false;
    }
    if (showGlobalSearchResults && globalSearchEl && !globalSearchEl.contains(event.target as Node)) {
      showGlobalSearchResults = false;
    }
  }

  onMount(() => {
    window.addEventListener('click', handleWindowClick);
  });

  onDestroy(() => {
    if (globalSearchTimer) clearTimeout(globalSearchTimer);
    window.removeEventListener('click', handleWindowClick);
  });
</script>

<svelte:head>
  <title>well-ambient</title>
</svelte:head>

<main
  class="functional-console"
  class:rail-collapsed={railCollapsed}
  style="--wa-main-content-top: {mainContentTop}px;"
>
  <aside class="console-rail" class:mobile-open={mobileRailOpen} aria-label="well-ambient 管理台导航">
    <div class="brand-block">
      <img class="brand-mark" src="/brand-mark.svg" alt="" aria-hidden="true" />
      <div class="brand-copy">
        <strong>well-ambient</strong>
        <span>Ambient Operations</span>
      </div>
    </div>

    <nav class="rail-nav" aria-label="功能模块">
      {#each visibleNav as item}
        <div class="nav-stack" class:active={activeRoute === item.route} class:has-children={visibleChildren(item).length > 0}>
          <button
            type="button"
            class:active={activeRoute === item.route}
            class="nav-primary tone-{item.tone}"
            aria-current={activeRoute === item.route ? 'page' : undefined}
            on:click={() => navigate(item.route)}
          >
            <span class="nav-icon wa-icon icon-{item.icon}" aria-hidden="true"></span>
            <span class="nav-copy">
              <strong>{item.label}</strong>
              <small>{item.subtitle}</small>
            </span>
          </button>

          {#if activeRoute === item.route && visibleChildren(item).length > 0}
            <div class="rail-subnav" aria-label="{item.label} 子菜单">
              {#each visibleChildren(item) as child}
                <button
                  type="button"
                  class="subnav-button"
                  class:active={isSubnavActive(item.route, child.section)}
                  aria-current={isSubnavActive(item.route, child.section) ? 'page' : undefined}
                  on:click={() => navigateSubItem(item.route, child.section)}
                >
                  <span class="subnav-dot" aria-hidden="true"></span>
                  <span class="subnav-copy">
                    <strong>{child.label}</strong>
                    <small>{child.group} · {child.subtitle}</small>
                  </span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </nav>

    <div class="rail-status">
      <span class="status-kicker">SYSTEM</span>
      <strong>{authDegraded ? 'Maintenance Mode' : 'Operational'}</strong>
      <span>{authDegraded ? '本地临时会话' : '实时遥测已接入'}</span>
    </div>
  </aside>

  {#if mobileRailOpen}
    <button type="button" class="rail-backdrop" aria-label="关闭菜单" on:click={() => mobileRailOpen = false}></button>
  {/if}

  <section class="console-main">
    <header class="console-topbar">
      <div class="title-cluster">
        <button type="button" class="topbar-menu" aria-label="折叠菜单" aria-expanded={mobileRailOpen || !railCollapsed} on:click={toggleRail}>
          <span class="wa-icon icon-menu" aria-hidden="true"></span>
        </button>
        <div>
          <span class="module-kicker">well-ambient / {activeItem.subtitle}</span>
          <h1>{activeItem.label}</h1>
        </div>
      </div>

      <form class="global-search" aria-label="全局搜索" bind:this={globalSearchEl} on:submit|preventDefault={submitGlobalSearch}>
        <span class="wa-icon icon-search" aria-hidden="true"></span>
        <label class="search-label" for="global-search-input">全局搜索</label>
        <input
          id="global-search-input"
          type="search"
          role="combobox"
          placeholder="搜索需求、Bug、负责人"
          bind:value={globalSearchQuery}
          aria-controls="global-search-results"
          aria-expanded={showGlobalSearchResults}
          aria-autocomplete="list"
          autocomplete="off"
          on:input={queueGlobalSearch}
          on:keydown={(event) => {
            if (event.key !== 'Enter') return;
            event.preventDefault();
            void submitGlobalSearch();
          }}
          on:focus={() => { if (globalSearchQuery.trim()) showGlobalSearchResults = true; }}
        />
        <button type="submit" class="global-search-submit" aria-label="提交搜索" aria-busy={globalSearchLoading}>
          {globalSearchLoading ? '···' : '搜索'}
        </button>

        {#if showGlobalSearchResults}
          <div id="global-search-results" class="global-search-results" role="listbox" aria-label="全局搜索结果">
            <div class="global-search-results-head">
              <span>{globalSearchLoading ? '正在搜索' : '搜索结果'}</span>
              {#if !globalSearchLoading && globalSearchResults.length > 0}
                <strong>{globalSearchResults.length} 条</strong>
              {/if}
            </div>
            {#if globalSearchError}
              <div class="global-search-empty">{globalSearchError}</div>
            {:else}
              {#each globalSearchResults as result}
                <button type="button" class="global-search-result" role="option" aria-selected="false" on:click={() => selectGlobalSearchResult(result)}>
                  <span class="search-result-type">{result.type.toLowerCase() === 'bug' ? 'Bug' : '需求'}</span>
                  <span class="search-result-copy">
                    <strong>{result.title}</strong>
                    <small>{result.id} · {result.owner} · {result.status}</small>
                  </span>
                  <span class="search-result-route">任务表</span>
                </button>
              {/each}
            {/if}
          </div>
        {/if}
      </form>

      <div class="topbar-actions">
        <div class="notification-wrap" bind:this={notificationWrapEl}>
          <button
            type="button"
            class="icon-button"
            class:has-unread={alertCount > 0}
            aria-label="通知中心"
            on:click={toggleAlerts}
          >
            <span class="wa-icon icon-bell" aria-hidden="true"></span>
            {#if alertCount > 0}
              <em>{unreadLabel}</em>
            {/if}
          </button>

          {#if showAlerts}
            <div class="popover alert-popover">
              <div class="popover-head">
                <div>
                  <strong>协同遥测</strong>
                  <span>{alertCount} 条未读</span>
                </div>
                {#if alerts.length > 0}
                  <button type="button" on:click={clearAlerts}>全部忽略</button>
                {/if}
              </div>
              <div class="alert-list">
                {#if alerts.length === 0}
                  <div class="empty-state">暂无未读遥测通知</div>
                {:else}
                  {#each alerts as alert}
                    <article class="alert-row">
                      <div class="alert-row-head">
                        <span>{getAlertTypeLabel(alert.type)}</span>
                        <small>{formatTimeAgo(alert.created_at)}</small>
                      </div>
                      <strong>{alert.title}</strong>
                      <p>{alert.message || alert.task_id}</p>
                      <div class="alert-row-actions">
                        {#if alert.link}
                          <a href={alert.link} target="_blank" rel="noopener noreferrer">查看证据</a>
                        {/if}
                        <button type="button" on:click={() => dismiss(alert)}>忽略</button>
                      </div>
                    </article>
                  {/each}
                {/if}
              </div>
            </div>
          {/if}
        </div>

        <button type="button" class="icon-button help-button" aria-label="帮助中心">
          <span class="wa-icon icon-help" aria-hidden="true"></span>
        </button>

        <div class="profile-wrap">
          <button type="button" class="profile-button" aria-label="用户菜单" on:click={toggleProfile}>
            <span class="avatar">
              {#if currentUserAvatar}
                <img src={currentUserAvatar} alt="" />
              {:else}
                {avatarMark}
              {/if}
            </span>
            <span class="profile-copy">
              <strong>{displayName}</strong>
              <small>{currentUserEmail || departmentLabel}</small>
            </span>
          </button>

          {#if showProfile}
            <div class="popover profile-popover">
              <div class="profile-card-head">
                <span class="avatar large">
                  {#if currentUserAvatar}
                    <img src={currentUserAvatar} alt="" />
                  {:else}
                    {avatarMark}
                  {/if}
                </span>
                <div>
                  <strong>{displayName}</strong>
                  <span>{currentUserEmail || '未绑定邮箱'}</span>
                </div>
              </div>
              <dl class="profile-meta" aria-label="登录信息">
                <div>
                  <dt>部门</dt>
                  <dd>{departmentLabel}</dd>
                </div>
                <div>
                  <dt>状态</dt>
                  <dd>{authDegraded ? '临时会话' : '已登录'}</dd>
                </div>
              </dl>
              <ProjectPreferences on:saved={(event) => onProjectPreferencesChange(event.detail)} />
              <button type="button" class="logout-button" on:click={onLogout}>退出登录</button>
            </div>
          {/if}
        </div>
      </div>
    </header>

    {#if authDegraded}
      <div class="degraded-banner">
        <strong>OS MAINTENANCE</strong>
        <span>{authDegradedMessage || 'WellOS 维护中，当前为本地临时会话，资料同步将在 OS 恢复后更新。'}</span>
      </div>
    {/if}

    <div class="workspace-stage" use:trackMainContentTop>
      <div
        class="workspace-frame"
        class:flow-frame={activeRoute === 'schedule' && (activeScheduleView === 'board' || activeScheduleView === 'projects')}
        class:viewport-fit-frame={['decision', 'schedule', 'solutions', 'evidence', 'tasks'].includes(activeRoute)}
        class:daily-jira-frame={activeRoute === 'decision' && activeDecisionView === 'daily_jira'}
        bind:this={workspaceFrameEl}
      >
        <slot />
      </div>
      <ToastHost />
    </div>
  </section>
</main>

<style>
  :global(body) {
    margin: 0;
    min-height: 100vh;
    background:
      radial-gradient(circle at 18% 0%, rgba(38, 221, 255, 0.12), transparent 28rem),
      radial-gradient(circle at 92% 18%, rgba(114, 230, 180, 0.08), transparent 30rem),
      linear-gradient(135deg, var(--wa-bg-base, #06090d) 0%, #071018 58%, #0b1218 100%);
    color: var(--wa-text-main, #d8e5ec);
    font-family: var(--wa-font-sans, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", sans-serif);
  }

  :global(body)::before {
    content: "";
    position: fixed;
    inset: 0;
    z-index: 0;
    pointer-events: none;
    opacity: 0.028;
    background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='200' height='200'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.75' numOctaves='4'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)'/%3E%3C/svg%3E");
  }

  :global(*) {
    box-sizing: border-box;
  }

  .functional-console {
    position: relative;
    z-index: 1;
    display: grid;
    grid-template-columns: 284px minmax(0, 1fr);
    width: 100%;
    min-height: 100vh;
    min-height: 100dvh;
    color: var(--wa-text-main, #d8e5ec);
  }

  .console-rail {
    position: sticky;
    top: 0;
    height: 100vh;
    height: 100dvh;
    padding: 22px 18px;
    background:
      linear-gradient(180deg, rgba(7, 11, 16, 0.96), rgba(9, 13, 18, 0.98)),
      radial-gradient(circle at 18% 0%, rgba(38, 221, 255, 0.14), transparent 18rem);
    color: var(--wa-text-strong, #f4fbff);
    display: flex;
    flex-direction: column;
    gap: 24px;
    border-right: 1px solid var(--wa-border-soft, rgba(203, 234, 244, 0.14));
    box-shadow: 18px 0 54px rgba(1, 9, 14, 0.28);
  }

  .brand-block {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 8px 18px;
    border-bottom: 1px solid var(--wa-border-soft, rgba(203, 234, 244, 0.14));
  }

  .brand-mark {
    width: 42px;
    height: 42px;
    border-radius: 14px;
    display: block;
    flex: 0 0 auto;
    object-fit: contain;
    box-shadow: none;
  }

  .brand-copy {
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;
  }

  .brand-copy strong {
    font-size: 1.02rem;
    letter-spacing: 0.01em;
  }

  .brand-copy span {
    color: var(--wa-text-muted, #90a8b5);
    font-size: 0.7rem;
    white-space: nowrap;
  }

  .rail-nav {
    display: grid;
    gap: 8px;
    min-height: 0;
    overflow: auto;
    padding-right: 2px;
    scrollbar-width: none;
  }

  .rail-nav::-webkit-scrollbar {
    display: none;
  }

  .nav-stack {
    display: grid;
    gap: 6px;
    min-width: 0;
  }

  .nav-primary {
    width: 100%;
    min-height: 58px;
    display: grid;
    grid-template-columns: 38px minmax(0, 1fr);
    align-items: center;
    gap: 10px;
    padding: 9px 11px;
    border: 1px solid transparent;
    border-radius: 16px;
    background: transparent;
    color: var(--wa-text-main, #d8e5ec);
    cursor: pointer;
    text-align: left;
    transition: transform 0.18s ease, background 0.18s ease, border-color 0.18s ease, color 0.18s ease;
  }

  .nav-primary:hover {
    transform: translateX(2px);
    background: rgba(244, 251, 255, 0.06);
    color: var(--wa-text-strong, #f4fbff);
  }

  .nav-primary.active {
    background: rgba(244, 251, 255, 0.11);
    border-color: rgba(203, 234, 244, 0.18);
    color: var(--wa-text-strong, #f4fbff);
    box-shadow: inset 0 1px 0 rgba(244, 251, 255, 0.08), 0 18px 42px rgba(1, 9, 14, 0.28);
  }

  .nav-icon {
    width: 38px;
    height: 38px;
    display: grid;
    place-items: center;
    border-radius: 13px;
    background: rgba(244, 251, 255, 0.08);
    color: var(--wa-text-main, #d8e5ec);
    font-size: 1rem;
    font-weight: 900;
  }

  .nav-primary.active .nav-icon {
    background: var(--wa-accent, #26ddff);
    color: var(--wa-accent-ink, #021318);
  }

  .nav-copy {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .nav-copy strong {
    font-size: 0.9rem;
    font-weight: 760;
  }

  .nav-copy small {
    color: var(--wa-text-muted, #90a8b5);
    font-size: 0.68rem;
  }

  .nav-primary.active .nav-copy small {
    color: var(--wa-text-main, #d8e5ec);
  }

  .rail-subnav {
    display: grid;
    gap: 5px;
    margin: 0 0 4px 19px;
    padding-left: 12px;
    border-left: 1px solid rgba(203, 234, 244, 0.14);
  }

  .subnav-button {
    width: 100%;
    min-height: 42px;
    display: grid;
    grid-template-columns: 7px minmax(0, 1fr);
    align-items: center;
    gap: 9px;
    padding: 8px 9px;
    border: 1px solid transparent;
    border-radius: 10px;
    background: transparent;
    color: var(--wa-text-main, #d8e5ec);
    cursor: pointer;
    text-align: left;
    transition: background 0.16s ease, border-color 0.16s ease, color 0.16s ease;
  }

  .subnav-button:hover {
    background: rgba(244, 251, 255, 0.055);
    color: var(--wa-text-strong, #f4fbff);
  }

  .subnav-button.active {
    border-color: rgba(203, 234, 244, 0.18);
    background: rgba(244, 251, 255, 0.09);
    color: var(--wa-text-strong, #f4fbff);
  }

  .subnav-dot {
    width: 7px;
    height: 7px;
    border-radius: 999px;
    background: rgba(203, 234, 244, 0.34);
  }

  .subnav-button.active .subnav-dot {
    background: var(--wa-accent, #26ddff);
    box-shadow: 0 0 12px rgba(38, 221, 255, 0.42);
  }

  .subnav-copy {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .subnav-copy strong {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 0.78rem;
    font-weight: 760;
  }

  .subnav-copy small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--wa-text-muted, #90a8b5);
    font-size: 0.63rem;
  }

  .rail-status {
    margin-top: auto;
    padding: 16px;
    border-radius: 18px;
    border: 1px solid var(--wa-border-soft, rgba(203, 234, 244, 0.14));
    background: rgba(244, 251, 255, 0.055);
    display: grid;
    gap: 5px;
  }

  .status-kicker,
  .module-kicker {
    color: var(--wa-text-subtle, #5f7582);
    font-size: 0.66rem;
    font-weight: 850;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  .rail-status .status-kicker {
    color: var(--wa-accent, #26ddff);
  }

  .rail-status strong {
    font-size: 0.92rem;
  }

  .rail-status span:last-child {
    color: var(--wa-text-muted, #90a8b5);
    font-size: 0.75rem;
  }

  .console-main {
    min-width: 0;
    min-height: 100vh;
    padding: 18px 22px 24px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .console-topbar {
    position: sticky;
    top: 0;
    z-index: 50;
    min-height: 78px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 18px;
    padding: 12px 14px 12px 22px;
    border: 1px solid var(--wa-border-soft, rgba(203, 234, 244, 0.14));
    border-radius: var(--wa-radius-xl, 18px);
    background:
      linear-gradient(145deg, rgba(244, 251, 255, 0.075), rgba(244, 251, 255, 0.025)),
      rgba(12, 18, 24, 0.78);
    box-shadow: var(--wa-shadow-sm, 0 10px 28px rgba(1, 9, 14, 0.26));
    backdrop-filter: blur(24px) saturate(1.2);
    -webkit-backdrop-filter: blur(24px) saturate(1.2);
  }

  .title-cluster {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 14px;
  }

  .title-cluster h1 {
    margin: 2px 0 0;
    color: var(--wa-text-strong, #f4fbff);
    font-size: 24px;
    line-height: 1.1;
    letter-spacing: 0;
  }

  .topbar-menu {
    width: 42px;
    height: 42px;
    display: grid;
    place-items: center;
    flex: 0 0 42px;
    border: 1px solid var(--wa-border-soft, rgba(121, 139, 159, 0.18));
    border-radius: var(--wa-radius-lg, 8px);
    background: rgba(255, 255, 255, 0.78);
    color: var(--wa-text-main, #293847);
    cursor: pointer;
  }

  .topbar-actions {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
  }

  .global-search {
    position: relative;
    z-index: 30;
    width: min(360px, 28vw);
    min-width: 220px;
    height: 46px;
    display: flex;
    align-items: center;
    gap: 9px;
    padding: 0 14px;
    border-radius: 16px;
    border: 1px solid var(--wa-border-soft, rgba(203, 234, 244, 0.14));
    background: rgba(7, 11, 16, 0.58);
    color: var(--wa-text-muted, #90a8b5);
  }

  .global-search input {
    width: 100%;
    min-width: 0;
    border: 0;
    outline: 0;
    background: transparent;
    color: var(--wa-text-strong, #f4fbff);
    font-size: 0.86rem;
  }

  .global-search input::placeholder {
    color: var(--wa-text-subtle, #5f7582);
  }

  .search-label {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  .global-search-submit {
    flex: 0 0 auto;
    min-width: 50px;
    height: 30px;
    border: 0;
    border-radius: 999px;
    padding: 0 11px;
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.1));
    color: var(--wa-accent-strong, #006f76);
    font: inherit;
    font-size: 12px;
    font-weight: 780;
    cursor: pointer;
    transition: background 160ms ease, color 160ms ease, transform 160ms ease;
  }

  .global-search-submit:hover:not(:disabled),
  .global-search-submit:focus-visible {
    background: var(--wa-accent, #008f96);
    color: var(--wa-accent-ink, #ffffff);
    outline: none;
  }

  .global-search-submit:active:not(:disabled) {
    transform: scale(0.97);
  }

  .global-search-submit:disabled {
    opacity: 0.58;
    cursor: wait;
  }

  .global-search:focus-within {
    border-color: var(--wa-border-focus, rgba(0, 143, 150, 0.46));
    background: #ffffff;
    box-shadow: 0 0 0 3px rgba(0, 143, 150, 0.1), 0 12px 28px rgba(26, 41, 58, 0.08);
  }

  .global-search-results {
    position: absolute;
    top: calc(100% + 9px);
    left: 0;
    width: 100%;
    min-width: min(520px, calc(100vw - 24px));
    max-height: min(440px, calc(100dvh - 92px));
    overflow-y: auto;
    border: 1px solid rgba(121, 139, 159, 0.2);
    border-radius: 14px;
    padding: 7px;
    background: rgba(255, 255, 255, 0.98);
    box-shadow: 0 24px 54px rgba(26, 41, 58, 0.16);
    color: var(--wa-text-main, #293847);
  }

  .global-search-results-head {
    min-height: 30px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 8px;
    color: var(--wa-text-muted, #667789);
    font-size: 11px;
    font-weight: 760;
  }

  .global-search-results-head strong {
    color: var(--wa-text-strong, #0d1722);
    font-variant-numeric: tabular-nums;
  }

  .global-search-result {
    width: 100%;
    min-height: 52px;
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: 9px;
    border: 0;
    border-radius: 10px;
    padding: 7px 9px;
    background: transparent;
    color: var(--wa-text-main, #293847);
    text-align: left;
    cursor: pointer;
  }

  .global-search-result:hover,
  .global-search-result:focus-visible {
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.1));
    outline: none;
  }

  .search-result-type,
  .search-result-route {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 24px;
    border-radius: 999px;
    padding: 0 8px;
    background: var(--wa-surface-inset, #f2f6f9);
    color: var(--wa-text-muted, #667789);
    font-size: 10px;
    font-weight: 800;
    white-space: nowrap;
  }

  .search-result-route {
    color: var(--wa-accent-strong, #006f76);
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.1));
  }

  .search-result-copy {
    min-width: 0;
    display: grid;
    gap: 3px;
  }

  .search-result-copy strong,
  .search-result-copy small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .search-result-copy strong {
    color: var(--wa-text-strong, #0d1722);
    font-size: 13px;
  }

  .search-result-copy small,
  .global-search-empty {
    color: var(--wa-text-muted, #667789);
    font-size: 11px;
  }

  .global-search-empty {
    padding: 18px 10px;
    text-align: center;
  }

  .notification-wrap,
  .profile-wrap {
    position: relative;
  }

  .icon-button {
    position: relative;
    width: 46px;
    height: 46px;
    border: 1px solid var(--wa-border-soft, rgba(203, 234, 244, 0.14));
    border-radius: 16px;
    background: rgba(7, 11, 16, 0.58);
    color: var(--wa-text-strong, #f4fbff);
    display: grid;
    place-items: center;
    cursor: pointer;
    font-size: 1.1rem;
  }

  .icon-button.has-unread {
    border-color: rgba(255, 97, 119, 0.34);
    background: rgba(255, 97, 119, 0.12);
  }

  .icon-button em {
    position: absolute;
    top: -5px;
    right: -5px;
    min-width: 18px;
    height: 18px;
    padding: 0 5px;
    border-radius: 999px;
    background: var(--wa-danger, #ff6177);
    color: #170207;
    border: 2px solid var(--wa-bg-base, #06090d);
    font-size: 0.62rem;
    font-style: normal;
    font-weight: 850;
    display: grid;
    place-items: center;
  }

  .profile-button {
    height: 46px;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 5px 12px 5px 6px;
    border-radius: 18px;
    border: 0;
    background: rgba(244, 251, 255, 0.08);
    color: var(--wa-text-strong, #f4fbff);
    cursor: pointer;
  }

  .avatar {
    width: 34px;
    height: 34px;
    flex: 0 0 34px;
    border-radius: 13px;
    overflow: hidden;
    display: grid;
    place-items: center;
    background: var(--wa-accent, #26ddff);
    color: var(--wa-accent-ink, #021318);
    font-weight: 820;
    font-size: 0.86rem;
  }

  .avatar.large {
    width: 48px;
    height: 48px;
    flex-basis: 48px;
    border-radius: 17px;
    font-size: 1rem;
  }

  .avatar img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .profile-copy {
    display: flex;
    flex-direction: column;
    gap: 1px;
    min-width: 0;
    text-align: left;
  }

  .profile-copy strong {
    max-width: 116px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 0.82rem;
  }

  .profile-copy small {
    color: var(--wa-text-muted, #90a8b5);
    font-size: 0.68rem;
  }

  .popover {
    position: absolute;
    top: calc(100% + 12px);
    right: 0;
    z-index: 80;
    width: min(390px, calc(100vw - 48px));
    border: 1px solid var(--wa-border-soft, rgba(203, 234, 244, 0.14));
    border-radius: var(--wa-radius-xl, 18px);
    background: rgba(12, 18, 24, 0.9);
    box-shadow: var(--wa-shadow-md, 0 22px 58px rgba(1, 9, 14, 0.38));
    backdrop-filter: blur(24px) saturate(1.18);
    -webkit-backdrop-filter: blur(24px) saturate(1.18);
  }

  .popover-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 16px 16px 10px;
  }

  .popover-head div {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .popover-head strong {
    font-size: 0.94rem;
  }

  .popover-head span {
    color: var(--wa-text-muted, #90a8b5);
    font-size: 0.74rem;
  }

  .popover-head button,
  .alert-row-actions button,
  .alert-row-actions a {
    border: 0;
    border-radius: 999px;
    background: var(--wa-accent, #26ddff);
    color: var(--wa-accent-ink, #021318);
    padding: 7px 10px;
    font-size: 0.72rem;
    font-weight: 760;
    text-decoration: none;
    cursor: pointer;
  }

  .alert-list {
    max-height: min(520px, 70vh);
    overflow: auto;
    padding: 0 10px 10px;
    display: grid;
    gap: 8px;
  }

  .alert-row {
    padding: 12px;
    border-radius: 16px;
    background: rgba(7, 11, 16, 0.62);
    border: 1px solid var(--wa-border-soft, rgba(203, 234, 244, 0.14));
  }

  .alert-row-head,
  .alert-row-actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }

  .alert-row-head span {
    color: var(--wa-accent, #26ddff);
    font-size: 0.7rem;
    font-weight: 850;
  }

  .alert-row-head small {
    color: var(--wa-text-subtle, #5f7582);
    font-size: 0.68rem;
  }

  .alert-row strong {
    display: block;
    margin-top: 8px;
    color: var(--wa-text-strong, #f4fbff);
    font-size: 0.86rem;
    line-height: 1.35;
  }

  .alert-row p {
    margin: 5px 0 11px;
    color: var(--wa-text-muted, #90a8b5);
    font-size: 0.76rem;
    line-height: 1.5;
  }

  .alert-row-actions {
    justify-content: flex-end;
  }

  .alert-row-actions button {
    background: rgba(244, 251, 255, 0.08);
    color: var(--wa-text-main, #d8e5ec);
  }

  .empty-state {
    padding: 22px;
    color: var(--wa-text-muted, #90a8b5);
    text-align: center;
    font-size: 0.84rem;
  }

  .profile-popover {
    padding: 14px;
    border: 0;
    max-height: min(720px, calc(100dvh - 92px));
    overflow-y: auto;
    overscroll-behavior: contain;
    scrollbar-width: none;
  }

  .profile-popover::-webkit-scrollbar {
    display: none;
  }

  .profile-card-head {
    display: flex;
    align-items: center;
    gap: 12px;
    padding-bottom: 12px;
    border-bottom: 0;
  }

  .profile-card-head div {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .profile-card-head strong {
    color: var(--wa-text-strong, #f4fbff);
  }

  .profile-card-head span {
    color: var(--wa-text-muted, #90a8b5);
    font-size: 0.78rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .profile-meta {
    margin: 8px 0 14px;
    display: grid;
    gap: 2px;
  }

  .profile-meta div {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 7px 0;
    border-top: 1px solid rgba(203, 234, 244, 0.08);
    background: transparent;
  }

  .profile-meta dt {
    color: var(--wa-text-muted, #90a8b5);
    font-size: 0.72rem;
  }

  .profile-meta dd {
    margin: 0;
    color: var(--wa-text-strong, #f4fbff);
    font-size: 0.78rem;
    font-weight: 760;
  }

  .logout-button {
    width: 100%;
    min-height: 42px;
    margin-top: 14px;
    border: 0;
    border-radius: 14px;
    background: var(--wa-danger, #ff6177);
    color: #170207;
    cursor: pointer;
    font-weight: 820;
  }

  .degraded-banner {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 14px;
    border-radius: 18px;
    border: 1px solid rgba(255, 197, 95, 0.24);
    background: rgba(255, 197, 95, 0.12);
    color: var(--wa-warning, #ffc55f);
  }

  .degraded-banner strong {
    font-size: 0.68rem;
    letter-spacing: 0.12em;
  }

  .degraded-banner span {
    font-size: 0.82rem;
  }

  .workspace-frame {
    flex: 1;
    min-width: 0;
    min-height: 0;
    overflow: auto;
    border: 1px solid var(--wa-border-soft, rgba(203, 234, 244, 0.14));
    border-radius: var(--wa-radius-xl, 18px);
    background: rgba(7, 11, 16, 0.58);
    box-shadow: var(--wa-shadow-panel, inset 0 1px 0 rgba(244, 251, 255, 0.065), 0 24px 68px rgba(1, 9, 14, 0.42));
    backdrop-filter: blur(20px) saturate(1.16);
    -webkit-backdrop-filter: blur(20px) saturate(1.16);
  }

  :global(.workspace-frame > *) {
    min-width: 0;
  }

  :global(.workspace-frame .glass-panel) {
    box-shadow: var(--wa-shadow-sm, 0 10px 28px rgba(1, 9, 14, 0.26));
  }

  @media (max-width: 1180px) {
    .functional-console {
      grid-template-columns: 94px minmax(0, 1fr);
    }

    .console-rail {
      padding: 16px 12px;
    }

    .brand-copy,
    .nav-copy,
    .rail-status span,
    .rail-status strong,
    .rail-status .status-kicker {
      display: none;
    }

    .brand-block {
      justify-content: center;
      padding-inline: 0;
    }

    .nav-primary {
      grid-template-columns: 1fr;
      justify-items: center;
      padding: 9px;
    }

    .rail-subnav {
      display: none;
    }
  }

  @media (max-width: 860px) {
    .functional-console {
      grid-template-columns: 1fr;
    }

    .console-rail {
      position: static;
      height: auto;
      border-right: 0;
      border-bottom: 1px solid var(--wa-border-soft, rgba(203, 234, 244, 0.14));
    }

    .brand-copy,
    .nav-copy {
      display: flex;
    }

    .rail-nav {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .nav-primary {
      grid-template-columns: 38px minmax(0, 1fr);
      justify-items: stretch;
    }

    .rail-subnav {
      display: grid;
      margin-left: 19px;
    }

    .rail-status {
      display: none;
    }

    .console-main {
      padding: 12px;
    }

    .console-topbar {
      position: static;
      align-items: stretch;
      flex-direction: column;
    }

    .topbar-actions {
      flex-wrap: wrap;
    }

    .global-search {
      width: 100%;
      min-width: 0;
      order: 3;
    }

    .profile-copy {
      display: none;
    }

    .workspace-frame {
      border-radius: 18px;
    }
  }

  @media (max-width: 560px) {
    .rail-nav {
      grid-template-columns: 1fr;
    }

    .popover {
      right: auto;
      left: 0;
      width: calc(100vw - 24px);
    }
  }

  /* Phase 41: light admin workbench with a dark well-ambient rail. */
  :global(body) {
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.62), transparent 18rem),
      var(--wa-bg-page, #eef3f8);
    color: var(--wa-text-main, #293847);
  }

  :global(body)::before {
    opacity: 0;
  }

  .functional-console {
    grid-template-columns: var(--wa-rail-w, 270px) minmax(0, 1fr);
    color: var(--wa-text-main, #293847);
    background: var(--wa-bg-page, #eef3f8);
  }

  .console-rail {
    padding: 24px 16px;
    background:
      linear-gradient(180deg, rgba(2, 11, 19, 0.98), rgba(5, 18, 30, 0.98)),
      var(--wa-bg-rail, #020b13);
    color: var(--wa-rail-text-main, #d4e5f2);
    border-right: 1px solid rgba(255, 255, 255, 0.08);
    box-shadow: 14px 0 34px rgba(15, 30, 45, 0.16);
  }

  .brand-block {
    padding: 4px 8px 18px;
    border-bottom-color: rgba(255, 255, 255, 0.08);
  }

  .brand-mark {
    width: 40px;
    height: 40px;
    border-radius: 12px;
    box-shadow: none;
  }

  .brand-copy strong {
    color: var(--wa-rail-text-strong, #f6fbff);
    font-size: 1.42rem;
    line-height: 1;
    letter-spacing: 0;
  }

  .brand-copy span {
    color: var(--wa-rail-text-muted, #87a0b7);
    letter-spacing: 0;
  }

  .rail-nav {
    gap: 6px;
  }

  .nav-primary {
    min-height: 56px;
    border-radius: var(--wa-radius-md, 8px);
    color: var(--wa-rail-text-main, #d4e5f2);
  }

  .nav-primary:hover {
    transform: none;
    background: rgba(255, 255, 255, 0.06);
    color: var(--wa-rail-text-strong, #f6fbff);
  }

  .nav-primary.active {
    background: linear-gradient(90deg, rgba(0, 143, 150, 0.82), rgba(0, 143, 150, 0.42));
    border-color: rgba(34, 211, 216, 0.38);
    box-shadow: inset 3px 0 0 #22d3d8;
    color: var(--wa-rail-text-strong, #f6fbff);
  }

  .nav-icon {
    width: 19px;
    height: 19px;
    margin: 0 auto;
    border-radius: 0;
    background: currentColor;
    color: currentColor;
  }

  .nav-primary.active .nav-icon {
    background: currentColor;
    color: var(--wa-rail-text-strong, #f6fbff);
  }

  .nav-copy small,
  .nav-primary.active .nav-copy small,
  .rail-status span:last-child {
    color: var(--wa-rail-text-muted, #87a0b7);
  }

  .rail-subnav {
    border-left-color: rgba(34, 211, 216, 0.2);
  }

  .subnav-button {
    border-radius: var(--wa-radius-md, 8px);
    color: var(--wa-rail-text-main, #d4e5f2);
  }

  .subnav-button:hover {
    background: rgba(255, 255, 255, 0.055);
    color: var(--wa-rail-text-strong, #f6fbff);
  }

  .subnav-button.active {
    border-color: rgba(34, 211, 216, 0.28);
    background: rgba(34, 211, 216, 0.12);
    color: var(--wa-rail-text-strong, #f6fbff);
  }

  .subnav-copy small {
    color: var(--wa-rail-text-muted, #87a0b7);
  }

  .rail-status {
    border-radius: var(--wa-radius-lg, 10px);
    border-color: rgba(255, 255, 255, 0.1);
    background: rgba(255, 255, 255, 0.05);
  }

  .rail-status strong {
    color: var(--wa-rail-text-strong, #f6fbff);
  }

  .status-kicker,
  .module-kicker {
    letter-spacing: 0;
    text-transform: none;
  }

  .console-main {
    padding: 0;
    gap: 0;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.84) 0, rgba(255, 255, 255, 0.42) 180px, transparent 460px),
      var(--wa-bg-page, #eef4f7);
  }

  .console-topbar {
    min-height: 68px;
    width: 100%;
    display: grid;
    grid-template-columns: minmax(220px, 1fr) minmax(320px, 520px) minmax(260px, 1fr);
    align-items: center;
    padding: 10px 22px;
    border: 0;
    border-bottom: 1px solid rgba(121, 139, 159, 0.18);
    border-radius: 0;
    background: rgba(255, 255, 255, 0.88);
    box-shadow: 0 8px 22px rgba(26, 41, 58, 0.065);
    -webkit-backdrop-filter: blur(18px) saturate(132%);
    backdrop-filter: blur(18px) saturate(132%);
  }

  .title-cluster h1 {
    color: var(--wa-text-strong, #0d1722);
    font-size: 22px;
  }

  .module-kicker {
    color: var(--wa-text-muted, #667789);
  }

  .global-search,
  .icon-button,
  .topbar-menu {
    border-color: rgba(121, 139, 159, 0.18);
    border-radius: var(--wa-radius-pill, 999px);
    background: rgba(255, 255, 255, 0.82);
    color: var(--wa-text-main, #293847);
    box-shadow: 0 8px 22px rgba(26, 41, 58, 0.06);
  }

  .global-search {
    width: min(520px, 100%);
    height: 42px;
    justify-self: center;
  }

  .topbar-actions {
    justify-self: end;
  }

  .profile-button {
    min-width: 170px;
    height: 42px;
    border: 0;
    background: transparent;
    box-shadow: none;
  }

  .profile-button:hover {
    background: rgba(0, 143, 150, 0.08);
  }

  .global-search .wa-icon,
  .icon-button .wa-icon,
  .topbar-menu .wa-icon {
    width: 18px;
    height: 18px;
  }

  .help-button {
    display: grid;
  }

  .global-search input {
    color: var(--wa-text-strong, #0d1722);
  }

  .global-search input::placeholder,
  .profile-copy small {
    color: var(--wa-text-subtle, #8a99aa);
  }

  .profile-copy strong {
    color: var(--wa-text-strong, #0d1722);
  }

  .avatar {
    border-radius: 50%;
    background: linear-gradient(135deg, #dce9f2, #ffffff);
    color: var(--wa-text-strong, #0d1722);
    border: 1px solid rgba(121, 139, 159, 0.18);
  }

  .icon-button.has-unread {
    border-color: rgba(221, 75, 62, 0.25);
    background: rgba(221, 75, 62, 0.08);
  }

  .icon-button em {
    border-color: #ffffff;
    background: var(--wa-danger, #dd4b3e);
    color: #ffffff;
  }

  .popover {
    border-color: rgba(121, 139, 159, 0.18);
    border-radius: var(--wa-radius-xl, 12px);
    background: rgba(255, 255, 255, 0.94);
    color: var(--wa-text-main, #293847);
    box-shadow: 0 22px 56px rgba(26, 41, 58, 0.16);
  }

  .profile-popover {
    border: 0;
    background: rgba(255, 255, 255, 0.96);
    box-shadow: 0 20px 48px rgba(26, 41, 58, 0.14);
  }

  .popover-head strong,
  .alert-row strong,
  .profile-card-head strong,
  .profile-meta dd {
    color: var(--wa-text-strong, #0d1722);
  }

  .popover-head span,
  .alert-row-head small,
  .alert-row p,
  .profile-card-head span,
  .profile-meta dt,
  .empty-state {
    color: var(--wa-text-muted, #667789);
  }

  .alert-row {
    border-color: rgba(121, 139, 159, 0.14);
    background: rgba(247, 250, 252, 0.82);
  }

  .profile-meta div {
    border-color: rgba(121, 139, 159, 0.12);
    background: transparent;
  }

  .popover-head button,
  .alert-row-actions a {
    background: var(--wa-accent, #008f96);
    color: var(--wa-accent-ink, #ffffff);
  }

  .alert-row-actions button {
    background: rgba(102, 119, 137, 0.1);
    color: var(--wa-text-main, #293847);
  }

  .logout-button {
    border-radius: var(--wa-radius-md, 8px);
    background: var(--wa-danger, #dd4b3e);
    color: #ffffff;
  }

  .degraded-banner {
    border-radius: var(--wa-radius-lg, 10px);
    border-color: rgba(216, 135, 0, 0.22);
    background: rgba(216, 135, 0, 0.12);
    color: var(--wa-warning, #d88700);
  }

  .degraded-banner strong {
    letter-spacing: 0;
  }

  .workspace-frame {
    --wa-content-max: 1720px;
    flex: 1;
    overflow: auto;
    padding: 14px 16px 18px;
    border: 0;
    border-radius: 0;
    background: transparent;
    box-shadow: none;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }

  :global(.workspace-frame .glass-panel) {
    box-shadow: var(--wa-shadow-sm, 0 10px 24px rgba(26, 41, 58, 0.08));
  }

  @media (max-width: 1180px) {
    .functional-console {
      grid-template-columns: 94px minmax(0, 1fr);
    }

    .brand-copy,
    .nav-copy,
    .rail-status span,
    .rail-status strong,
    .rail-status .status-kicker {
      display: none;
    }

    .brand-block {
      justify-content: center;
      padding-inline: 0;
    }

    .nav-primary {
      grid-template-columns: 1fr;
      justify-items: center;
      padding: 9px;
    }

    .rail-subnav {
      display: none;
    }
  }

  @media (max-width: 860px) {
    .functional-console {
      grid-template-columns: 1fr;
    }

    .console-rail {
      position: static;
      height: auto;
      border-right: 0;
      border-bottom: 1px solid rgba(255, 255, 255, 0.08);
    }

    .brand-copy,
    .nav-copy {
      display: flex;
    }

    .rail-nav {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .nav-primary {
      grid-template-columns: 38px minmax(0, 1fr);
      justify-items: stretch;
    }

    .rail-subnav {
      display: grid;
    }

    .rail-status {
      display: none;
    }

    .console-main {
      padding: 0;
    }

    .console-topbar {
      position: static;
      align-items: stretch;
      grid-template-columns: 1fr;
      border-radius: 0;
    }

    .topbar-actions {
      flex-wrap: wrap;
      justify-self: stretch;
      justify-content: flex-end;
    }

    .global-search {
      width: 100%;
      min-width: 0;
      order: 3;
    }

    .profile-copy {
      display: none;
    }

    .workspace-frame {
      padding: 12px;
    }
  }

  @media (max-width: 560px) {
    .rail-nav {
      grid-template-columns: 1fr;
    }

    .popover {
      right: auto;
      left: 0;
      width: calc(100vw - 24px);
    }
  }

  /* Strongest-brain shell contract: one parent owns page gutter, surface and scroll. */
  .functional-console {
    --wa-shell-gutter: clamp(14px, 1.25vw, 22px);
    --wa-shell-bottom-gap: max(var(--wa-shell-gutter), env(safe-area-inset-bottom, 0px));
    --wa-content-max: 1720px;
    --wa-workspace-topbar-h: 68px;
    --wa-workspace-min-h: calc(100dvh - var(--wa-workspace-topbar-h) - (var(--wa-shell-gutter) * 2));
    --wa-workspace-inline-start: var(--wa-rail-w, 270px);
    min-width: 0;
    overflow: hidden;
    background: var(--wa-bg-ambient), var(--wa-bg-page, #e8f0f4);
  }

  .console-main {
    min-width: 0;
    height: 100vh;
    height: 100dvh;
    overflow: hidden;
    background: var(--wa-bg-ambient), var(--wa-bg-page, #e8f0f4);
  }

  .console-topbar {
    flex: 0 0 var(--wa-workspace-topbar-h);
  }

  .workspace-stage {
    position: relative;
    isolation: isolate;
    flex: 1 1 auto;
    min-width: 0;
    min-height: 0;
    overflow: hidden;
  }

  .workspace-frame {
    height: 100%;
    min-width: 0;
    min-height: 0;
    width: 100%;
    overflow-x: hidden;
    overflow-y: auto;
    scrollbar-gutter: auto;
    overflow-anchor: none;
    padding: var(--wa-shell-gutter);
    padding-bottom: var(--wa-shell-bottom-gap);
    background: var(--wa-bg-ambient), var(--wa-bg-page, #e8f0f4);
  }

  .workspace-frame.flow-frame {
    overflow-y: hidden;
  }

  .workspace-frame.viewport-fit-frame {
    overflow-y: hidden;
    scrollbar-gutter: auto;
  }

  :global(.workspace-frame > *) {
    width: 100%;
    min-width: 0;
  }

  @media (max-width: 860px) {
    .functional-console {
      --wa-workspace-topbar-h: auto;
      overflow: visible;
    }

    .console-main {
      height: auto;
      min-height: 100dvh;
      overflow: visible;
    }

    .workspace-frame {
      overflow: visible;
      scrollbar-gutter: auto;
      padding: var(--wa-shell-gutter);
    }

    .workspace-frame.flow-frame {
      overflow: visible;
    }
  }

  /* Phase 46 responsive shell: compact desktop rail and off-canvas mobile navigation. */
  .rail-backdrop {
    display: none;
  }

  .functional-console.rail-collapsed {
    --wa-workspace-inline-start: 94px;
    grid-template-columns: 94px minmax(0, 1fr);
  }

  .rail-collapsed .console-rail {
    padding-inline: 12px;
  }

  .rail-collapsed .brand-block {
    justify-content: center;
    padding-inline: 0;
  }

  .rail-collapsed .brand-copy,
  .rail-collapsed .nav-copy,
  .rail-collapsed .rail-status span,
  .rail-collapsed .rail-status strong,
  .rail-collapsed .rail-status .status-kicker,
  .rail-collapsed .rail-subnav {
    display: none;
  }

  .rail-collapsed .nav-primary {
    grid-template-columns: 1fr;
    justify-items: center;
    padding-inline: 9px;
  }

  @media (max-width: 860px) {
    .functional-console,
    .functional-console.rail-collapsed {
      --wa-workspace-inline-start: 0px;
      grid-template-columns: minmax(0, 1fr);
      height: 100dvh;
      min-height: 100dvh;
      overflow: hidden;
    }

    .console-rail {
      position: fixed !important;
      inset: 0 auto 0 0;
      z-index: 130;
      width: min(320px, 88vw);
      height: 100dvh !important;
      padding: 16px 14px !important;
      border-right: 1px solid rgba(255, 255, 255, 0.1) !important;
      border-bottom: 0 !important;
      transform: translateX(-102%);
      transition: transform 180ms cubic-bezier(0.2, 0, 0, 1);
      box-shadow: 20px 0 52px rgba(2, 11, 19, 0.24);
    }

    .console-rail.mobile-open {
      transform: translateX(0);
    }

    .rail-backdrop {
      position: fixed;
      inset: 0;
      z-index: 120;
      display: block;
      padding: 0;
      border: 0;
      border-radius: 0;
      background: rgba(15, 27, 38, 0.24);
      backdrop-filter: blur(6px);
      -webkit-backdrop-filter: blur(6px);
    }

    .brand-copy,
    .nav-copy,
    .rail-collapsed .brand-copy,
    .rail-collapsed .nav-copy {
      display: flex;
    }

    .brand-block {
      justify-content: flex-start;
      padding-inline: 8px;
    }

    .rail-nav {
      grid-template-columns: minmax(0, 1fr) !important;
      overflow-y: auto;
    }

    .nav-primary,
    .rail-collapsed .nav-primary {
      grid-template-columns: 38px minmax(0, 1fr);
      justify-items: stretch;
      padding-inline: 12px;
    }

    .rail-subnav,
    .rail-collapsed .rail-subnav {
      display: grid;
      margin-left: 19px;
    }

    .console-main {
      height: 100dvh !important;
      min-height: 0 !important;
      overflow: hidden !important;
    }

    .console-topbar {
      position: relative !important;
      min-height: 64px;
      display: grid !important;
      grid-template-columns: minmax(0, 1fr) auto !important;
      align-items: center !important;
      padding: 8px 12px !important;
    }

    .title-cluster h1 {
      font-size: 18px;
    }

    .module-kicker {
      font-size: 10px;
    }

    .global-search,
    .help-button,
    .profile-copy {
      display: none !important;
    }

    .topbar-actions {
      grid-column: 2;
      flex-wrap: nowrap;
    }

    .profile-button {
      min-width: 42px;
      padding: 4px;
    }

    .workspace-frame,
    .workspace-frame.flow-frame {
      min-height: 0;
      overflow-x: hidden;
      overflow-y: auto;
      padding: var(--wa-shell-gutter);
      padding-bottom: var(--wa-shell-bottom-gap);
    }

    .workspace-frame.viewport-fit-frame {
      overflow-y: visible;
      scrollbar-gutter: auto;
    }
  }

  @media (max-width: 560px) {
    .profile-popover {
      position: fixed;
      top: 64px;
      right: 12px;
      left: 12px;
      width: auto;
      max-height: calc(100dvh - 76px);
    }
  }

  /* Final viewport container contract: the browser shell never grows with a
     route. Overflow remains inside the fixed workspace hierarchy. */
  @media (max-width: 860px) {
    .functional-console,
    .functional-console.rail-collapsed {
      height: 100dvh !important;
      min-height: 100dvh;
      overflow: hidden !important;
    }

    .console-main {
      height: 100dvh !important;
      min-height: 0 !important;
      overflow: hidden !important;
    }

    .workspace-stage {
      height: auto;
      min-height: 0;
      overflow: hidden;
    }

    .workspace-frame,
    .workspace-frame.flow-frame,
    .workspace-frame.viewport-fit-frame {
      height: 100%;
      min-height: 0;
      overflow-x: hidden;
      overflow-y: auto;
    }

    .workspace-frame.viewport-fit-frame {
      overflow-y: hidden;
    }
  }
</style>
