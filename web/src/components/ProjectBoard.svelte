<script lang="ts">
  import { onMount } from 'svelte';
  import Alert from './shared/Alert.svelte';

  type LaneKey = 'todo' | 'doing' | 'done';

  interface ProjectPreferenceOption {
    project_key: string;
    project_name: string;
  }

  interface ProjectPreferenceResponse {
    mode: 'all' | 'selected';
    project_keys: string[];
    projects: ProjectPreferenceOption[];
  }

  interface ProjectConfig {
    project_key: string;
    project_name: string;
    base_priority: string;
    project_phase: string;
  }

  interface ProjectOption extends ProjectConfig {
    followed: boolean;
  }

  interface ScheduleItem {
    demand_id: string;
    title: string;
    description: string;
    assignee: string;
    department: string;
    repo: string;
    branch: string;
    status: string;
    due_date: string;
    last_update: string;
    issue_type: string;
    risk_level: string;
    risk_label: string;
    risk_reason: string;
    risk_rank: number;
    subtask_total: number;
    subtask_done: number;
    project_key?: string;
    project_priority?: string;
    target_release_id?: number;
    target_release_name?: string;
  }

  interface ScheduleResponse {
    generated_at: string;
    items: ScheduleItem[];
  }

  const lanes: Array<{ key: LaneKey; label: string; helper: string }> = [
    { key: 'todo', label: '待办', helper: '等待进入处理' },
    { key: 'doing', label: '处理中', helper: '开发与评审' },
    { key: 'done', label: '完成', helper: '已经交付闭环' }
  ];

  let projects: ProjectOption[] = [];
  let scheduleItems: ScheduleItem[] = [];
  let selectedProjectKey = '';
  let expandedItemID = '';
  let generatedAt = '';
  let jiraBaseURL = '';
  let loading = true;
  let refreshing = false;
  let errorMessage = '';
  let boardRequestSequence = 0;
  let projectRail: HTMLDivElement;

  $: selectedProject = projects.find((project) => project.project_key === selectedProjectKey) || null;
  $: selectedItems = scheduleItems
    .filter((item) => projectKeyForItem(item) === selectedProjectKey)
    .sort(compareScheduleItems);
  $: laneItems = {
    todo: selectedItems.filter((item) => laneForItem(item) === 'todo'),
    doing: selectedItems.filter((item) => laneForItem(item) === 'doing'),
    done: selectedItems.filter((item) => laneForItem(item) === 'done')
  };
  $: projectCounts = {
    todo: laneItems.todo.length,
    doing: laneItems.doing.length,
    done: laneItems.done.length
  };

  function normalizedProjectKey(value: string): string {
    return (value || '').trim().toUpperCase();
  }

  function priorityRank(value: string): number {
    const normalized = (value || '').trim().toUpperCase();
    if (normalized === 'P0') return 0;
    if (normalized === 'P1') return 1;
    if (normalized === 'P2') return 2;
    if (normalized === 'P3') return 3;
    return 4;
  }

  function compareProjects(a: ProjectOption, b: ProjectOption): number {
    const rankDelta = priorityRank(a.base_priority) - priorityRank(b.base_priority);
    if (rankDelta !== 0) return rankDelta;
    return a.project_name.localeCompare(b.project_name, 'zh-CN') || a.project_key.localeCompare(b.project_key);
  }

  function compareScheduleItems(a: ScheduleItem, b: ScheduleItem): number {
    const riskDelta = (b.risk_rank || 0) - (a.risk_rank || 0);
    if (riskDelta !== 0) return riskDelta;
    if (!a.due_date && b.due_date) return 1;
    if (a.due_date && !b.due_date) return -1;
    return (a.due_date || '').localeCompare(b.due_date || '') || a.demand_id.localeCompare(b.demand_id);
  }

  function projectKeyForItem(item: ScheduleItem): string {
    return normalizedProjectKey(item.project_key || '');
  }

  function laneForItem(item: ScheduleItem): LaneKey {
    const status = (item.status || '').trim().toLowerCase();
    if (status === 'done') return 'done';
    if (status === 'progress' || status === 'review') return 'doing';
    return 'todo';
  }

  function isBug(item: ScheduleItem): boolean {
    return ['bug', 'defect', '缺陷', '故障'].includes((item.issue_type || '').trim().toLowerCase());
  }

  function issueTypeLabel(item: ScheduleItem): string {
    return isBug(item) ? 'Bug' : 'Task';
  }

  function statusLabel(status: string): string {
    switch ((status || '').toLowerCase()) {
      case 'progress': return '开发中';
      case 'review': return '评审中';
      case 'done': return '已完成';
      default: return '待处理';
    }
  }

  function formatDate(value: string): string {
    if (!value) return '未排期';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' });
  }

  function formatUpdatedAt(value: string): string {
    if (!value) return '-';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false
    });
  }

  function jiraURL(item: ScheduleItem): string {
    return jiraBaseURL ? `${jiraBaseURL}/browse/${encodeURIComponent(item.demand_id)}` : '';
  }

  function selectProject(projectKey: string) {
    selectedProjectKey = projectKey;
    expandedItemID = '';
  }

  function toggleItem(itemID: string) {
    expandedItemID = expandedItemID === itemID ? '' : itemID;
  }

  function scrollProjectRail(direction: -1 | 1) {
    projectRail?.scrollBy({ left: direction * Math.max(240, projectRail.clientWidth * 0.68), behavior: 'smooth' });
  }

  function buildProjects(
    preferences: ProjectPreferenceResponse,
    configs: ProjectConfig[],
    items: ScheduleItem[]
  ): ProjectOption[] {
    const configByKey = new Map<string, ProjectConfig>();
    for (const config of configs || []) {
      const key = normalizedProjectKey(config.project_key);
      if (!key) continue;
      configByKey.set(key, { ...config, project_key: key });
    }
    const preferenceByKey = new Map<string, ProjectPreferenceOption>();
    for (const project of preferences.projects || []) {
      const key = normalizedProjectKey(project.project_key);
      if (key) preferenceByKey.set(key, { ...project, project_key: key });
    }
    for (const item of items) {
      const key = projectKeyForItem(item);
      if (key && !preferenceByKey.has(key)) {
        preferenceByKey.set(key, { project_key: key, project_name: key });
      }
    }

    const followedKeys = new Set((preferences.project_keys || []).map(normalizedProjectKey));
    const keys = preferences.mode === 'selected' && followedKeys.size > 0
      ? Array.from(followedKeys)
      : Array.from(new Set([...preferenceByKey.keys(), ...configByKey.keys()]));

    return keys.map((key) => {
      const config = configByKey.get(key);
      const preference = preferenceByKey.get(key);
      return {
        project_key: key,
        project_name: preference?.project_name || config?.project_name || key,
        base_priority: config?.base_priority || items.find((item) => projectKeyForItem(item) === key)?.project_priority || 'P2',
        project_phase: config?.project_phase || '交付',
        followed: followedKeys.has(key)
      };
    }).sort(compareProjects);
  }

  async function fetchBoard(options: { silent?: boolean } = {}) {
    const requestSequence = ++boardRequestSequence;
    const silent = options.silent === true && projects.length > 0;
    if (silent) refreshing = true;
    else loading = true;
    errorMessage = '';
    try {
      const [preferencesResponse, projectsResponse, scheduleResponse, jiraResponse] = await Promise.all([
        fetch('/api/me/project-preferences'),
        fetch('/api/projects/config'),
        fetch('/api/schedule'),
        fetch('/api/jira/link-config')
      ]);
      if (!preferencesResponse.ok || !projectsResponse.ok || !scheduleResponse.ok) {
        throw new Error('项目看板数据暂时无法加载');
      }
      const preferences: ProjectPreferenceResponse = await preferencesResponse.json();
      const configs: ProjectConfig[] = await projectsResponse.json();
      const schedule: ScheduleResponse = await scheduleResponse.json();
      if (requestSequence !== boardRequestSequence) return;
      const items = Array.isArray(schedule.items) ? schedule.items : [];
      const nextProjects = buildProjects(preferences, Array.isArray(configs) ? configs : [], items);

      scheduleItems = items;
      projects = nextProjects;
      generatedAt = schedule.generated_at || '';
      if (!nextProjects.some((project) => project.project_key === selectedProjectKey)) {
        selectedProjectKey = nextProjects[0]?.project_key || '';
        expandedItemID = '';
      }
      if (jiraResponse.ok) {
        const jira = await jiraResponse.json();
        jiraBaseURL = (jira?.base_url || '').replace(/\/+$/, '');
      }
    } catch (error: any) {
      if (requestSequence !== boardRequestSequence) return;
      errorMessage = error?.message || '项目看板数据加载失败';
    } finally {
      if (requestSequence === boardRequestSequence) {
        loading = false;
        refreshing = false;
      }
    }
  }

  onMount(() => {
    const handleProjectPreferencesUpdated = () => {
      void fetchBoard();
    };
    const handleJiraProjectionUpdated = () => {
      void fetchBoard({ silent: true });
    };
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') void fetchBoard({ silent: true });
    };
    void fetchBoard();
    const refreshInterval = window.setInterval(() => {
      if (document.visibilityState === 'visible') void fetchBoard({ silent: true });
    }, 15000);
    window.addEventListener('project-preferences-updated', handleProjectPreferencesUpdated);
    window.addEventListener('jira-sync-complete', handleJiraProjectionUpdated);
    window.addEventListener('jira-sync-updated', handleJiraProjectionUpdated);
    window.addEventListener('focus', handleJiraProjectionUpdated);
    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => {
      window.clearInterval(refreshInterval);
      window.removeEventListener('project-preferences-updated', handleProjectPreferencesUpdated);
      window.removeEventListener('jira-sync-complete', handleJiraProjectionUpdated);
      window.removeEventListener('jira-sync-updated', handleJiraProjectionUpdated);
      window.removeEventListener('focus', handleJiraProjectionUpdated);
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  });
</script>

<section class="project-board" aria-label="项目看板">
  <header class="project-switcher-panel">
    <div class="project-switcher-summary">
      <div>
        <span class="section-kicker">Project scope</span>
        <strong>{selectedProject?.project_name || '暂无可见项目'}</strong>
      </div>
      {#if selectedProject}
        <div class="project-meta" aria-label="当前项目摘要">
          <span class="priority priority-{(selectedProject.base_priority || 'P2').toLowerCase()}">{selectedProject.base_priority || 'P2'}</span>
          <span>{selectedProject.project_phase || '交付'}</span>
          <span>{selectedItems.length} 项</span>
        </div>
      {/if}
    </div>

    <div class="project-switcher">
      <button type="button" class="rail-control" aria-label="向前浏览项目" on:click={() => scrollProjectRail(-1)}>‹</button>
      <div class="project-rail" bind:this={projectRail} role="tablist" aria-label="切换项目">
        {#each projects as project (project.project_key)}
          <button
            type="button"
            role="tab"
            class="project-tab"
            class:active={selectedProjectKey === project.project_key}
            aria-selected={selectedProjectKey === project.project_key}
            on:click={() => selectProject(project.project_key)}
          >
            <strong>{project.project_name}</strong>
            {#if project.followed}<small>关注</small>{/if}
          </button>
        {/each}
      </div>
      <button type="button" class="rail-control" aria-label="向后浏览项目" on:click={() => scrollProjectRail(1)}>›</button>
    </div>
  </header>

  <div class="project-board-feedback">
    {#if errorMessage}
      <Alert type="error" message={errorMessage} />
    {/if}
  </div>

  <div class="project-lanes" aria-busy={loading || refreshing}>
    {#each lanes as lane (lane.key)}
      <section class="project-lane lane-{lane.key}" aria-label={lane.label}>
        <header class="lane-header">
          <div>
            <span class="lane-signal" aria-hidden="true"></span>
            <strong>{lane.label}</strong>
            <small>{lane.helper}</small>
          </div>
          <span class="lane-count">{projectCounts[lane.key]}</span>
        </header>

        <div class="lane-list">
          {#if loading}
            {#each Array(3) as _}
              <div class="project-card skeleton-card" aria-hidden="true">
                <span></span><span></span><span></span>
              </div>
            {/each}
          {:else if !selectedProject}
            <div class="lane-empty">
              <strong>没有可见项目</strong>
              <span>可在个人资料中设置关注项目。</span>
            </div>
          {:else if laneItems[lane.key].length === 0}
            <div class="lane-empty">
              <strong>本列暂无事项</strong>
              <span>{selectedProject.project_name} 当前没有{lane.label}内容。</span>
            </div>
          {:else}
            {#each laneItems[lane.key] as item (item.demand_id)}
              <article class="project-card" class:expanded={expandedItemID === item.demand_id}>
                <button
                  type="button"
                  class="card-main"
                  aria-expanded={expandedItemID === item.demand_id}
                  on:click={() => toggleItem(item.demand_id)}
                >
                  <span class="card-topline">
                    <span class="issue-type-mark" class:bug={isBug(item)} aria-label={issueTypeLabel(item)}>
                      <span aria-hidden="true">{isBug(item) ? 'B' : 'T'}</span>
                    </span>
                    <span class="card-id">{item.demand_id}</span>
                    {#if item.target_release_name}
                      <span class="release-badge" title={`版本 ${item.target_release_name}`}>
                        版本 {item.target_release_name}
                      </span>
                    {/if}
                    <span class="risk risk-{item.risk_level || 'safe'}">{item.risk_label || '正常'}</span>
                  </span>
                  <strong class="card-title">{item.title || item.demand_id}</strong>
                  <span class="card-facts">
                    <span title={item.assignee || '未指派'}><small>负责人</small>{item.assignee || '未指派'}</span>
                    <span><small>计划日</small>{formatDate(item.due_date)}</span>
                    <span><small>状态</small>{statusLabel(item.status)}</span>
                  </span>
                  {#if item.subtask_total > 0}
                    <span class="subtask-progress">
                      <span style={`width: ${Math.min(100, Math.round((item.subtask_done / item.subtask_total) * 100))}%`}></span>
                    </span>
                    <small class="subtask-label">子任务 {item.subtask_done} / {item.subtask_total}</small>
                  {/if}
                </button>

                <div class="card-actions" aria-label={`${item.demand_id} 快捷操作`}>
                  <span>{expandedItemID === item.demand_id ? '收起详情' : '点击卡片查看详情'}</span>
                  {#if jiraURL(item)}
                    <a
                      href={jiraURL(item)}
                      target="_blank"
                      rel="noopener noreferrer"
                      aria-label={`在 Jira 查看 ${item.demand_id}`}
                    >Jira ↗</a>
                  {:else}
                    <span class="jira-unavailable">Jira 未配置</span>
                  {/if}
                </div>

                {#if expandedItemID === item.demand_id}
                  <div class="card-detail">
                    <p>{item.description || '暂无事项描述。'}</p>
                    <dl>
                      <div><dt>风险依据</dt><dd>{item.risk_reason || '暂无风险说明'}</dd></div>
                      <div><dt>代码仓库</dt><dd>{item.repo || '-'}</dd></div>
                      <div><dt>分支</dt><dd>{item.branch || '-'}</dd></div>
                      <div><dt>最近更新</dt><dd>{formatUpdatedAt(item.last_update)}</dd></div>
                    </dl>
                  </div>
                {/if}
              </article>
            {/each}
          {/if}
        </div>
      </section>
    {/each}
  </div>

  <p class="project-board-updated" aria-live="polite">
    {#if loading || refreshing}正在刷新项目看板{:else if generatedAt}数据更新于 {formatUpdatedAt(generatedAt)}{:else}项目看板已就绪{/if}
  </p>
</section>

<style>
  .project-board {
    display: grid;
    grid-template-rows: auto auto minmax(0, 1fr) auto;
    gap: 12px;
    width: 100%;
    height: 100%;
    min-height: 0;
    color: var(--wa-text-main, #293847);
  }

  .project-switcher-panel {
    display: grid;
    gap: 10px;
    padding: 14px 16px;
    border: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    border-radius: var(--wa-radius-lg, 14px);
    background: var(--wa-surface-flat, #fbfdfe);
    box-shadow: var(--wa-shadow-sm, 0 8px 24px rgba(30, 46, 64, 0.06));
  }

  .project-switcher-summary,
  .project-meta,
  .project-switcher,
  .lane-header > div,
  .card-topline,
  .card-facts {
    display: flex;
    align-items: center;
  }

  .project-switcher-summary {
    justify-content: space-between;
    gap: 16px;
  }

  .project-switcher-summary > div:first-child {
    min-width: 0;
    display: grid;
    gap: 2px;
  }

  .section-kicker {
    color: var(--wa-text-muted, #667789);
    font-size: 10px;
    font-weight: 780;
    letter-spacing: 0.1em;
    text-transform: uppercase;
  }

  .project-switcher-summary strong {
    overflow: hidden;
    color: var(--wa-text-strong, #0d1722);
    font-size: 16px;
    line-height: 1.3;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .project-meta {
    flex: none;
    gap: 7px;
    color: var(--wa-text-muted, #667789);
    font-size: 11px;
  }

  .project-meta > span {
    min-height: 24px;
    display: inline-flex;
    align-items: center;
    border: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    border-radius: 7px;
    padding: 0 8px;
    background: var(--wa-surface-inset, #f5f8fb);
    font-weight: 700;
  }

  .project-meta .priority-p0 { border-color: rgba(211, 68, 55, 0.28); color: var(--wa-danger, #c9473c); }
  .project-meta .priority-p1 { border-color: rgba(207, 133, 33, 0.3); color: #9b651b; }

  .project-switcher {
    min-width: 0;
    gap: 8px;
  }

  .project-rail {
    min-width: 0;
    display: flex;
    flex: 1 1 auto;
    gap: 8px;
    overflow-x: auto;
    padding: 2px;
    scroll-behavior: smooth;
    scroll-snap-type: x proximity;
    scrollbar-width: none;
  }

  .project-rail::-webkit-scrollbar { display: none; }

  .rail-control {
    flex: none;
    width: 34px;
    height: 34px;
    border: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.2));
    border-radius: 9px;
    background: var(--wa-surface-inset, #f5f8fb);
    color: var(--wa-text-muted, #667789);
    font: 700 22px/1 system-ui, sans-serif;
    cursor: pointer;
  }

  .rail-control:hover,
  .rail-control:focus-visible {
    border-color: var(--wa-border-focus, rgba(0, 143, 150, 0.86));
    outline: none;
    color: var(--wa-accent, #008f96);
  }

  .project-tab {
    position: relative;
    min-width: 168px;
    max-width: 228px;
    min-height: 42px;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: 7px;
    scroll-snap-align: start;
    border: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.2));
    border-radius: 9px;
    padding: 7px 10px;
    background: transparent;
    color: var(--wa-text-main, #293847);
    text-align: left;
    cursor: pointer;
  }

  .project-tab strong {
    overflow: hidden;
    font-size: 12px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .project-tab small {
    border-radius: 5px;
    padding: 2px 4px;
    background: rgba(0, 143, 150, 0.09);
    color: var(--wa-accent, #008f96);
    font-size: 9px;
    font-weight: 760;
  }

  .project-tab:hover,
  .project-tab:focus-visible,
  .project-tab.active {
    border-color: rgba(0, 143, 150, 0.44);
    outline: none;
    background: rgba(0, 143, 150, 0.055);
  }

  .project-tab.active {
    box-shadow: inset 0 -2px 0 var(--wa-accent, #008f96);
  }

  .project-lanes {
    min-height: 0;
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
    overflow: hidden;
  }

  .project-lane {
    min-width: 0;
    min-height: 0;
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    overflow: hidden;
    border: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.18));
    border-radius: var(--wa-radius-lg, 14px);
    background: rgba(250, 253, 255, 0.74);
  }

  .lane-header {
    min-height: 54px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    border-bottom: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.16));
    padding: 10px 12px;
    background: var(--wa-surface-inset, #f5f8fb);
  }

  .lane-header > div {
    min-width: 0;
    flex-wrap: wrap;
    gap: 7px;
  }

  .lane-header strong {
    color: var(--wa-text-strong, #0d1722);
    font-size: 13px;
  }

  .lane-header small {
    color: var(--wa-text-muted, #667789);
    font-size: 10px;
  }

  .lane-signal {
    width: 7px;
    height: 7px;
    flex: none;
    border-radius: 50%;
    background: #8997a5;
  }

  .lane-doing .lane-signal { background: #cf8521; }
  .lane-done .lane-signal { background: #348777; }

  .lane-count {
    min-width: 26px;
    min-height: 24px;
    display: grid;
    flex: none;
    place-items: center;
    border-radius: 7px;
    background: rgba(102, 119, 137, 0.09);
    color: var(--wa-text-main, #293847);
    font-size: 11px;
    font-weight: 800;
  }

  .lane-list {
    min-height: 0;
    display: grid;
    grid-auto-rows: max-content;
    align-content: start;
    gap: 8px;
    overflow-y: auto;
    padding: 10px;
    overscroll-behavior: contain;
    scrollbar-gutter: stable;
  }

  .project-card {
    overflow: hidden;
    border: 1px solid rgba(123, 143, 160, 0.18);
    border-radius: 11px;
    background: var(--wa-surface-flat, #fbfdfe);
    box-shadow: 0 5px 14px rgba(30, 46, 64, 0.045);
  }

  .project-card:hover,
  .project-card.expanded {
    border-color: rgba(0, 143, 150, 0.32);
  }

  .card-main {
    width: 100%;
    display: grid;
    gap: 9px;
    border: 0;
    padding: 12px;
    background: transparent;
    color: inherit;
    text-align: left;
    cursor: pointer;
  }

  .card-main:focus-visible {
    outline: 3px solid rgba(0, 143, 150, 0.14);
    outline-offset: -3px;
  }

  .card-actions {
    min-height: 34px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    border-top: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.13));
    padding: 6px 12px;
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
  }

  .card-actions > span:first-child {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .card-actions a {
    min-width: 54px;
    min-height: 26px;
    display: inline-flex;
    flex: none;
    align-items: center;
    justify-content: center;
    border-radius: 7px;
    color: var(--wa-accent, #008f96);
    font-size: 10px;
    font-weight: 780;
    text-decoration: none;
  }

  .card-actions a:hover,
  .card-actions a:focus-visible {
    outline: 2px solid rgba(0, 143, 150, 0.16);
    outline-offset: 1px;
    background: rgba(0, 143, 150, 0.07);
  }

  .jira-unavailable {
    flex: none;
    color: var(--wa-text-muted, #667789);
  }

  .card-topline {
    min-width: 0;
    gap: 7px;
  }

  .issue-type-mark {
    width: 22px;
    height: 22px;
    display: grid;
    flex: none;
    place-items: center;
    border: 1px solid rgba(43, 105, 179, 0.24);
    border-radius: 6px;
    background: rgba(43, 105, 179, 0.08);
    color: #2b69b3;
    font: 800 10px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  }

  .issue-type-mark.bug {
    border-color: rgba(201, 71, 60, 0.25);
    background: rgba(201, 71, 60, 0.08);
    color: var(--wa-danger, #c9473c);
  }

  .card-id {
    min-width: 0;
    flex: 1 1 auto;
    overflow: hidden;
    color: var(--wa-text-muted, #667789);
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 10px;
    font-weight: 720;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .risk {
    flex: none;
    border-radius: 6px;
    padding: 3px 6px;
    background: rgba(52, 135, 119, 0.09);
    color: #2f776a;
    font-size: 9px;
    font-weight: 760;
  }

  .release-badge {
    max-width: 42%;
    flex: 0 1 auto;
    overflow: hidden;
    border: 1px solid rgba(0, 143, 150, 0.2);
    border-radius: 6px;
    padding: 3px 6px;
    background: rgba(0, 143, 150, 0.07);
    color: var(--wa-accent-strong, #006f76);
    font-size: 9px;
    font-weight: 760;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .risk-critical { background: rgba(201, 71, 60, 0.1); color: var(--wa-danger, #c9473c); }
  .risk-warning { background: rgba(207, 133, 33, 0.11); color: #9b651b; }

  .card-title {
    display: -webkit-box;
    overflow: hidden;
    color: var(--wa-text-strong, #0d1722);
    font-size: 13px;
    line-height: 1.45;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
  }

  .card-facts {
    min-width: 0;
    gap: 8px;
  }

  .card-facts > span {
    min-width: 0;
    display: grid;
    flex: 1 1 0;
    gap: 2px;
    overflow: hidden;
    color: var(--wa-text-main, #293847);
    font-size: 10px;
    font-weight: 690;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .card-facts small {
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
    font-weight: 620;
  }

  .subtask-progress {
    height: 4px;
    overflow: hidden;
    border-radius: 999px;
    background: rgba(102, 119, 137, 0.12);
  }

  .subtask-progress > span {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: var(--wa-accent, #008f96);
  }

  .subtask-label {
    margin-top: -5px;
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
  }

  .card-detail {
    display: grid;
    gap: 10px;
    border-top: 1px solid var(--wa-border-divider, rgba(123, 143, 160, 0.16));
    padding: 11px 12px 12px;
    background: var(--wa-surface-inset, #f5f8fb);
  }

  .card-detail p {
    margin: 0;
    color: var(--wa-text-main, #293847);
    font-size: 11px;
    line-height: 1.55;
  }

  .card-detail dl {
    display: grid;
    gap: 6px;
    margin: 0;
  }

  .card-detail dl > div {
    display: grid;
    grid-template-columns: 64px minmax(0, 1fr);
    gap: 8px;
  }

  .card-detail dt,
  .card-detail dd {
    margin: 0;
    font-size: 10px;
    line-height: 1.4;
  }

  .card-detail dt { color: var(--wa-text-muted, #667789); }
  .card-detail dd { overflow-wrap: anywhere; color: var(--wa-text-main, #293847); }

  .lane-empty {
    min-height: 126px;
    display: grid;
    place-content: center;
    gap: 5px;
    padding: 20px;
    color: var(--wa-text-muted, #667789);
    text-align: center;
  }

  .lane-empty strong { color: var(--wa-text-main, #293847); font-size: 12px; }
  .lane-empty span { font-size: 10px; line-height: 1.5; }

  .skeleton-card {
    display: grid;
    gap: 10px;
    padding: 14px;
  }

  .skeleton-card span {
    height: 10px;
    border-radius: 5px;
    background: linear-gradient(90deg, rgba(123, 143, 160, 0.08), rgba(123, 143, 160, 0.16), rgba(123, 143, 160, 0.08));
    background-size: 200% 100%;
    animation: skeleton-pulse 1.4s ease-in-out infinite;
  }

  .skeleton-card span:nth-child(2) { width: 82%; }
  .skeleton-card span:nth-child(3) { width: 56%; }

  .project-board-updated {
    margin: 0;
    color: var(--wa-text-muted, #667789);
    font-size: 9px;
    text-align: right;
  }

  @keyframes skeleton-pulse {
    from { background-position: 100% 0; }
    to { background-position: -100% 0; }
  }

  @media (max-width: 860px) {
    .project-board {
      height: auto;
      min-height: 0;
    }

    .project-switcher-summary {
      align-items: flex-start;
      flex-direction: column;
      gap: 9px;
    }

    .rail-control {
      width: 44px;
      height: 44px;
    }

    .project-tab {
      min-height: 44px;
    }

    .card-actions a {
      min-width: 64px;
      min-height: 44px;
    }

    .project-lanes {
      grid-template-columns: minmax(0, 1fr);
      overflow: visible;
    }

    .project-lane {
      min-height: 340px;
      overflow: visible;
    }

    .lane-list {
      max-height: 520px;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .project-rail { scroll-behavior: auto; }
    .skeleton-card span { animation: none; }
  }
</style>
