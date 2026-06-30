<script lang="ts">
  import { onMount } from 'svelte';

  export let taskID = '';
  export let isOpen = false;
  export let onClose: () => void = () => {};

  let commits: any[] = [];
  let loading = false;
  let errorMsg = '';

  $: if (isOpen && taskID) {
    fetchCommits();
  }

  async function fetchCommits() {
    loading = true;
    errorMsg = '';
    commits = [];
    try {
      const token = localStorage.getItem('jwt_token');
      const res = await fetch(`/api/tasks/commits?task_id=${taskID}`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (res.ok) {
        const data = await res.json();
        commits = data || [];
      } else {
        throw new Error('获取代码轨迹失败');
      }
    } catch (e: any) {
      errorMsg = e.message || '加载代码轨迹失败';
    } finally {
      loading = false;
    }
  }

  function formatTimeBrief(timeStr: string): string {
    if (!timeStr) return '';
    const date = new Date(timeStr);
    if (Number.isNaN(date.getTime())) return timeStr;
    return `${date.getMonth() + 1}/${date.getDate()} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && isOpen) {
      onClose();
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if isOpen}
  <div class="drawer-backdrop" on:click={onClose}>
    <div class="drawer-panel font-sans" on:click|stopPropagation>
      <div class="drawer-header">
        <div>
          <span class="drawer-kicker font-mono">GIT TELEMETRY TRACKER</span>
          <h3>🛰️ 代码提交轨迹 ({taskID})</h3>
        </div>
        <button class="close-btn" on:click={onClose} aria-label="关闭代码轨迹面板">&times;</button>
      </div>

      <div class="drawer-body">
        {#if loading}
          <div class="loading-state font-mono">
            <span class="spinner"></span> 正在拉取最新的 Git 遥测明细...
          </div>
        {:else if errorMsg}
          <div class="error-state font-mono">❌ {errorMsg}</div>
        {:else if commits && commits.length > 0}
          <div class="commit-timeline">
            {#each commits as log}
              <div class="timeline-item">
                <div class="timeline-badge-container">
                  <span class="timeline-badge badge-{log.action}">
                    {log.action === 'git_push' ? 'Push' : (log.action === 'jira_comment' ? 'Comment' : 'MR')}
                  </span>
                  <span class="timeline-time font-mono">{formatTimeBrief(log.created_at)}</span>
                </div>
                <div class="timeline-content">
                  <div class="timeline-meta">
                    {#if log.action === 'jira_comment'}
                      <span class="meta-repo">💬 Jira 评论</span>
                    {:else}
                      <span class="meta-repo">📁 {log.repo}</span>
                      <span class="meta-branch">🌿 {log.branch}</span>
                      {#if log.commit_id}
                        <span class="meta-hash font-mono" title="Commit Hash">{log.commit_id.substring(0, 8)}</span>
                      {/if}
                    {/if}
                  </div>
                  <div class="timeline-body font-mono">
                    {#if log.mr_url}
                      <a href={log.mr_url} target="_blank" rel="noopener noreferrer" class="mr-timeline-link">
                        !{log.mr_iid}: {log.message}
                      </a>
                    {:else}
                      {log.message}
                    {/if}
                  </div>
                  <div class="timeline-footer">
                    <span>👤 {log.action === 'jira_comment' ? '评论人' : '提交人'}: {log.author}</span>
                  </div>
                </div>
              </div>
            {/each}
          </div>
        {:else}
          <div class="empty-state font-mono">
            <p>ℹ️ 该任务目前仅处于 Jira 状态，暂无关联的代码提交 (Git Telemetry) 记录。</p>
            <p class="empty-hint">请确保您的分支或 Commit 包含任务前缀 (如 <code>{taskID}</code>) 并推送到 GitLab 仓库中以进行自动关联。</p>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .drawer-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(15, 23, 42, 0.7);
    backdrop-filter: blur(4px);
    z-index: 2000;
    display: flex;
    justify-content: flex-end;
    animation: fadeIn 0.2s ease-out;
  }

  .drawer-panel {
    width: 500px;
    max-width: 90%;
    height: 100%;
    background: #0b1329;
    border-left: 1px solid rgba(56, 189, 248, 0.2);
    box-shadow: -10px 0 30px rgba(0, 0, 0, 0.5);
    display: flex;
    flex-direction: column;
    animation: slideIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .drawer-header {
    padding: 24px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.5);
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .drawer-kicker {
    font-size: 0.65rem;
    font-weight: 800;
    color: #38bdf8;
    letter-spacing: 0.1em;
    display: block;
    margin-bottom: 4px;
  }

  .drawer-header h3 {
    margin: 0;
    font-size: 1.25rem;
    font-weight: 700;
    color: #f1f5f9;
  }

  .close-btn {
    background: none;
    border: none;
    color: #64748b;
    font-size: 1.75rem;
    cursor: pointer;
    line-height: 1;
    padding: 0;
    transition: color 0.2s;
  }

  .close-btn:hover {
    color: #f1f5f9;
  }

  .drawer-body {
    flex: 1;
    overflow-y: auto;
    padding: 24px;
  }

  /* Spinner */
  .spinner {
    display: inline-block;
    width: 14px;
    height: 14px;
    border: 2px solid rgba(56, 189, 248, 0.2);
    border-top-color: #38bdf8;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    margin-right: 8px;
    vertical-align: middle;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* Loading & States */
  .loading-state, .error-state {
    padding: 20px;
    text-align: center;
    color: #94a3b8;
    font-size: 0.85rem;
  }

  .error-state {
    color: #f87171;
  }

  .empty-state {
    padding: 32px 20px;
    text-align: center;
    border: 1px dashed rgba(51, 65, 85, 0.4);
    border-radius: 8px;
    color: #64748b;
    background: rgba(15, 23, 42, 0.2);
    font-size: 0.85rem;
    line-height: 1.6;
  }

  .empty-hint {
    margin-top: 16px;
    font-size: 0.75rem;
    color: #475569;
  }

  /* Commit Timeline */
  .commit-timeline {
    display: flex;
    flex-direction: column;
    gap: 16px;
    position: relative;
    padding-left: 12px;
  }

  .commit-timeline::before {
    content: '';
    position: absolute;
    top: 6px;
    bottom: 6px;
    left: 4px;
    width: 2px;
    background: rgba(51, 65, 85, 0.5);
  }

  .timeline-item {
    display: flex;
    flex-direction: column;
    gap: 8px;
    position: relative;
  }

  .timeline-badge-container {
    display: flex;
    align-items: center;
    gap: 10px;
    padding-left: 14px;
  }

  .timeline-badge-container::before {
    content: '';
    position: absolute;
    left: 1px;
    top: 6px;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #38bdf8;
    box-shadow: 0 0 6px #38bdf8;
  }

  .timeline-badge {
    font-size: 0.65rem;
    font-weight: 700;
    padding: 2px 6px;
    border-radius: 4px;
    text-transform: uppercase;
  }

  .timeline-badge.badge-git_push {
    background: rgba(14, 165, 233, 0.15);
    color: #38bdf8;
    border: 1px solid rgba(14, 165, 233, 0.3);
  }

  .timeline-badge.badge-mr_open,
  .timeline-badge.badge-mr_merge,
  .timeline-badge.badge-mr_close {
    background: rgba(168, 85, 247, 0.15);
    color: #c084fc;
    border: 1px solid rgba(168, 85, 247, 0.3);
  }

  .timeline-badge.badge-jira_comment {
    background: rgba(245, 158, 11, 0.15);
    color: #fbbf24;
    border: 1px solid rgba(245, 158, 11, 0.3);
  }

  .timeline-time {
    font-size: 0.75rem;
    color: #64748b;
  }

  .timeline-content {
    background: rgba(15, 23, 42, 0.4);
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 6px;
    padding: 12px;
    margin-left: 14px;
  }

  .timeline-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    font-size: 0.75rem;
    color: #94a3b8;
    margin-bottom: 6px;
  }

  .meta-repo, .meta-branch, .meta-hash {
    display: inline-flex;
    align-items: center;
  }

  .meta-hash {
    background: rgba(30, 41, 59, 0.8);
    border: 1px solid rgba(51, 65, 85, 0.6);
    padding: 1px 4px;
    border-radius: 3px;
    color: #38bdf8;
  }

  .timeline-body {
    font-size: 0.8rem;
    color: #e2e8f0;
    word-break: break-all;
    line-height: 1.5;
    background: rgba(15, 23, 42, 0.2);
    border-radius: 4px;
    padding: 6px 8px;
    margin-bottom: 6px;
  }

  .mr-timeline-link {
    color: #a855f7;
    text-decoration: none;
    border-bottom: 1px dashed rgba(168, 85, 247, 0.4);
  }

  .mr-timeline-link:hover {
    color: #c084fc;
    border-bottom-style: solid;
  }

  .timeline-footer {
    font-size: 0.7rem;
    color: #64748b;
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @keyframes slideIn {
    from { transform: translateX(100%); }
    to { transform: translateX(0); }
  }
</style>
