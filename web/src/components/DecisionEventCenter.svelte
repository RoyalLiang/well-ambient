<script lang="ts">
  import { createEventDispatcher, onMount, tick } from 'svelte';
  import { refreshAgendaSummary, subscribeAgendaSummary } from '../lib/agenda-summary';
  import OverlayCloseButton from './shared/OverlayCloseButton.svelte';

  export let openRequest = 0;

  const dispatch = createEventDispatcher<{
    summary: {
      kind: 'automatic' | 'manual';
      kindLabel: string;
      taskId: string;
      title: string;
      message: string;
      dateLabel: string;
      timeLabel: string;
      dateTime: string;
    } | null;
  }>();

  interface AutomaticEvent {
    time: string;
    occurred_at?: string;
    task_id: string;
    message: string;
    assignee?: string;
    commit_id?: string;
    commit_url?: string;
  }

  interface ManualEvent {
    id: number;
    task_id: string;
    actor: string;
    action: string;
    original_value: string;
    override_value: string;
    reason: string;
    created_at: string;
  }

  interface TimelineEvent {
    id: string;
    kind: 'automatic' | 'manual';
    dateKey: string;
    dateLabel: string;
    timeLabel: string;
    dateTime: string;
    sortValue: number;
    taskId: string;
    title: string;
    message: string;
    actor: string;
    commitId?: string;
    commitUrl?: string;
  }

  interface TimelineGroup {
    dateKey: string;
    dateLabel: string;
    events: TimelineEvent[];
  }

  let automaticEvents: AutomaticEvent[] = [];
  let manualEvents: ManualEvent[] = [];
  let coreMembers = new Set<string>();
  let jiraBaseUrl = '';
  let gitlabBaseUrl = '';
  let drawerOpen = false;
  let drawerEl: HTMLElement;
  let closeButton: OverlayCloseButton;
  let handledOpenRequest = openRequest;
  let publishedSummaryKey = '';
  let refreshing = false;

  $: visibleAutomaticEvents = automaticEvents.filter((event) => !event.assignee || isCoreMember(event.assignee));
  $: timelineGroups = buildTimelineGroups(visibleAutomaticEvents, manualEvents);
  $: timelineEvents = timelineGroups.flatMap((group) => group.events);
  $: latestEvent = timelineEvents[0] || null;
  $: latestSummaryKey = latestEvent ? `${latestEvent.id}:${latestEvent.sortValue}` : 'empty';
  $: if (latestSummaryKey !== publishedSummaryKey) {
    publishedSummaryKey = latestSummaryKey;
    dispatch('summary', latestEvent
      ? {
          kind: latestEvent.kind,
          kindLabel: latestEvent.kind === 'automatic' ? '自动流转' : '人工调停',
          taskId: latestEvent.taskId,
          title: latestEvent.title,
          message: latestEvent.message,
          dateLabel: latestEvent.dateLabel,
          timeLabel: latestEvent.timeLabel,
          dateTime: latestEvent.dateTime
        }
      : null);
  }
  $: if (openRequest > handledOpenRequest) {
    handledOpenRequest = openRequest;
    openDrawer();
  }

  function isCoreMember(name: string): boolean {
    if (!name || coreMembers.size === 0) return true;
    return coreMembers.has(name) || coreMembers.has(name.split(' ')[0]);
  }

  function updateCoreMembers(config: any) {
    let users: string[] = [];
    if (config?.jira?.sync_users?.length > 0) {
      users = [...config.jira.sync_users];
    } else if (config?.jira?.custom_jql) {
      const match = config.jira.custom_jql.match(/assignee\s+in\s*\(([^)]+)\)/i);
      if (match?.[1]) {
        users = match[1].split(',').map((name: string) => name.trim().replace(/['"]/g, ''));
      }
    }
    coreMembers = new Set(users.filter((name) => name && name !== '未指派' && name !== '-'));
  }

  function parseTimelineDate(rawValue: string): Date {
    const raw = (rawValue || '').trim();
    const normalized = /^\d{4}-\d{2}-\d{2}\s\d{2}:\d{2}(?::\d{2})?$/.test(raw)
      ? raw.replace(' ', 'T')
      : raw;
    const parsed = new Date(normalized);
    if (raw && !Number.isNaN(parsed.getTime()) && /\d{4}[-/]\d{1,2}[-/]\d{1,2}|T/.test(raw)) {
      return parsed;
    }

    const fallback = new Date();
    const timeMatch = raw.match(/(\d{1,2}):(\d{2})(?::(\d{2}))?/);
    if (timeMatch) {
      fallback.setHours(Number(timeMatch[1]), Number(timeMatch[2]), Number(timeMatch[3] || 0), 0);
      if (fallback.getTime() > Date.now()) fallback.setDate(fallback.getDate() - 1);
    }
    return fallback;
  }

  function dateKey(date: Date): string {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
  }

  function dateLabel(date: Date): string {
    const today = new Date();
    const yesterday = new Date(today.getFullYear(), today.getMonth(), today.getDate() - 1);
    const key = dateKey(date);
    if (key === dateKey(today)) return '今天';
    if (key === dateKey(yesterday)) return '昨天';
    return date.toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' });
  }

  function timeLabel(date: Date): string {
    return date.toLocaleTimeString('zh-CN', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false
    });
  }

  function manualActionLabel(action: string): string {
    if (action === 'override_reassign') return '覆盖指派';
    if (action === 'override_reschedule') return '调整截止';
    if (action === 'override_link_repo') return '关联仓库';
    return '人工调停';
  }

  function manualEventMessage(event: ManualEvent): string {
    const reason = event.reason?.trim() ? `。${event.reason.trim()}` : '';
    if (event.action === 'override_reassign') {
      return `${event.actor || '人工调停'} 将负责人从 ${event.original_value || '未指派'} 调整为 ${event.override_value || '未指派'}${reason}`;
    }
    if (event.action === 'override_reschedule') {
      return `${event.actor || '人工调停'} 将截止日从 ${event.original_value || '未设置'} 调整为 ${event.override_value || '未设置'}${reason}`;
    }
    if (event.action === 'override_link_repo') {
      return `${event.actor || '人工调停'} 将关联仓库调整为 ${event.override_value || '未设置'}${reason}`;
    }
    return `${event.actor || '人工调停'} 更新了 ${event.task_id}${reason}`;
  }

  function buildTimelineGroups(automatic: AutomaticEvent[], manual: ManualEvent[]): TimelineGroup[] {
    const events: TimelineEvent[] = automatic.map((event, index) => {
      const date = parseTimelineDate(event.occurred_at || event.time);
      return {
        id: `automatic-${event.task_id}-${event.occurred_at || event.time}-${index}`,
        kind: 'automatic',
        dateKey: dateKey(date),
        dateLabel: dateLabel(date),
        timeLabel: timeLabel(date),
        dateTime: date.toISOString(),
        sortValue: date.getTime(),
        taskId: event.task_id,
        title: '自动流转',
        message: event.message,
        actor: event.assignee || '系统',
        commitId: event.commit_id,
        commitUrl: event.commit_url
      };
    });

    manual.forEach((event) => {
      const date = parseTimelineDate(event.created_at);
      events.push({
        id: `manual-${event.id}`,
        kind: 'manual',
        dateKey: dateKey(date),
        dateLabel: dateLabel(date),
        timeLabel: timeLabel(date),
        dateTime: date.toISOString(),
        sortValue: date.getTime(),
        taskId: event.task_id,
        title: manualActionLabel(event.action),
        message: manualEventMessage(event),
        actor: event.actor || '人工调停'
      });
    });

    events.sort((a, b) => b.sortValue - a.sortValue);
    const grouped = new Map<string, TimelineGroup>();
    events.forEach((event) => {
      const group = grouped.get(event.dateKey) || {
        dateKey: event.dateKey,
        dateLabel: event.dateLabel,
        events: []
      };
      group.events.push(event);
      grouped.set(event.dateKey, group);
    });
    return Array.from(grouped.values());
  }

  async function loadConfiguration() {
    const [linkResult, configResult] = await Promise.allSettled([
      fetch('/api/jira/link-config'),
      fetch('/api/config')
    ]);

    if (linkResult.status === 'fulfilled' && linkResult.value.ok) {
      const data = await linkResult.value.json();
      jiraBaseUrl = data?.base_url ? data.base_url.replace(/\/+$/, '') : '';
    }
    if (configResult.status === 'fulfilled' && configResult.value.ok) {
      const data = await configResult.value.json();
      updateCoreMembers(data);
      if (!jiraBaseUrl) jiraBaseUrl = data?.jira?.base_url ? data.jira.base_url.replace(/\/+$/, '') : '';
      gitlabBaseUrl = data?.gitlab?.base_url ? data.gitlab.base_url.replace(/\/+$/, '') : '';
    }
  }

  async function refreshEvents() {
    if (refreshing) return;
    refreshing = true;
    try {
      const [agendaResult, manualResult] = await Promise.allSettled([
        refreshAgendaSummary(),
        fetch('/api/strongest-brain/override-audit?limit=200')
      ]);

      void agendaResult;
      if (manualResult.status === 'fulfilled' && manualResult.value.ok) {
        const data = await manualResult.value.json();
        manualEvents = Array.isArray(data?.items) ? data.items : [];
      }
    } finally {
      refreshing = false;
    }
  }

  function getJiraUrl(taskId: string) {
    if (!jiraBaseUrl || !/^[A-Z][A-Z0-9_]*-\d+$/.test(taskId || '')) return '';
    return `${jiraBaseUrl}/browse/${taskId}`;
  }

  function shortCommit(commitId?: string) {
    return commitId ? commitId.slice(0, 8) : '';
  }

  function normalizedCommitUrl(rawUrl?: string) {
    const url = (rawUrl || '').trim();
    if (!url || /^about:blank$/i.test(url)) return '';
    const commitUrl = url.replace(/\/-\/commit\//g, '/commit/');
    if (/^https?:\/\//i.test(commitUrl)) return commitUrl;
    if (commitUrl.startsWith('//')) return `https:${commitUrl}`;
    if (commitUrl.startsWith('/') && gitlabBaseUrl) return `${gitlabBaseUrl}${commitUrl}`;
    if (commitUrl.includes('/commit/') && gitlabBaseUrl) return `${gitlabBaseUrl}/${commitUrl.replace(/^\/+/, '')}`;
    if (/^[\w.-]+(?::\d+)?\//.test(commitUrl)) return `https://${commitUrl}`;
    return '';
  }

  function portalToConsole(node: HTMLElement) {
    const target = document.querySelector<HTMLElement>('.functional-console') || document.body;
    target.appendChild(node);
    return {
      destroy() {
        node.remove();
      }
    };
  }

  async function openDrawer() {
    drawerOpen = true;
    await tick();
    closeButton?.focus();
  }

  async function closeDrawer() {
    if (!drawerOpen) return;
    drawerOpen = false;
    await tick();
    document.querySelector<HTMLButtonElement>('.workspace-latest-event')?.focus();
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!drawerOpen) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      closeDrawer();
      return;
    }
    if (event.key !== 'Tab' || !drawerEl) return;

    const focusable = Array.from(drawerEl.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
    )).filter((element) => !element.hasAttribute('hidden'));
    if (focusable.length === 0) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  onMount(() => {
    let disposed = false;
    const unsubscribeAgenda = subscribeAgendaSummary((agendaState) => {
      if (agendaState.data) {
        automaticEvents = Array.isArray(agendaState.data.auto_decisions)
          ? agendaState.data.auto_decisions as unknown as AutomaticEvent[]
          : [];
      }
    });
    const bootstrap = async () => {
      await loadConfiguration();
      if (!disposed) await refreshEvents();
    };
    const handleUpdate = () => refreshEvents();

    bootstrap();
    const interval = window.setInterval(refreshEvents, 15000);
    window.addEventListener('decision-events-updated', handleUpdate);

    return () => {
      disposed = true;
      unsubscribeAgenda();
      window.clearInterval(interval);
      window.removeEventListener('decision-events-updated', handleUpdate);
      dispatch('summary', null);
    };
  });
</script>

<svelte:window on:keydown={handleKeydown} />

{#if drawerOpen}
  <div use:portalToConsole class="timeline-drawer-layer">
    <button type="button" class="timeline-drawer-backdrop" aria-label="关闭事件记录背景层" on:click={closeDrawer}></button>
    <div
      class="timeline-drawer"
      role="dialog"
      aria-modal="true"
      aria-labelledby="workspace-timeline-drawer-title"
      bind:this={drawerEl}
    >
      <header class="timeline-drawer-header">
        <div>
          <span class="timeline-drawer-eyebrow">决策证据</span>
          <h2 id="workspace-timeline-drawer-title">事件记录</h2>
          <p>全部可见事件按发生时间倒序展示，最新记录在前。</p>
        </div>
        <div class="timeline-drawer-actions">
          <strong class="timeline-drawer-count">{timelineEvents.length} 条</strong>
          <OverlayCloseButton label="关闭事件记录" bind:this={closeButton} on:click={closeDrawer} />
        </div>
      </header>

      <div class="timeline-drawer-body">
        {#if timelineGroups.length === 0}
          <div class="timeline-drawer-empty">
            <strong>暂无事件记录</strong>
            <span>自动流转或人工调停发生后会在这里按时间归档。</span>
          </div>
        {:else}
          {#each timelineGroups as group}
            <section class="timeline-day" aria-labelledby={`workspace-timeline-day-${group.dateKey}`}>
              <div class="timeline-day-heading">
                <h3 id={`workspace-timeline-day-${group.dateKey}`}>{group.dateLabel}</h3>
                <span>{group.events.length} 条</span>
              </div>
              <ol>
                {#each group.events as event, index (event.id)}
                  <li class="timeline-drawer-event {event.kind}" aria-current={group === timelineGroups[0] && index === 0 ? 'true' : undefined}>
                    <div class="timeline-event-time">
                      <time datetime={event.dateTime}>{event.timeLabel}</time>
                      <span class="timeline-event-kind">{event.kind === 'automatic' ? '自动流转' : '人工调停'}</span>
                    </div>
                    <div class="timeline-event-content">
                      <div class="timeline-event-title">
                        <strong>{event.title}</strong>
                        {#if getJiraUrl(event.taskId)}
                          <a href={getJiraUrl(event.taskId)} target="_blank" rel="noopener noreferrer">{event.taskId}</a>
                        {:else}
                          <span>{event.taskId}</span>
                        {/if}
                      </div>
                      <p>{event.message}</p>
                      <div class="timeline-event-evidence">
                        <span>{event.actor}</span>
                        {#if event.commitId}
                          {#if normalizedCommitUrl(event.commitUrl)}
                            <a href={normalizedCommitUrl(event.commitUrl)} target="_blank" rel="noopener noreferrer">commit {shortCommit(event.commitId)}</a>
                          {:else}
                            <span>commit {shortCommit(event.commitId)}</span>
                          {/if}
                        {/if}
                      </div>
                    </div>
                  </li>
                {/each}
              </ol>
            </section>
          {/each}
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .timeline-drawer-layer {
    position: fixed;
    inset: 0;
    z-index: var(--wa-layer-modal, 120);
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(440px, 560px);
    pointer-events: none;
  }

  .timeline-drawer-backdrop {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    padding: 0;
    border: 0;
    border-radius: 0;
    background: rgba(13, 23, 34, 0.28);
    backdrop-filter: blur(2px);
    -webkit-backdrop-filter: blur(2px);
    cursor: default;
    pointer-events: auto;
  }

  .timeline-drawer {
    position: relative;
    z-index: 1;
    grid-column: 2;
    min-width: 0;
    height: 100dvh;
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    border-left: 1px solid var(--wa-border-soft);
    background: var(--wa-surface-flat, #fbfdff);
    box-shadow: -16px 0 40px rgba(13, 23, 34, 0.18);
    pointer-events: auto;
  }

  .timeline-drawer-header {
    min-height: 112px;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: flex-start;
    gap: var(--wa-space-4);
    padding: 20px 22px;
    border-bottom: 1px solid var(--wa-border-soft);
  }

  .timeline-drawer-header > div:first-child {
    min-width: 0;
    display: grid;
    gap: var(--wa-space-1);
  }

  .timeline-drawer-eyebrow,
  .timeline-drawer-header h2,
  .timeline-drawer-header p {
    margin: 0;
  }

  .timeline-drawer-eyebrow {
    color: var(--wa-accent-strong);
    font-size: 10px;
    font-weight: 780;
    letter-spacing: 0.08em;
  }

  .timeline-drawer-header h2 {
    color: var(--wa-text-strong);
    font-size: 20px;
    line-height: 1.25;
  }

  .timeline-drawer-header p {
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.5;
  }

  .timeline-drawer-actions {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    gap: var(--wa-space-2);
  }

  .timeline-drawer-count {
    color: var(--wa-text-muted);
    font-family: var(--wa-font-mono);
    font-size: 11px;
    font-variant-numeric: tabular-nums;
  }

  .timeline-drawer-body {
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
    scrollbar-gutter: stable;
    padding: 0 22px 28px;
  }

  .timeline-day {
    display: grid;
  }

  .timeline-day-heading {
    position: sticky;
    top: 0;
    z-index: 2;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--wa-space-3);
    padding: 14px 0 10px;
    border-bottom: 1px solid var(--wa-border-soft);
    background: var(--wa-surface-flat, #fbfdff);
    color: var(--wa-text-muted);
    font-size: 11px;
    font-weight: 760;
  }

  .timeline-day-heading h3,
  .timeline-day-heading span {
    margin: 0;
    color: inherit;
    font: inherit;
  }

  .timeline-day ol {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .timeline-drawer-event {
    position: relative;
    min-width: 0;
    display: grid;
    grid-template-columns: 84px minmax(0, 1fr);
    gap: var(--wa-space-4);
    padding: 16px 0 16px 24px;
  }

  .timeline-drawer-event::before {
    content: "";
    position: absolute;
    top: 22px;
    left: 4px;
    z-index: 1;
    width: 9px;
    height: 9px;
    border-radius: 999px;
    background: var(--wa-info);
    box-shadow: 0 0 0 4px var(--wa-info-soft);
  }

  .timeline-drawer-event.manual::before {
    background: var(--wa-accent);
    box-shadow: 0 0 0 4px var(--wa-accent-soft);
  }

  .timeline-drawer-event:not(:last-child)::after {
    content: "";
    position: absolute;
    top: 33px;
    bottom: -6px;
    left: 8px;
    width: 1px;
    background: var(--wa-border-soft);
  }

  .timeline-event-time {
    display: grid;
    align-content: start;
    gap: var(--wa-space-1);
  }

  .timeline-event-time time {
    color: var(--wa-text-strong);
    font-family: var(--wa-font-mono);
    font-size: 11px;
    font-variant-numeric: tabular-nums;
  }

  .timeline-event-kind {
    color: var(--wa-info);
    font-size: 10px;
    font-weight: 760;
  }

  .timeline-drawer-event.manual .timeline-event-kind {
    color: var(--wa-accent-strong);
  }

  .timeline-event-content {
    min-width: 0;
    display: grid;
    gap: var(--wa-space-2);
  }

  .timeline-event-title,
  .timeline-event-evidence {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--wa-space-2);
  }

  .timeline-event-title strong {
    min-width: 0;
    color: var(--wa-text-strong);
    font-size: 12px;
  }

  .timeline-event-title a,
  .timeline-event-evidence a {
    color: var(--wa-info);
    text-decoration: none;
  }

  .timeline-event-title a:hover,
  .timeline-event-evidence a:hover {
    color: var(--wa-accent-strong);
  }

  .timeline-event-content p {
    margin: 0;
    color: var(--wa-text-main);
    font-size: 12px;
    line-height: 1.55;
  }

  .timeline-event-evidence {
    color: var(--wa-text-muted);
    font-size: 10px;
  }

  .timeline-drawer-empty {
    min-height: 240px;
    display: grid;
    place-content: center;
    gap: var(--wa-space-2);
    text-align: center;
  }

  .timeline-drawer-empty strong {
    color: var(--wa-text-strong);
  }

  .timeline-drawer-empty span {
    color: var(--wa-text-muted);
    font-size: 12px;
  }

  @media (max-width: 640px) {
    .timeline-drawer-layer {
      grid-template-columns: 1fr;
    }

    .timeline-drawer {
      grid-column: 1;
      width: 100%;
    }

    .timeline-drawer-header {
      min-height: 104px;
      padding: 16px;
    }

    .timeline-drawer-body {
      padding: 0 16px 24px;
    }

    .timeline-drawer-event {
      grid-template-columns: 1fr;
      gap: var(--wa-space-2);
    }

    .timeline-event-time {
      display: flex;
      align-items: baseline;
      gap: var(--wa-space-2);
    }
  }

</style>
