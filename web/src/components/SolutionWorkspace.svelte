<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { reconcileSolutionEditor, type SolutionEditorSyncState } from '../lib/solution-editor-sync';
  import MarkdownWorkbench from './shared/MarkdownWorkbench.svelte';
  import Modal from './shared/Modal.svelte';

  export let demand: any;
  export let currentUserPermissions: string[] = [];

  type Revision = {
    id: number; version: number; status: string; kind: string; markdown: string;
    content_hash: string; content_bytes: number; stored_bytes: number; content_encoding: string;
    authored_by: string; created_at: string;
  };
  type PolishJob = {
    id: number; status: string; last_error: string; attempt_count: number;
    next_attempt_at?: string | null; started_at?: string | null; completed_at?: string | null;
    created_at: string; updated_at: string;
  };
  type PolishState = { label: string; detail: string; active: boolean; danger: boolean; technicalDetail?: string };
  type Workspace = {
    asset: { id: number; demand_id: string; revision: number };
    working?: Revision; published?: Revision; candidates: Revision[]; history: Revision[];
    sources: Array<{ id: number; external_id: string; author: string; marker: string; eligible: boolean; current: boolean }>;
    jobs: PolishJob[];
  };

  let workspace: Workspace | null = null;
  let loadedDemandID = '';
  let loading = false;
  let action = '';
  let error = '';
  let notice = '';
  let conflict = false;
  let editorOpen = false;
  let discardPrompt = false;
  let publishFlow = false;
  let editorState: SolutionEditorSyncState = { markdown: '', baselineHash: '', dirty: false, remoteUpdateAvailable: false };
  let pollTimer: ReturnType<typeof setInterval> | null = null;

  $: canRead = currentUserPermissions.includes('solution:read');
  $: canWrite = currentUserPermissions.includes('solution:write');
  $: canPublish = currentUserPermissions.includes('solution:publish');
  $: latestJob = workspace?.jobs[0] || null;
  $: polishState = polishStateFor(latestJob);
  $: activeJob = polishState?.active ? latestJob : null;
  $: retryableFailedJob = canWrite && !workspace?.working && latestJob?.status === 'failed' ? latestJob : null;
  $: hasSolutionContent = !!workspace?.working || !!latestJob;
  $: if (canRead && demand?.task_id && demand.task_id !== loadedDemandID) {
    loadedDemandID = demand.task_id;
    workspace = null;
    error = '';
    notice = '';
    conflict = false;
    editorOpen = false;
    discardPrompt = false;
    publishFlow = false;
    editorState = { markdown: '', baselineHash: '', dirty: false, remoteUpdateAvailable: false };
    void loadWorkspace();
  }

  onMount(() => {
    pollTimer = setInterval(() => {
      if (activeJob || workspace) void loadWorkspace(true);
    }, 4000);
    const onTelemetry = (event: Event) => {
      const taskID = (event as CustomEvent<{ task_id?: string }>).detail?.task_id;
      if (taskID === demand?.task_id) void loadWorkspace(true);
    };
    window.addEventListener('well-ambient:telemetry-updated', onTelemetry);
    return () => window.removeEventListener('well-ambient:telemetry-updated', onTelemetry);
  });

  onDestroy(() => {
    if (pollTimer) clearInterval(pollTimer);
  });

  async function api(path: string, init: RequestInit = {}) {
    const response = await fetch(path, init);
    if (!response.ok) {
      const message = (await response.text()) || `HTTP ${response.status}`;
      const requestError = new Error(message.trim()) as Error & { status?: number };
      requestError.status = response.status;
      throw requestError;
    }
    return response.json();
  }

  function acceptWorkspace(next: Workspace) {
    workspace = next;
    editorState = reconcileSolutionEditor(editorState, next.working || null);
  }

  async function loadWorkspace(silent = false) {
    if (!canRead || !demand?.task_id) return;
    if (!silent) loading = true;
    try {
      const response = await api(`/api/solutions/workspace?demand_id=${encodeURIComponent(demand.task_id)}`);
      if (!response.exists) {
        workspace = null;
        return;
      }
      acceptWorkspace(response.workspace);
    } catch (requestError: any) {
      if (!silent) error = requestError.message || '加载方案失败';
    } finally {
      if (!silent) loading = false;
    }
  }

  async function retryFailedSolution() {
    const failedJob = retryableFailedJob;
    if (!failedJob || action) return;
    action = 'retry'; error = ''; notice = '';
    try {
      const response = await api(`/api/solutions/jobs/${failedJob.id}/retry`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ demand_id: demand.task_id })
      });
      acceptWorkspace(response.workspace);
      notice = response.replayed ? '重新生成任务已在队列中。' : '已重新加入生成队列。';
    } catch (requestError: any) {
      if (requestError.status === 409) {
        await loadWorkspace(true);
        error = '方案状态已变化，请根据最新状态继续操作。';
      } else {
        error = '重新生成请求提交失败，请稍后重试。';
      }
    } finally { action = ''; }
  }

  function handleMarkdownChange(event: CustomEvent<string>) {
    const markdown = event.detail;
    editorState = { ...editorState, markdown, dirty: markdown !== (workspace?.working?.markdown || '') };
  }

  async function saveDraft() {
    if (!workspace?.working || !editorState.dirty) return true;
    if (workspace.working.status !== 'draft') {
      error = '当前方案尚未准备为可编辑草稿。';
      return false;
    }
    action = 'save'; error = ''; notice = ''; conflict = false;
    try {
      const response = await api('/api/solutions/draft', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          demand_id: demand.task_id, expected_revision: workspace.asset.revision,
          base_revision_id: workspace.working.id, title: demand.title, markdown: editorState.markdown
        })
      });
      editorState = { ...editorState, dirty: false, remoteUpdateAvailable: false };
      acceptWorkspace(response.workspace);
      notice = '方案已保存。';
      return true;
    } catch (requestError: any) {
      conflict = requestError.status === 409;
      error = conflict ? '方案已被其他人更新。本地内容仍保留，请复制后再加载远端内容。' : (requestError.message || '保存方案失败');
      return false;
    } finally { action = ''; }
  }

  async function forkPublished() {
    if (!workspace?.working) return false;
    action = 'fork'; error = ''; notice = '';
    try {
      const response = await api('/api/solutions/fork', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ demand_id: demand.task_id, expected_revision: workspace.asset.revision })
      });
      acceptWorkspace(response.workspace);
      notice = '已准备可编辑草稿。';
      return true;
    } catch (requestError: any) {
      error = requestError.message || '建立编辑草稿失败';
      return false;
    }
    finally { action = ''; }
  }

  async function publishSolution() {
    if (!workspace?.working || editorState.dirty || workspace.working.status !== 'draft') return false;
    action = 'publish'; error = ''; notice = '';
    try {
      const response = await api('/api/solutions/publish', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ demand_id: demand.task_id, expected_revision: workspace.asset.revision })
      });
      acceptWorkspace(response.workspace);
      notice = `方案已发布，并已进入 Jira 链接回写队列。`;
      return true;
    } catch (requestError: any) {
      error = requestError.message || '发布方案失败';
      return false;
    }
    finally { action = ''; }
  }

  async function openEditor() {
    if (!canWrite || !workspace?.working || action) return;
    error = '';
    notice = '';
    conflict = false;
    discardPrompt = false;
    editorOpen = true;
    if (workspace.working.status === 'published') await forkPublished();
  }

  async function saveAndPublish() {
    if (!workspace?.working || workspace.working.status !== 'draft' || !canPublish) return;
    publishFlow = true;
    try {
      if (editorState.dirty && !(await saveDraft())) return;
      if (await publishSolution()) {
        editorOpen = false;
        discardPrompt = false;
      }
    } finally {
      publishFlow = false;
    }
  }

  function requestCloseEditor() {
    if (action) {
      notice = '操作正在处理中，请稍候。';
      return;
    }
    if (editorState.dirty) {
      discardPrompt = true;
      return;
    }
    editorOpen = false;
    discardPrompt = false;
    error = '';
  }

  function discardEditorChanges() {
    editorState = {
      markdown: workspace?.working?.markdown || '',
      baselineHash: workspace?.working?.content_hash || '',
      dirty: false,
      remoteUpdateAvailable: false
    };
    conflict = false;
    error = '';
    discardPrompt = false;
    editorOpen = false;
  }

  async function reloadRemote() {
    editorState = { ...editorState, dirty: false, remoteUpdateAvailable: false };
    conflict = false;
    discardPrompt = false;
    await loadWorkspace();
  }

  async function copyLocal() {
    await navigator.clipboard.writeText(editorState.markdown);
    notice = '本地内容已复制。';
  }

  function formatTime(value: string) {
    if (!value) return '-';
    return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value));
  }

  function failedPolishState(lastError: string): PolishState {
    const technicalDetail = (lastError || '').trim();
    const normalized = technicalDetail.toLowerCase();
    let detail = '自动重试已结束，首版方案仍未生成。可以重新生成。';
    if (/status\s+524|"(?:status|error_code)"\s*:\s*524|origin_response_timeout/.test(normalized)) {
      detail = '上游服务等待超时，自动重试已结束。可以重新生成。';
    } else if (/status\s+(502|503|504)|bad gateway|service unavailable/.test(normalized)) {
      detail = '上游服务暂时不可用，自动重试已结束。可以重新生成。';
    } else if (/database is locked|database locked/.test(normalized)) {
      detail = '后台存储暂时繁忙，自动重试已结束。可以重新生成。';
    } else if (/timeout|timed out|deadline exceeded/.test(normalized)) {
      detail = '生成等待超时，自动重试已结束。可以重新生成。';
    }
    return { label: '方案生成失败', detail, active: false, danger: true, technicalDetail: technicalDetail || undefined };
  }

  function polishStateFor(job: PolishJob | null): PolishState | null {
    if (!job) return null;
    if (job.status === 'running') {
      return { label: '正在生成方案', detail: `首版方案正在后台生成 · 开始于 ${formatTime(job.started_at || job.updated_at)}`, active: true, danger: false };
    }
    if (job.status === 'queued' && (job.attempt_count > 0 || job.next_attempt_at)) {
      const retryAt = job.next_attempt_at && new Date(job.next_attempt_at).getTime() > Date.now()
        ? `，预计 ${formatTime(job.next_attempt_at)} 自动重试`
        : job.next_attempt_at ? '，已到重试时间，正在等待后台队列' : '，系统将自动重试';
      return { label: '等待重试', detail: `上次执行未完成${retryAt}`, active: true, danger: false };
    }
    if (job.status === 'queued') {
      return { label: '排队中', detail: `任务已进入后台串行队列 · 提交于 ${formatTime(job.created_at)}`, active: true, danger: false };
    }
    if (job.status === 'failed') {
      return failedPolishState(job.last_error);
    }
    return null;
  }
</script>

{#if canRead}
  <section id="demand-solution" class="solution-workspace" class:is-empty={!loading && !hasSolutionContent} aria-label="需求方案">
    {#if loading}
      <div class="solution-empty">正在读取方案…</div>
    {:else if error && !workspace}
      <div class="solution-message error" role="alert">{error}</div>
    {:else if !workspace || !hasSolutionContent}
      <div class="solution-empty is-compact">
        <strong>暂无解决方案</strong>
        <p>Jira 尚未同步到标记为方案的评论。</p>
      </div>
    {:else}
      {#if error && !editorOpen}<div class="solution-message error" role="alert">{error}</div>{/if}
      {#if notice && !editorOpen}<div class="solution-message success" aria-live="polite">{notice}</div>{/if}
      {#if polishState}
        <div
          class:danger={polishState.danger}
          class="solution-job-state"
          role={polishState.danger ? 'alert' : undefined}
          aria-live="polite"
          aria-busy={action === 'retry'}
        >
          <div class="solution-job-copy">
            <strong>{polishState.label}</strong>
            <span>{polishState.detail}</span>
          </div>
          {#if retryableFailedJob}
            <button
              type="button"
              class="primary solution-job-retry"
              disabled={!!action}
              aria-label={`重新生成 ${demand.task_id} 的方案`}
              on:click={retryFailedSolution}
            >{action === 'retry' ? '重新排队中…' : '重新生成'}</button>
          {/if}
          {#if polishState.technicalDetail}
            <details class="solution-job-details">
              <summary>查看技术详情</summary>
              <pre>{polishState.technicalDetail}</pre>
            </details>
          {/if}
        </div>
      {/if}
      {#if !editorOpen && (conflict || editorState.remoteUpdateAvailable)}
        <div class="solution-conflict" role="alert">
          <div><strong>检测到远端更新</strong><span>当前未保存内容没有被替换。</span></div>
          <div class="solution-actions"><button type="button" on:click={copyLocal}>复制本地内容</button><button type="button" on:click={reloadRemote}>加载远端内容</button></div>
        </div>
      {/if}

      {#if workspace.working}
        <div class="solution-preview-content">
          <div class="solution-toolbar">
            {#if canWrite}<button type="button" class="primary" disabled={!!action} on:click={openEditor}>编辑方案</button>{/if}
          </div>

          <MarkdownWorkbench
            value={workspace.working.markdown}
            mode="preview"
            readonly
            label="方案预览"
            description={`已同步 · ${formatTime(workspace.working.created_at)}`}
            minHeight={420}
          />
        </div>
      {/if}
    {/if}
  </section>

  <Modal
    show={editorOpen}
    title="即时修改方案"
    size="wide"
    closeLabel="关闭方案编辑"
    hideBodyScrollbar={true}
    on:close={requestCloseEditor}
  >
    {#if workspace?.working}
      <div class="solution-editor-body">
        {#if action === 'fork'}
          <div class="solution-editor-state" aria-live="polite">
            <strong>正在准备编辑</strong>
            <span>正在建立可编辑草稿，请稍候。</span>
          </div>
        {/if}
        {#if error}<div class="solution-message error" role="alert">{error}</div>{/if}
        {#if notice}<div class="solution-message success" aria-live="polite">{notice}</div>{/if}
        {#if conflict || editorState.remoteUpdateAvailable}
          <div class="solution-conflict" role="alert">
            <div><strong>检测到远端更新</strong><span>当前未保存内容没有被替换。</span></div>
            <div class="solution-actions"><button type="button" on:click={copyLocal}>复制本地内容</button><button type="button" on:click={reloadRemote}>加载远端内容</button></div>
          </div>
        {/if}
        {#if discardPrompt}
          <div class="solution-discard" role="alert">
            <div><strong>有未保存修改</strong><span>放弃后将恢复为最近一次保存的内容。</span></div>
            <div class="solution-actions">
              <button type="button" on:click={() => discardPrompt = false}>继续编辑</button>
              <button type="button" class="danger" on:click={discardEditorChanges}>放弃修改</button>
            </div>
          </div>
        {/if}

        <MarkdownWorkbench
          value={editorState.markdown}
          mode="live"
          readonly={!canWrite || workspace.working.status !== 'draft' || action === 'fork'}
          label="即时编辑"
          description={editorState.dirty ? '有未保存修改' : `已同步 · ${formatTime(workspace.working.created_at)}`}
          minHeight={520}
          embedded={true}
          saving={action === 'save' || action === 'publish'}
          on:change={handleMarkdownChange}
          on:save={saveDraft}
        />
      </div>

    {/if}

    <div slot="footer" class="solution-editor-footer">
      {#if workspace?.working}
        <span aria-live="polite">
          {#if action === 'fork'}正在准备编辑{:else if editorState.dirty}有未保存修改{:else}内容已保存{/if}
        </span>
        <div class="solution-actions">
          <button
            type="button"
            disabled={!editorState.dirty || !!action || workspace.working.status !== 'draft'}
            on:click={saveDraft}
          >{action === 'save' && !publishFlow ? '保存中…' : '保存'}</button>
          {#if canPublish}
            <button
              type="button"
              class="primary"
              disabled={!!action || workspace.working.status !== 'draft'}
              on:click={saveAndPublish}
            >{action === 'publish' ? '发布中…' : action === 'save' && publishFlow ? '保存后发布…' : '发布'}</button>
          {/if}
        </div>
      {/if}
    </div>
  </Modal>
{/if}

<style>
  .solution-workspace { container-type:inline-size; min-height:0; display:flex; flex:1 1 auto; flex-direction:column; gap:var(--wa-space-4,16px); color:var(--wa-text-main,#293847); }
  .solution-workspace.is-empty { justify-content:flex-start; }
  .solution-preview-content { min-height:0; display:grid; flex:1 1 auto; grid-template-rows:auto minmax(0,1fr); gap:var(--wa-space-4,16px); }
  .solution-preview-content > :global(.markdown-workbench) { height:100%; min-height:0; }
  .solution-toolbar,.solution-conflict { display:flex; align-items:center; justify-content:space-between; gap:14px; }
  .solution-actions { display:flex; flex-wrap:wrap; gap:8px; }
  button { min-height:40px; box-sizing:border-box; padding:0 14px; border:1px solid var(--wa-border-strong,rgba(91,119,137,.28)); border-radius:8px; background:var(--wa-surface,#fbfdff); color:var(--wa-text-main,#293847); font:700 12px/1 var(--wa-font-sans,system-ui); cursor:pointer; }
  button.primary { border-color:var(--wa-accent,#008f96); background:var(--wa-accent,#008f96); color:var(--wa-accent-fill-ink,#f6fbff); }
  button:disabled { opacity:.48; cursor:not-allowed; }
  .solution-empty { display:grid; justify-items:start; gap:8px; padding:20px; border:1px dashed var(--wa-border-strong,rgba(91,119,137,.28)); border-radius:10px; background:var(--wa-surface-soft,#f4f8f9); }
  .solution-empty.is-compact { min-height:104px; align-content:center; }
  .solution-empty strong { color:var(--wa-text-strong,#0d1722); font-size:14px; }
  .solution-empty p { margin:0; color:var(--wa-text-muted,#667789); font-size:13px; }
  .solution-message,.solution-conflict { padding:11px 13px; border-radius:8px; font-size:13px; }
  .solution-message.error,.solution-conflict { border:1px solid rgba(194,116,43,.3); background:#fff8ed; color:#7a4318; }
  .solution-message.success { border:1px solid rgba(32,133,105,.24); background:#f0fbf7; color:#17634f; }
  .solution-job-state { display:grid; grid-template-columns:minmax(0,1fr) auto; align-items:start; gap:10px 12px; padding:0 2px; color:var(--wa-text-muted,#667789); font-size:12px; }
  .solution-job-copy { display:flex; min-width:0; align-items:baseline; gap:8px; }
  .solution-job-copy span { min-width:0; overflow-wrap:anywhere; }
  .solution-job-state strong { color:var(--wa-accent-strong,#006f76); font-size:12px; white-space:nowrap; }
  .solution-job-state.danger { padding:12px 13px; border:1px solid rgba(188,65,81,.2); border-radius:8px; background:#fff7f8; color:#7f3a45; }
  .solution-job-state.danger strong { color:#9a3542; }
  .solution-job-retry { align-self:center; white-space:nowrap; }
  .solution-job-retry:not(:disabled):hover { filter:brightness(.96); }
  .solution-job-retry:not(:disabled):active { transform:translateY(1px); }
  .solution-job-retry:focus-visible,.solution-job-details summary:focus-visible { outline:2px solid var(--wa-focus-ring,#008f96); outline-offset:2px; }
  .solution-job-details { grid-column:1 / -1; min-width:0; }
  .solution-job-details summary { width:max-content; max-width:100%; color:#7f3a45; font-weight:700; cursor:pointer; text-underline-offset:3px; }
  .solution-job-details[open] summary { margin-bottom:8px; }
  .solution-job-details pre { max-height:160px; margin:0; padding:10px; overflow:auto; border-radius:6px; background:rgba(122,52,66,.06); color:#5f3039; font:500 11px/1.55 var(--wa-font-mono,ui-monospace,SFMono-Regular,monospace); white-space:pre-wrap; overflow-wrap:anywhere; }
  .solution-conflict>div:first-child { display:grid; gap:3px; }
  .solution-conflict span { font-size:12px; }
  .solution-editor-body { display:grid; gap:0; padding:0; }
  .solution-editor-body :global(.markdown-workbench) { height:clamp(320px,calc(100dvh - 250px),560px); }
  .solution-editor-body > .solution-editor-state,
  .solution-editor-body > .solution-message,
  .solution-editor-body > .solution-conflict,
  .solution-editor-body > .solution-discard { margin:16px 22px 0; }
  .solution-editor-state { display:grid; gap:3px; color:var(--wa-text-muted,#667789); font-size:12px; }
  .solution-editor-state strong { color:var(--wa-accent-strong,#006f76); }
  .solution-discard { display:flex; align-items:center; justify-content:space-between; gap:14px; padding:11px 13px; border:1px solid rgba(194,116,43,.3); border-radius:8px; background:#fff8ed; color:#7a4318; font-size:13px; }
  .solution-discard>div:first-child { display:grid; gap:3px; }
  .solution-discard span { font-size:12px; }
  button.danger { border-color:rgba(200,22,29,.34); color:var(--wa-danger,#c8161d); }
  .solution-editor-footer { display:flex; align-items:center; justify-content:space-between; gap:16px; }
  .solution-editor-footer>span { color:var(--wa-text-muted,#667789); font-size:12px; }
  @container (max-width: 700px) { .solution-toolbar,.solution-conflict { align-items:stretch; flex-direction:column; } .solution-actions button { flex:1 1 auto; } .solution-job-state { grid-template-columns:minmax(0,1fr); } .solution-job-copy { align-items:flex-start; flex-direction:column; gap:4px; } .solution-job-retry { width:100%; } }
  @media (max-width: 1280px) {
    .solution-workspace { flex:0 0 auto; }
    .solution-preview-content { flex:0 0 auto; grid-template-rows:auto auto; }
    .solution-preview-content > :global(.markdown-workbench) { height:var(--workbench-height); min-height:auto; }
  }
  @media (max-width: 860px) {
    .solution-toolbar,.solution-conflict,.solution-discard,.solution-editor-footer { align-items:stretch; flex-direction:column; }
    .solution-actions button { min-height:44px; flex:1 1 auto; }
    .solution-job-retry { min-height:44px; }
    .solution-editor-body :global(.markdown-workbench) { height:clamp(280px,calc(100dvh - 288px),560px); }
    .solution-editor-body > .solution-editor-state,
    .solution-editor-body > .solution-message,
    .solution-editor-body > .solution-conflict,
    .solution-editor-body > .solution-discard { margin-inline:16px; }
    .solution-editor-footer>span { min-height:18px; }
  }
</style>
