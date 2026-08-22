<script lang="ts">

  import { onMount } from 'svelte';
  import OverlayCloseButton from './shared/OverlayCloseButton.svelte';

  export let taskID = '';
  export let isOpen = false;
  export let onClose: () => void = () => {};
  export let presentation: 'drawer' | 'inline' | 'modal' = 'drawer';
  export let embeddedInModal = false;

  let commits: any[] = [];
  let loading = false;
  let errorMsg = '';
  let lastFetchedTaskID = '';
  let requestVersion = 0;
  let refreshPending = false;
  let workspaceScoped = false;

  $: if (isOpen && taskID) {
    fetchCommitsIfNeeded();
  }

  $: pushCount = commits.filter(log => log.action === 'git_push').length;
  $: mrCount = commits.filter(log => log.action && log.action.startsWith('mr_')).length;
  $: commentCount = commits.filter(log => log.action === 'jira_comment').length;
  $: latestLog = commits[0] || null;
  $: evidenceStateLabel = commits.length > 0 ? '已捕获代码证据' : '等待代码证据';
  $: evidenceStateTone = commits.length > 0 ? 'ready' : 'empty';
  $: isInitialLoading = loading && commits.length === 0;
  $: if (!isOpen) {
    lastFetchedTaskID = '';
    refreshPending = false;
  }

  function fetchCommitsIfNeeded() {
    if (!taskID || lastFetchedTaskID === taskID) return;
    fetchCommits();
  }

  function portalToWorkspaceStage(node: HTMLElement) {
    if (presentation !== 'drawer') return {};
    const target = node.closest<HTMLElement>('.workspace-stage')
      || document.querySelector<HTMLElement>('.workspace-stage');
    if (!target) return {};

    workspaceScoped = true;
    target.appendChild(node);

    const syncWorkspaceBounds = () => {
      const rect = target.getBoundingClientRect();
      node.style.setProperty('--wa-drawer-workspace-top', `${Math.max(0, rect.top)}px`);
      node.style.setProperty('--wa-drawer-workspace-right', `${Math.max(0, window.innerWidth - rect.right)}px`);
      node.style.setProperty('--wa-drawer-workspace-bottom', `${Math.max(0, window.innerHeight - rect.bottom)}px`);
      node.style.setProperty('--wa-drawer-workspace-left', `${Math.max(0, rect.left)}px`);
    };
    const resizeObserver = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(syncWorkspaceBounds);
    syncWorkspaceBounds();
    resizeObserver?.observe(target);
    window.addEventListener('resize', syncWorkspaceBounds);

    return {
      destroy() {
        window.removeEventListener('resize', syncWorkspaceBounds);
        resizeObserver?.disconnect();
        workspaceScoped = false;
        node.remove();
      }
    };
  }

  async function fetchCommits(preserveExisting = false) {
    const requestedTaskID = taskID;
    const currentRequestVersion = ++requestVersion;
    loading = true;
    errorMsg = '';
    if (!preserveExisting) commits = [];
    lastFetchedTaskID = requestedTaskID;
    try {
      const token = localStorage.getItem('jwt_token');
      const res = await fetch(`/api/tasks/commits?task_id=${encodeURIComponent(requestedTaskID)}`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (res.ok) {
        const data = await res.json();
        if (currentRequestVersion === requestVersion && requestedTaskID === taskID) {
          commits = data || [];
        }
      } else {
        throw new Error('获取代码轨迹失败');
      }
    } catch (e: any) {
      if (currentRequestVersion === requestVersion && requestedTaskID === taskID && (!preserveExisting || commits.length === 0)) {
        errorMsg = e.message || '加载代码轨迹失败';
      }
    } finally {
      if (currentRequestVersion === requestVersion) {
        loading = false;
        if (refreshPending && isOpen && requestedTaskID === taskID) {
          refreshPending = false;
          lastFetchedTaskID = '';
          void fetchCommits(true);
        }
      }
    }
  }

  function refreshCommits() {
    if (loading) {
      refreshPending = true;
      return;
    }
    lastFetchedTaskID = '';
    void fetchCommits(true);
  }

  function handleTelemetryUpdated(event: Event) {
    const updatedTaskID = String((event as CustomEvent<{ task_id?: string }>).detail?.task_id || '').trim();
    if (!isOpen || !taskID || !updatedTaskID || updatedTaskID.toLowerCase() !== taskID.toLowerCase()) return;
    refreshCommits();
  }

  onMount(() => {
    window.addEventListener('well-ambient:telemetry-updated', handleTelemetryUpdated);
    return () => window.removeEventListener('well-ambient:telemetry-updated', handleTelemetryUpdated);
  });

  function formatTimeBrief(timeStr: string): string {
    if (!timeStr) return '';
    const date = new Date(timeStr);
    if (Number.isNaN(date.getTime())) return timeStr;
    return `${date.getMonth() + 1}/${date.getDate()} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && isOpen && presentation === 'drawer') {
      onClose();
    }
  }

  function getActionLabel(action: string) {
    if (action === 'git_push') return 'Push';
    if (action === 'jira_comment') return 'Comment';
    if (action === 'mr_open') return 'MR Open';
    if (action === 'mr_merge') return 'MR Merge';
    if (action === 'mr_close') return 'MR Close';
    return action || 'Event';
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if isOpen}
  <div use:portalToWorkspaceStage class="drawer-root" class:is-inline={presentation !== 'drawer'} class:is-workspace-scoped={workspaceScoped}>
    {#if presentation === 'drawer'}
      <button class="drawer-backdrop" type="button" aria-label="关闭代码轨迹面板" on:click={onClose}></button>
    {/if}
    <aside
      class="drawer-panel font-sans"
      class:is-inline={presentation !== 'drawer'}
      class:is-modal={presentation === 'modal'}
      aria-label="代码提交轨迹"
    >
      {#if embeddedInModal}
        <div class="modal-context-bar">
          <span class="drawer-subtitle font-mono" title={taskID}>{taskID || '未选择任务'}</span>
          <button class="refresh-btn font-mono" type="button" on:click={refreshCommits} disabled={loading}>刷新</button>
        </div>
      {:else}
        <div class="drawer-header">
        <div class="drawer-title-stack">
          <span class="drawer-kicker font-mono">GIT TELEMETRY TRACKER</span>
          <h3>代码提交轨迹</h3>
          <p class="drawer-subtitle font-mono">{taskID || '未选择任务'}</p>
        </div>
        <div class="drawer-actions">
          <button class="refresh-btn font-mono" type="button" on:click={refreshCommits} disabled={loading}>刷新</button>
          {#if presentation === 'drawer'}
            <OverlayCloseButton label="关闭代码轨迹面板" on:click={onClose} />
          {/if}
        </div>
        </div>
      {/if}

      <div class="telemetry-summary">
        <div class="summary-cell state-{evidenceStateTone}">
          <span class="font-mono">STATE</span>
          <strong>{evidenceStateLabel}</strong>
        </div>
        <div class="summary-cell">
          <span class="font-mono">PUSH</span>
          <strong>{pushCount}</strong>
        </div>
        <div class="summary-cell">
          <span class="font-mono">MR</span>
          <strong>{mrCount}</strong>
        </div>
        <div class="summary-cell">
          <span class="font-mono">COMMENT</span>
          <strong>{commentCount}</strong>
        </div>
      </div>

      <div class="drawer-body">
        {#if isInitialLoading}
          <div class="loading-state" aria-label="正在加载代码轨迹">
            <div class="skeleton-line wide"></div>
            <div class="skeleton-card"></div>
            <div class="skeleton-card compact"></div>
            <div class="skeleton-card"></div>
          </div>
        {:else if errorMsg}
          <div class="error-state">
            <span class="font-mono">LOAD FAILED</span>
            <strong>{errorMsg}</strong>
            <button type="button" class="retry-btn font-mono" on:click={refreshCommits}>重试</button>
          </div>
        {:else if commits && commits.length > 0}
          {#if latestLog}
            <div class="latest-signal">
              <span class="summary-kicker font-mono">LATEST SIGNAL</span>
              <strong>{getActionLabel(latestLog.action)}</strong>
              <em>{formatTimeBrief(latestLog.created_at)} / {latestLog.author || '未知提交人'}</em>
            </div>
          {/if}
          <div class="commit-timeline">
            {#each commits as log}
              <div class="timeline-item">
                <div class="timeline-badge-container">
                  <span class="timeline-badge badge-{log.action}">
                    {getActionLabel(log.action)}
                  </span>
                  <span class="timeline-time font-mono">{formatTimeBrief(log.created_at)}</span>
                </div>
                <div class="timeline-content">
                  <div class="timeline-meta">
                    {#if log.action === 'jira_comment'}
                      <span class="meta-repo" title="Jira 评论">Jira 评论</span>
                    {:else}
                      <span class="meta-repo" title={log.repo || '未知仓库'}>{log.repo || '未知仓库'}</span>
                      <span class="meta-branch" title={log.branch || '未知分支'}>{log.branch || '未知分支'}</span>
                      {#if log.commit_id}
                        <span class="meta-hash font-mono" title={log.commit_id}>{log.commit_id.substring(0, 8)}</span>
                      {/if}
                    {/if}
                  </div>
                  <div class="timeline-body font-mono" title={log.message || ''}>
                    {#if log.mr_url}
                      <a href={log.mr_url} target="_blank" rel="noopener noreferrer" class="mr-timeline-link">
                        !{log.mr_iid}: {log.message}
                      </a>
                    {:else}
                      {log.message}
                    {/if}
                  </div>
                  <div class="timeline-footer">
                    <span>{log.action === 'jira_comment' ? '评论人' : '提交人'}: {log.author || '未知'}</span>
                  </div>
                </div>
              </div>
            {/each}
          </div>
        {:else}
          <div class="empty-state">
            <span class="font-mono">NO TELEMETRY</span>
            <h4>暂无关联代码证据</h4>
            <p>该任务目前仅处于 Jira 状态，尚未匹配到 GitLab Push、MR 或评论记录。</p>
            <p class="empty-hint">分支或 Commit 需要包含任务前缀 <code>{taskID}</code>，推送后会自动关联。</p>
          </div>
        {/if}
      </div>
    </aside>
  </div>
{/if}

<style>
  .drawer-root {
    position: fixed;
    inset: 0;
    z-index: 2000;
    display: flex;
    justify-content: flex-end;
    pointer-events: auto;
    animation: fadeIn 0.18s ease-out;
  }

  .drawer-root.is-workspace-scoped {
    position: fixed;
    inset:
      var(--wa-drawer-workspace-top, 0px)
      var(--wa-drawer-workspace-right, 0px)
      var(--wa-drawer-workspace-bottom, 0px)
      var(--wa-drawer-workspace-left, 0px);
  }

  .drawer-backdrop {
    position: absolute;
    inset: 0;
    border: 0;
    background:
      linear-gradient(120deg, rgba(15, 23, 42, 0.58), rgba(15, 23, 42, 0.34)),
      rgba(2, 6, 23, 0.18);
    backdrop-filter: blur(14px) saturate(128%);
    cursor: pointer;
  }

  .drawer-backdrop:focus-visible {
    outline: 2px solid rgba(32, 197, 183, 0.72);
    outline-offset: -4px;
  }

  .drawer-panel {
    position: relative;
    z-index: 1;
    width: min(584px, calc(100vw - 24px));
    height: 100%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    color: var(--wa-ink-strong, #18212f);
    background:
      linear-gradient(150deg, rgba(255, 255, 255, 0.86), rgba(236, 246, 244, 0.74)),
      linear-gradient(180deg, rgba(38, 166, 154, 0.12), rgba(53, 76, 121, 0.08));
    border-left: 1px solid rgba(255, 255, 255, 0.58);
    box-shadow:
      -28px 0 70px rgba(15, 23, 42, 0.25),
      inset 1px 0 0 rgba(255, 255, 255, 0.72);
    backdrop-filter: blur(26px) saturate(145%);
    animation: slideIn 0.28s cubic-bezier(0.2, 0.84, 0.28, 1);
  }

  .drawer-panel::before {
    content: '';
    position: absolute;
    inset: 0;
    pointer-events: none;
    background:
      linear-gradient(90deg, rgba(32, 197, 183, 0.16), transparent 16%),
      linear-gradient(180deg, rgba(255, 255, 255, 0.42), transparent 34%);
  }

  .drawer-header {
    position: relative;
    z-index: 1;
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 20px;
    padding: 28px 28px 18px;
    border-bottom: 1px solid rgba(103, 119, 137, 0.18);
  }

  .modal-context-bar {
    min-width: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 16px 24px 12px;
  }

  .modal-context-bar .drawer-subtitle {
    min-width: 0;
    max-width: none;
  }

  .drawer-title-stack {
    min-width: 0;
    display: grid;
    gap: 7px;
  }

  .drawer-kicker,
  .summary-kicker,
  .summary-cell span,
  .error-state span,
  .empty-state > span {
    color: var(--wa-ink-muted, #667489);
    font-size: 0.64rem;
    font-weight: 800;
    letter-spacing: 0;
    text-transform: uppercase;
  }

  .drawer-header h3 {
    margin: 0;
    color: var(--wa-ink-strong, #18212f);
    font-size: clamp(1.35rem, 2vw, 1.72rem);
    font-weight: 760;
    line-height: 1.08;
    letter-spacing: 0;
  }

  .drawer-subtitle {
    margin: 0;
    max-width: 360px;
    overflow: hidden;
    color: rgba(24, 33, 47, 0.62);
    font-size: 0.78rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .drawer-actions {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    flex: 0 0 auto;
  }

  .refresh-btn,
  .retry-btn {
    min-height: 36px;
    border: 1px solid rgba(103, 119, 137, 0.24);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.58);
    color: var(--wa-ink-strong, #18212f);
    cursor: pointer;
    transition: transform 160ms ease, border-color 160ms ease, background 160ms ease, box-shadow 160ms ease;
  }

  .refresh-btn {
    padding: 0 14px;
    color: rgba(24, 33, 47, 0.74);
    font-size: 0.7rem;
    font-weight: 800;
  }

  .refresh-btn:hover,
  .retry-btn:hover {
    transform: translateY(-1px);
    border-color: rgba(32, 197, 183, 0.46);
    background: rgba(255, 255, 255, 0.82);
    box-shadow: 0 12px 26px rgba(15, 23, 42, 0.09);
  }

  .refresh-btn:disabled {
    cursor: wait;
    opacity: 0.56;
    transform: none;
    box-shadow: none;
  }

  .refresh-btn:focus-visible,
  .retry-btn:focus-visible,
  .mr-timeline-link:focus-visible {
    outline: 2px solid rgba(32, 197, 183, 0.58);
    outline-offset: 2px;
  }

  .telemetry-summary {
    position: relative;
    z-index: 1;
    display: grid;
    grid-template-columns: minmax(0, 1.45fr) repeat(3, minmax(76px, 1fr));
    gap: 1px;
    margin: 0 28px 20px;
    overflow: hidden;
    border: 1px solid rgba(103, 119, 137, 0.16);
    border-radius: 8px;
    background: rgba(103, 119, 137, 0.16);
  }

  .summary-cell {
    min-width: 0;
    min-height: 78px;
    padding: 14px;
    display: grid;
    align-content: space-between;
    background: rgba(255, 255, 255, 0.48);
  }

  .summary-cell strong {
    min-width: 0;
    color: var(--wa-ink-strong, #18212f);
    font-size: 1.26rem;
    font-weight: 760;
    line-height: 1.05;
    letter-spacing: 0;
    overflow-wrap: anywhere;
  }

  .summary-cell.state-ready {
    background:
      linear-gradient(135deg, rgba(32, 197, 183, 0.18), rgba(255, 255, 255, 0.56)),
      rgba(255, 255, 255, 0.42);
  }

  .summary-cell.state-empty {
    background:
      linear-gradient(135deg, rgba(214, 172, 72, 0.16), rgba(255, 255, 255, 0.58)),
      rgba(255, 255, 255, 0.42);
  }

  .drawer-body {
    position: relative;
    z-index: 1;
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 0 28px 28px;
    scrollbar-width: thin;
    scrollbar-color: rgba(103, 119, 137, 0.36) transparent;
  }

  .drawer-body::-webkit-scrollbar {
    width: 8px;
  }

  .drawer-body::-webkit-scrollbar-thumb {
    border-radius: 999px;
    background: rgba(103, 119, 137, 0.32);
  }

  .loading-state {
    display: grid;
    gap: 12px;
  }

  .skeleton-line,
  .skeleton-card {
    position: relative;
    overflow: hidden;
    border: 1px solid rgba(103, 119, 137, 0.12);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.52);
  }

  .skeleton-line::after,
  .skeleton-card::after {
    content: '';
    position: absolute;
    inset: 0;
    transform: translateX(-100%);
    background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.68), transparent);
    animation: pulse 1.25s ease-in-out infinite;
  }

  .skeleton-line {
    width: 68%;
    height: 18px;
  }

  .skeleton-line.wide {
    width: 84%;
  }

  .skeleton-card {
    height: 110px;
  }

  .skeleton-card.compact {
    height: 82px;
  }

  .error-state,
  .empty-state {
    display: grid;
    justify-items: start;
    gap: 12px;
    padding: 26px;
    border: 1px solid rgba(103, 119, 137, 0.18);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.5);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7);
  }

  .error-state strong,
  .empty-state h4 {
    margin: 0;
    color: var(--wa-ink-strong, #18212f);
    font-size: 1.08rem;
    font-weight: 760;
    letter-spacing: 0;
  }

  .error-state {
    background:
      linear-gradient(135deg, rgba(244, 99, 88, 0.12), rgba(255, 255, 255, 0.62)),
      rgba(255, 255, 255, 0.5);
  }

  .retry-btn {
    min-height: 38px;
    padding: 0 16px;
    font-size: 0.7rem;
    font-weight: 800;
  }

  .empty-state p {
    margin: 0;
    color: rgba(24, 33, 47, 0.66);
    font-size: 0.88rem;
    line-height: 1.65;
  }

  .empty-hint {
    padding-top: 8px;
    border-top: 1px solid rgba(103, 119, 137, 0.16);
  }

  .empty-hint code {
    padding: 2px 6px;
    border: 1px solid rgba(103, 119, 137, 0.18);
    border-radius: 6px;
    background: rgba(255, 255, 255, 0.72);
    color: var(--wa-ink-strong, #18212f);
  }

  .latest-signal {
    display: grid;
    gap: 7px;
    margin-bottom: 18px;
    padding: 18px;
    border: 1px solid rgba(32, 197, 183, 0.22);
    border-radius: 8px;
    background:
      linear-gradient(135deg, rgba(32, 197, 183, 0.16), rgba(255, 255, 255, 0.6)),
      rgba(255, 255, 255, 0.48);
  }

  .latest-signal strong {
    color: var(--wa-ink-strong, #18212f);
    font-size: 1.06rem;
    font-weight: 760;
    letter-spacing: 0;
  }

  .latest-signal em {
    color: rgba(24, 33, 47, 0.58);
    font-size: 0.78rem;
    font-style: normal;
  }

  .commit-timeline {
    position: relative;
    display: grid;
    gap: 14px;
    padding-left: 18px;
  }

  .commit-timeline::before {
    content: '';
    position: absolute;
    top: 10px;
    bottom: 10px;
    left: 4px;
    width: 1px;
    background: linear-gradient(180deg, rgba(32, 197, 183, 0.5), rgba(103, 119, 137, 0.14));
  }

  .timeline-item {
    position: relative;
    display: grid;
    gap: 8px;
  }

  .timeline-badge-container {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    min-height: 26px;
  }

  .timeline-badge-container::before {
    content: '';
    position: absolute;
    left: -18px;
    top: 9px;
    width: 9px;
    height: 9px;
    border-radius: 999px;
    background: rgba(32, 197, 183, 0.88);
    box-shadow: 0 0 0 4px rgba(32, 197, 183, 0.14);
  }

  .timeline-badge {
    min-height: 24px;
    padding: 4px 8px;
    display: inline-flex;
    align-items: center;
    border: 1px solid rgba(103, 119, 137, 0.2);
    border-radius: 6px;
    background: rgba(255, 255, 255, 0.6);
    color: rgba(24, 33, 47, 0.7);
    font-size: 0.64rem;
    font-weight: 850;
    letter-spacing: 0;
    text-transform: uppercase;
  }

  .timeline-badge.badge-git_push {
    border-color: rgba(32, 197, 183, 0.3);
    background: rgba(32, 197, 183, 0.14);
    color: #116b63;
  }

  .timeline-badge.badge-mr_open,
  .timeline-badge.badge-mr_merge,
  .timeline-badge.badge-mr_close {
    border-color: rgba(64, 86, 154, 0.24);
    background: rgba(64, 86, 154, 0.12);
    color: #33467f;
  }

  .timeline-badge.badge-jira_comment {
    border-color: rgba(214, 172, 72, 0.34);
    background: rgba(214, 172, 72, 0.14);
    color: #795d12;
  }

  .timeline-time {
    flex: 0 0 auto;
    color: rgba(24, 33, 47, 0.48);
    font-size: 0.72rem;
  }

  .timeline-content {
    min-width: 0;
    padding: 15px;
    border: 1px solid rgba(103, 119, 137, 0.16);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.56);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
  }

  .timeline-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-bottom: 10px;
  }

  .meta-repo,
  .meta-branch,
  .meta-hash {
    min-height: 24px;
    max-width: 100%;
    padding: 3px 8px;
    display: inline-flex;
    align-items: center;
    border: 1px solid rgba(103, 119, 137, 0.15);
    border-radius: 6px;
    background: rgba(255, 255, 255, 0.48);
    color: rgba(24, 33, 47, 0.62);
    font-size: 0.72rem;
    line-height: 1.2;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .meta-hash {
    border-color: rgba(32, 197, 183, 0.24);
    color: #116b63;
  }

  .timeline-body {
    min-width: 0;
    padding: 10px 12px;
    border: 1px solid rgba(103, 119, 137, 0.12);
    border-radius: 8px;
    background: rgba(245, 249, 248, 0.74);
    color: rgba(24, 33, 47, 0.82);
    font-size: 0.78rem;
    line-height: 1.62;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mr-timeline-link {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: #33467f;
    text-decoration: none;
    border-bottom: 1px solid rgba(64, 86, 154, 0.3);
  }

  .mr-timeline-link:hover {
    color: #20305f;
    border-bottom-color: rgba(64, 86, 154, 0.72);
  }

  .timeline-footer {
    margin-top: 9px;
    color: rgba(24, 33, 47, 0.48);
    font-size: 0.72rem;
  }

  @media (max-width: 640px) {
    .drawer-panel {
      width: calc(100vw - 12px);
    }

    .drawer-header {
      padding: 22px 18px 16px;
    }

    .telemetry-summary {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      margin: 0 18px 18px;
    }

    .drawer-body {
      padding: 0 18px 22px;
    }

    .drawer-actions {
      flex-direction: column-reverse;
      align-items: flex-end;
    }

    .modal-context-bar {
      padding-inline: 18px;
    }
  }

  .drawer-root.is-inline {
    position: relative;
    inset: auto;
    z-index: auto;
    display: block;
    min-width: 0;
    animation: none;
  }

  .drawer-panel.is-inline {
    width: 100%;
    height: auto;
    min-width: 0;
    overflow: visible;
    border: 0;
    background: transparent;
    box-shadow: none;
    backdrop-filter: none;
    animation: none;
  }

  .drawer-panel.is-inline::before {
    display: none;
  }

  .drawer-panel.is-inline .drawer-header {
    align-items: center;
    gap: 12px;
    padding: 0 0 12px;
    border-bottom-color: rgba(103, 119, 137, 0.16);
  }

  .drawer-panel.is-inline .drawer-title-stack {
    gap: 3px;
  }

  .drawer-panel.is-inline .drawer-kicker {
    display: none;
  }

  .drawer-panel.is-inline .drawer-header h3 {
    font-size: 0.94rem;
    line-height: 1.25;
  }

  .drawer-panel.is-inline .drawer-subtitle {
    font-size: 0.7rem;
  }

  .drawer-panel.is-inline .refresh-btn {
    min-height: 32px;
    padding-inline: 11px;
    background: transparent;
    box-shadow: none;
  }

  .drawer-panel.is-inline .telemetry-summary {
    grid-template-columns: minmax(0, 1.45fr) repeat(3, minmax(52px, 0.7fr));
    margin: 12px 0;
    border-radius: 8px;
  }

  .drawer-panel.is-inline .summary-cell {
    min-height: 56px;
    padding: 9px 10px;
  }

  .drawer-panel.is-inline .summary-cell strong {
    font-size: 0.92rem;
  }

  .drawer-panel.is-inline .drawer-body {
    overflow: visible;
    padding: 0;
  }

  .drawer-panel.is-inline .latest-signal {
    gap: 4px;
    margin: 0 0 10px;
    padding: 10px 0;
    border: 0;
    border-bottom: 1px solid rgba(103, 119, 137, 0.16);
    border-radius: 0;
    background: transparent;
  }

  .drawer-panel.is-inline .commit-timeline {
    gap: 10px;
    padding-left: 15px;
  }

  .drawer-panel.is-inline .timeline-badge-container::before {
    left: -15px;
  }

  .drawer-panel.is-inline .timeline-content {
    padding: 10px 0;
    border: 0;
    border-bottom: 1px solid rgba(103, 119, 137, 0.14);
    border-radius: 0;
    background: transparent;
    box-shadow: none;
  }

  .drawer-panel.is-inline .timeline-body {
    padding: 8px 0;
    border: 0;
    border-radius: 0;
    background: transparent;
  }

  .drawer-panel.is-inline .error-state,
  .drawer-panel.is-inline .empty-state {
    padding: 18px 0;
    border: 0;
    border-radius: 0;
    background: transparent;
    box-shadow: none;
  }

  /* The schedule inspector owns the viewport; inline telemetry stays dense and complete. */
  .drawer-panel.is-inline .drawer-header {
    padding-bottom: 8px;
  }

  .drawer-panel.is-inline .refresh-btn {
    min-height: 28px;
  }

  .drawer-panel.is-inline .telemetry-summary {
    margin: 8px 0;
  }

  .drawer-panel.is-inline .summary-cell {
    min-height: 42px;
    padding: 6px 8px;
  }

  .drawer-panel.is-inline .summary-cell strong {
    font-size: 0.82rem;
  }

  .drawer-panel.is-inline .latest-signal {
    display: none;
  }

  .drawer-panel.is-inline .commit-timeline {
    gap: 5px;
    padding-left: 13px;
  }

  .drawer-panel.is-inline .commit-timeline::before {
    left: 3px;
  }

  .drawer-panel.is-inline .timeline-item {
    grid-template-columns: 92px minmax(0, 1fr);
    align-items: start;
    gap: 8px;
  }

  .drawer-panel.is-inline .timeline-badge-container {
    min-height: 24px;
    align-items: flex-start;
    flex-direction: column;
    justify-content: flex-start;
    gap: 2px;
    padding-top: 5px;
  }

  .drawer-panel.is-inline .timeline-badge-container::before {
    left: -13px;
    top: 10px;
    width: 7px;
    height: 7px;
    box-shadow: 0 0 0 3px rgba(32, 197, 183, 0.12);
  }

  .drawer-panel.is-inline .timeline-badge {
    min-height: 20px;
    padding: 2px 6px;
  }

  .drawer-panel.is-inline .timeline-time {
    font-size: 0.64rem;
  }

  .drawer-panel.is-inline .timeline-content {
    padding: 5px 0;
  }

  .drawer-panel.is-inline .timeline-meta {
    flex-wrap: wrap;
    gap: 4px;
    margin-bottom: 2px;
  }

  .drawer-panel.is-inline .meta-repo,
  .drawer-panel.is-inline .meta-branch,
  .drawer-panel.is-inline .meta-hash {
    min-width: 0;
    min-height: 20px;
    max-width: 100%;
    overflow: visible;
    padding: 2px 6px;
    font-size: 0.64rem;
    text-overflow: clip;
    white-space: normal;
    overflow-wrap: anywhere;
  }

  .drawer-panel.is-inline .timeline-body {
    display: block;
    overflow: visible;
    padding: 3px 0;
    font-size: 0.7rem;
    line-height: 1.4;
    line-clamp: unset;
    -webkit-line-clamp: unset;
    text-overflow: clip;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .drawer-panel.is-inline .mr-timeline-link {
    overflow: visible;
    text-overflow: clip;
    white-space: normal;
    overflow-wrap: anywhere;
  }

  .drawer-panel.is-inline .timeline-footer {
    margin-top: 2px;
    font-size: 0.64rem;
  }

  .drawer-panel.is-modal .telemetry-summary {
    margin: 0 24px 16px;
  }

  .drawer-panel.is-modal .drawer-body {
    padding: 0 24px 24px;
  }

  .drawer-panel.is-modal .timeline-body {
    display: block;
    overflow: hidden;
    line-clamp: unset;
    -webkit-line-clamp: unset;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  @media (max-width: 640px) {
    .drawer-panel.is-modal .telemetry-summary {
      margin-inline: 18px;
    }

    .drawer-panel.is-modal .drawer-body {
      padding-inline: 18px;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .drawer-root,
    .drawer-panel,
    .skeleton-line::after,
    .skeleton-card::after {
      animation: none;
    }
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @keyframes slideIn {
    from { transform: translateX(32px); opacity: 0.82; }
    to { transform: translateX(0); opacity: 1; }
  }

  @keyframes pulse {
    to { transform: translateX(100%); }
  }
</style>
