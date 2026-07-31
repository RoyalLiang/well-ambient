<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import Button from '../shared/Button.svelte';
  import MarkdownWorkbench, { type MarkdownMode } from '../shared/MarkdownWorkbench.svelte';
  import Select from '../shared/Select.svelte';

  export let currentUserPermissions: string[] = [];
  export let aiReady = false;

  type ContextDocument = {
    id: number;
    parent_document_id?: number;
    title: string;
    original_name: string;
    mime_type: string;
    type: string;
    scope: string;
    scope_id: string;
    source: string;
    owner: string;
    imported_by: string;
    status: string;
    ingestion_status: string;
    ingestion_error?: string;
    version: number;
    content_hash: string;
    summary: string;
    content: string;
    token_count: number;
    candidate_count?: number;
    pending_count?: number;
    published_count?: number;
    created_at: string;
    updated_at: string;
  };

  const dispatch = createEventDispatcher<{ imported: { candidateCount: number }; openreview: void }>();
  const scopeOptions = [
    { value: 'global', label: '全局' },
    { value: 'repo', label: '仓库' },
    { value: 'module', label: '模块' },
    { value: 'demand_type', label: '需求类型' }
  ];
  const supportedFileExtensions = new Set([
    'md', 'markdown', 'txt', 'json', 'html', 'htm', 'xml', 'pdf', 'doc', 'docx', 'rtf', 'odt', 'ppt', 'pptx', 'csv', 'tsv', 'xls', 'xlsx'
  ]);
  const maxUploadBytes = 10 * 1024 * 1024;
  type ImportSourceMode = 'file' | 'markdown';

  let documents: ContextDocument[] = [];
  let selectedDocument: ContextDocument | null = null;
  let selectedDocumentID = 0;
  let loading = false;
  let detailLoading = false;
  let error = '';
  let mode: 'view' | 'import' = 'view';
  let importing = false;
  let importError = '';
  let importSuccess = '';
  let importedCandidateCount = 0;
  let importTitle = '';
  let importOriginalName = 'pasted-markdown.md';
  let importMimeType = 'text/markdown';
  let importSourceMode: ImportSourceMode = 'file';
  let importFile: File | null = null;
  let importScope = 'global';
  let importScopeID = '';
  let importContent = '';
  let importMarkdownMode: MarkdownMode = 'live';

  $: canRead = currentUserPermissions.includes('ai_context:read');
  $: canWrite = currentUserPermissions.includes('ai_context:write');

  onMount(() => {
    if (canRead) void loadDocuments();
  });

  async function parseResponse(response: Response) {
    const text = await response.text();
    if (!text) return {};
    try {
      return JSON.parse(text);
    } catch {
      return { error: text };
    }
  }

  async function loadDocuments(preferredID = selectedDocumentID) {
    loading = true;
    error = '';
    try {
      const response = await fetch('/api/context/documents');
      const data = await parseResponse(response);
      if (!response.ok) throw new Error(data.error || data.message || `HTTP ${response.status}`);
      documents = Array.isArray(data.items) ? data.items : [];
      const nextID = preferredID && documents.some(item => item.id === preferredID)
        ? preferredID
        : documents[0]?.id || 0;
      if (nextID && mode === 'view') await selectDocument(nextID);
      if (!nextID) {
        selectedDocumentID = 0;
        selectedDocument = null;
      }
    } catch (reason: any) {
      error = reason.message || '原始资料加载失败';
    } finally {
      loading = false;
    }
  }

  async function selectDocument(id: number) {
    selectedDocumentID = id;
    mode = 'view';
    detailLoading = true;
    error = '';
    try {
      const response = await fetch(`/api/context/documents/${id}`);
      const data = await parseResponse(response);
      if (!response.ok) throw new Error(data.error || data.message || `HTTP ${response.status}`);
      selectedDocument = data.document || null;
    } catch (reason: any) {
      error = reason.message || '资料正文加载失败';
      selectedDocument = null;
    } finally {
      detailLoading = false;
    }
  }

  function openImport(base?: ContextDocument | null) {
    importError = '';
    importSuccess = '';
    importedCandidateCount = 0;
    if (base) {
      importTitle = base.title || '';
      importOriginalName = base.original_name || 'pasted-markdown.md';
      importMimeType = base.mime_type || 'text/markdown';
      importScope = base.scope || 'global';
      importScopeID = base.scope_id || '';
      importContent = base.content || '';
      importSourceMode = 'markdown';
    } else {
      importTitle = '';
      importOriginalName = 'pasted-markdown.md';
      importMimeType = 'text/markdown';
      importScope = 'global';
      importScopeID = '';
      importContent = '';
      importSourceMode = 'file';
    }
    importFile = null;
    importMarkdownMode = 'live';
    mode = 'import';
  }

  function handleImportScopeChange(event: CustomEvent<string>) {
    importScope = event.detail;
    if (importScope === 'global') importScopeID = '';
  }

  function handleFile(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    importError = '';
    const extension = file.name.toLowerCase().split('.').pop() || '';
    if (!supportedFileExtensions.has(extension)) {
      importError = '该文件类型不在当前 LLM 文档输入白名单中';
      input.value = '';
      return;
    }
    if (file.size > maxUploadBytes) {
      importError = '文件不能超过 10 MiB';
      input.value = '';
      return;
    }
    importFile = file;
    importOriginalName = file.name;
    importMimeType = file.type || 'application/octet-stream';
    if (!importTitle.trim()) importTitle = file.name.replace(/\.[^.]+$/, '');
    input.value = '';
  }

  function setImportSourceMode(nextMode: ImportSourceMode) {
    importSourceMode = nextMode;
    importError = '';
    importSuccess = '';
  }

  function clearImportFile() {
    importFile = null;
    importOriginalName = 'pasted-markdown.md';
    importMimeType = 'text/markdown';
  }

  function formatFileSize(bytes: number) {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
  }

  async function importDocument() {
    importError = '';
    importSuccess = '';
    if (!aiReady) {
      importError = '请先启用并保存可用的 AI 引擎配置';
      return;
    }
    if (!importTitle.trim()) {
      importError = '请填写资料标题';
      return;
    }
    if (importSourceMode === 'file' && !importFile) {
      importError = '请选择需要直接交给 LLM 的资料文件';
      return;
    }
    if (importSourceMode === 'markdown' && !importContent.trim()) {
      importError = '请填写需要提交给 LLM 的 Markdown 内容';
      return;
    }
    if (importScope !== 'global' && !importScopeID.trim()) {
      importError = '非全局资料需要填写范围标识';
      return;
    }
    importing = true;
    try {
      let response: Response;
      if (importSourceMode === 'file' && importFile) {
        const body = new FormData();
        body.set('title', importTitle.trim());
        body.set('type', 'system_design');
        body.set('scope', importScope);
        body.set('scope_id', importScope === 'global' ? '' : importScopeID.trim());
        body.set('file', importFile, importFile.name);
        response = await fetch('/api/context/documents/import', { method: 'POST', body });
      } else {
        response = await fetch('/api/context/documents/import', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            title: importTitle.trim(),
            original_name: 'pasted-markdown.md',
            mime_type: 'text/markdown',
            type: 'system_design',
            scope: importScope,
            scope_id: importScope === 'global' ? '' : importScopeID.trim(),
            content: importContent
          })
        });
      }
      const data = await parseResponse(response);
      if (!response.ok) {
        const message = data.error || data.message || `HTTP ${response.status}`;
        if (data.document?.id) {
          mode = 'view';
          await loadDocuments(data.document.id);
          error = `${message}。资料元数据已保留，但文件正文未在本系统落库。可修正 AI 配置后重新提交文件。`;
          return;
        }
        throw new Error(message);
      }
      importedCandidateCount = Array.isArray(data.candidates) ? data.candidates.length : 0;
      importSuccess = data.reused
        ? `该资料已经解析，已复用现有版本和 ${importedCandidateCount} 条候选。`
        : `LLM 已完成资料解析，并生成 ${importedCandidateCount} 条待审核候选。`;
      const document = data.document as ContextDocument;
      selectedDocumentID = document?.id || 0;
      dispatch('imported', { candidateCount: importedCandidateCount });
      await loadDocuments(selectedDocumentID);
      mode = 'import';
    } catch (reason: any) {
      importError = reason.message || '资料导入和候选生成失败';
    } finally {
      importing = false;
    }
  }

  async function archiveSelectedDocument() {
    if (!selectedDocument || !confirm(`确认归档“${selectedDocument.title}”的原始资料版本？已发布事实不会自动停用。`)) return;
    error = '';
    try {
      const response = await fetch(`/api/context/documents/${selectedDocument.id}/archive`, { method: 'POST' });
      const data = await parseResponse(response);
      if (!response.ok) throw new Error(data.error || data.message || `HTTP ${response.status}`);
      selectedDocument = null;
      selectedDocumentID = 0;
      await loadDocuments();
    } catch (reason: any) {
      error = reason.message || '归档原始资料失败';
    }
  }

  function formatDate(value: string) {
    if (!value) return '暂无时间';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN');
  }

  function statusLabel(document: ContextDocument) {
    if (document.ingestion_status === 'processing') return '解析中';
    if (document.ingestion_status === 'failed') return '解析失败';
    return '已生成候选';
  }
</script>

{#if canRead}
  <section class="corpus-source-library" aria-label="原始资料与导入">
    <header class="source-library-header">
      <div>
        <h4>原始资料与导入</h4>
        <p>文件直接交给 LLM 解析，本系统只保存解析稿与候选；审核完成前不会进入生产上下文。</p>
      </div>
      {#if canWrite}
        <Button variant="primary" size="small" on:click={() => openImport()}>导入资料</Button>
      {/if}
    </header>

    {#if error}<div class="source-message error" role="alert">{error}</div>{/if}

    <div class="source-library-grid">
      <aside class="source-directory" aria-label="原始资料目录">
        <div class="source-directory-head">
          <strong>资料版本</strong>
          <button type="button" on:click={() => loadDocuments()} disabled={loading}>{loading ? '加载中' : '刷新'}</button>
        </div>
        {#if loading}
          <div class="source-skeleton" aria-label="原始资料加载中"><span></span><span></span><span></span></div>
        {:else if documents.length === 0}
          <div class="source-empty">
            <strong>暂无原始资料</strong>
            <p>上传资料文件或 Markdown 后，LLM 会在统一审核中心生成可追溯候选。</p>
          </div>
        {:else}
          <div class="source-list" role="list">
            {#each documents as document (document.id)}
              <button
                type="button"
                class:active={mode === 'view' && selectedDocumentID === document.id}
                on:click={() => selectDocument(document.id)}
              >
                <span class="source-row-meta">
                  <b>v{document.version || 1}</b>
                  <em class:failed={document.ingestion_status === 'failed'}>{statusLabel(document)}</em>
                </span>
                <strong>{document.title}</strong>
                <small>{document.original_name || 'pasted-markdown.md'}</small>
                <span class="source-row-facts">
                  <span>{document.candidate_count || 0} 候选</span>
                  <span>{document.pending_count || 0} 待审</span>
                  <span>{document.published_count || 0} 已发布</span>
                </span>
              </button>
            {/each}
          </div>
        {/if}
      </aside>

      <div class="source-workbench">
        {#if mode === 'import'}
          <div class="source-workbench-head">
            <div>
              <strong>导入资料</strong>
            </div>
            <Button variant="ghost" size="small" on:click={() => { mode = 'view'; importError = ''; }}>取消</Button>
          </div>

          {#if !aiReady}
            <div class="source-message warning">AI 引擎尚未启用或凭证不完整。可以先选择文件或编辑 Markdown，完成配置后再导入。</div>
          {/if}
          {#if importError}<div class="source-message error" role="alert">{importError}</div>{/if}
          {#if importSuccess}
            <div class="source-message success" role="status">
              <span>{importSuccess}</span>
              <button type="button" on:click={() => dispatch('openreview')}>前往统一审核</button>
            </div>
          {/if}

          <div class="source-import-fields">
            <label class="wide">
              <span>资料标题</span>
              <input placeholder="例如：订单系统架构与发布约束" bind:value={importTitle} />
            </label>
            <Select
              id="corpus-import-scope"
              label="适用范围"
              value={importScope}
              options={scopeOptions}
              searchable={false}
              compact={true}
              disabled={importing}
              on:change={handleImportScopeChange}
            />
            <label>
              <span>范围标识</span>
              <input placeholder={importScope === 'global' ? '全局资料可留空' : 'repo / module / demand type'} bind:value={importScopeID} disabled={importScope === 'global'} />
            </label>
          </div>

          <fieldset class="source-mode-fieldset">
            <legend>资料来源</legend>
            <div class="source-mode-switch" role="group" aria-label="选择资料来源">
              <button type="button" class:active={importSourceMode === 'file'} aria-pressed={importSourceMode === 'file'} on:click={() => setImportSourceMode('file')}>上传文件</button>
              <button type="button" class:active={importSourceMode === 'markdown'} aria-pressed={importSourceMode === 'markdown'} on:click={() => setImportSourceMode('markdown')}>Markdown</button>
            </div>
          </fieldset>

          {#if importSourceMode === 'file'}
            <div class="file-handoff">
              <label class="file-field">
                <span>选择资料文件</span>
                <input
                  type="file"
                  accept=".md,.markdown,.txt,.json,.html,.htm,.xml,.pdf,.doc,.docx,.rtf,.odt,.ppt,.pptx,.csv,.tsv,.xls,.xlsx"
                  on:change={handleFile}
                />
                <small>支持常用文本、PDF、Office 文档与表格，最大 10 MiB</small>
              </label>
              {#if importFile}
                <div class="selected-file" role="status">
                  <div>
                    <strong>{importFile.name}</strong>
                    <span>{importFile.type || '由扩展名识别 MIME'} · {formatFileSize(importFile.size)}</span>
                  </div>
                  <button type="button" on:click={clearImportFile} disabled={importing}>移除</button>
                </div>
              {:else}
                <p>尚未选择文件。浏览器不会读取文件正文，后端也不会把正文转换成提示词。</p>
              {/if}
              <div class="handoff-note">
                <strong>直接交给 LLM</strong>
                <span>后端仅校验文件大小、类型与完整性，并将原始字节封装为模型文件输入。资料正文只从 LLM 的结构化解析结果进入语料库。</span>
              </div>
            </div>
          {:else}
            <MarkdownWorkbench
              value={importContent}
              mode={importMarkdownMode}
              label="Markdown"
              description="输入时即时排版，当前行保留 Markdown 语法；提交后保存内容仍以 LLM 返回的解析稿为准"
              placeholder="# 系统设计资料\n\n架构、流程、边界、风险或术语内容…"
              minHeight={520}
              on:change={(event) => importContent = event.detail}
            />
          {/if}

          <div class="source-actions">
            <span>{importSourceMode === 'file' ? '文件正文不在本系统解析或落库。' : '人工文本提交后仍需经过 LLM 解析与审核。'} 不会直接创建 active Context Fact。</span>
            <Button variant="primary" loading={importing} disabled={!canWrite || !aiReady} on:click={importDocument}>交给 LLM 并生成候选</Button>
          </div>
        {:else if detailLoading}
          <div class="source-detail-loading" aria-live="polite">正在读取 LLM 解析稿…</div>
        {:else if selectedDocument}
          <div class="source-workbench-head">
            <div>
              <strong>{selectedDocument.title}</strong>
              <span>{selectedDocument.original_name} · v{selectedDocument.version || 1} · {formatDate(selectedDocument.created_at)}</span>
            </div>
            {#if canWrite}
              <div class="source-head-actions">
                <Button variant="secondary" size="small" on:click={() => openImport(selectedDocument)}>创建新版本</Button>
                <Button variant="danger" size="small" on:click={archiveSelectedDocument}>归档</Button>
              </div>
            {/if}
          </div>

          <div class="source-facts" aria-label="原始资料元数据">
            <span><b>范围</b>{selectedDocument.scope === 'global' ? 'global' : `${selectedDocument.scope}:${selectedDocument.scope_id}`}</span>
            <span><b>导入人</b>{selectedDocument.imported_by || selectedDocument.owner || 'system'}</span>
            <span><b>Token</b>{selectedDocument.token_count || 0}</span>
            <span><b>Hash</b><code>{selectedDocument.content_hash?.slice(0, 12) || '-'}</code></span>
          </div>

          {#if selectedDocument.ingestion_status === 'failed'}
            <div class="source-message error" role="alert">{selectedDocument.ingestion_error || 'LLM 解析失败，请重新提交原文件。'}</div>
          {/if}

          <MarkdownWorkbench
            value={selectedDocument.content || ''}
            mode="preview"
            readonly={true}
            label="LLM 解析稿"
            description="由模型返回并按版本保存；候选和正式事实均保留文件哈希与此解析版本引用"
            minHeight={560}
          />
        {:else}
          <div class="source-empty workbench-empty">
            {#if canWrite}<Button variant="primary" on:click={() => openImport()}>导入第一份资料</Button>{/if}
          </div>
        {/if}
      </div>
    </div>
  </section>
{/if}

<style>
  .corpus-source-library { min-width: 0; display: grid; gap: 16px; color: var(--scw-text, #293847); }
  .source-library-header, .source-workbench-head, .source-directory-head, .source-actions, .source-message.success { min-width: 0; display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
  .source-library-header { padding-bottom: 14px; border-bottom: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); }
  .source-library-header h4 { margin: 0; color: var(--scw-ink, #0d1722); font-size: 15px; text-wrap: balance; }
  .source-library-header p { max-width: 72ch; margin: 5px 0 0; color: var(--scw-muted, #667789); font-size: 12px; line-height: 1.5; text-wrap: pretty; }
  .source-library-grid { min-width: 0; display: grid; grid-template-columns: minmax(280px, 360px) minmax(0, 1fr); align-items: start; gap: 20px; }
  .source-directory, .source-workbench { min-width: 0; display: grid; gap: 14px; }
  .source-directory { position: sticky; top: 12px; }
  .source-directory-head { align-items: center; padding-bottom: 10px; border-bottom: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); }
  .source-directory-head strong, .source-workbench-head strong { color: var(--scw-ink, #0d1722); font-size: 13px; }
  .source-directory-head button, .source-message button { min-height: 32px; border: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); border-radius: 8px; background: rgba(251, 253, 254, .78); color: var(--scw-accent-strong, #006f76); padding: 0 10px; font: 760 12px/1 var(--wa-font-sans, sans-serif); cursor: pointer; }
  .source-directory-head button:focus-visible, .source-message button:focus-visible, .source-list > button:focus-visible, .source-mode-switch button:focus-visible, .selected-file button:focus-visible, .source-import-fields input:focus, .file-field input:focus { outline: 2px solid rgba(0, 143, 150, .28); outline-offset: 2px; }
  .source-list { max-height: 660px; overflow: auto; border: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); border-radius: 10px; background: rgba(251, 253, 254, .46); }
  .source-list > button { width: 100%; min-width: 0; display: grid; gap: 6px; padding: 12px; border: 0; border-bottom: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); background: transparent; color: var(--scw-text, #293847); text-align: left; cursor: pointer; }
  .source-list > button:last-child { border-bottom: 0; }
  .source-list > button:hover, .source-list > button.active { background: rgba(0, 143, 150, .06); }
  .source-list > button strong { overflow: hidden; color: var(--scw-ink, #0d1722); font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
  .source-list > button small { overflow: hidden; color: var(--scw-muted, #667789); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
  .source-row-meta, .source-row-facts { display: flex; align-items: center; flex-wrap: wrap; gap: 6px 10px; color: var(--scw-subtle, #8a99aa); font-size: 10px; }
  .source-row-meta b { color: var(--scw-accent-strong, #006f76); font-family: var(--wa-font-mono, monospace); }
  .source-row-meta em { color: var(--wa-success, #04966f); font-style: normal; }
  .source-row-meta em.failed { color: var(--wa-danger, #c84236); }
  .source-row-facts { font-family: var(--wa-font-mono, monospace); }
  .source-workbench-head { align-items: center; padding-bottom: 12px; border-bottom: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); }
  .source-workbench-head > div:first-child { min-width: 0; display: grid; gap: 4px; }
  .source-workbench-head span { color: var(--scw-muted, #667789); font-size: 11px; line-height: 1.45; }
  .source-head-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; }
  .source-import-fields { min-width: 0; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px 18px; }
  .source-import-fields label { min-width: 0; display: grid; gap: 7px; color: var(--scw-text, #293847); font-size: 12px; font-weight: 700; }
  .source-import-fields label.wide { grid-column: 1 / -1; }
  .source-import-fields input { width: 100%; min-height: 38px; box-sizing: border-box; border: 1px solid var(--scw-line-strong, rgba(69, 95, 118, .28)); border-radius: 8px; background: rgba(251, 253, 254, .84); color: var(--scw-ink, #0d1722); padding: 0 11px; font: 13px/1 var(--wa-font-sans, sans-serif); }
  .source-import-fields input:disabled { background: rgba(239, 244, 246, .88); color: var(--scw-subtle, #8a99aa); }
  .file-field small { color: var(--scw-muted, #667789); font-size: 10px; font-weight: 500; }
  .source-mode-fieldset { min-width: 0; display: grid; gap: 8px; margin: 0; padding: 0; border: 0; }
  .source-mode-fieldset legend { margin-bottom: 8px; color: var(--scw-text, #293847); font-size: 12px; font-weight: 700; }
  .source-mode-switch { width: fit-content; display: inline-flex; gap: 4px; padding: 3px; border: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); border-radius: 9px; background: rgba(239, 244, 246, .72); }
  .source-mode-switch button { min-height: 34px; border: 0; border-radius: 7px; background: transparent; color: var(--scw-muted, #667789); padding: 0 13px; font: 720 12px/1 var(--wa-font-sans, sans-serif); cursor: pointer; }
  .source-mode-switch button:hover { color: var(--scw-ink, #0d1722); }
  .source-mode-switch button.active { background: rgba(251, 253, 254, .96); color: var(--scw-accent-strong, #006f76); box-shadow: 0 1px 4px rgba(28, 53, 69, .08); }
  .file-handoff { min-width: 0; display: grid; gap: 14px; padding: 16px 0; border-top: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); border-bottom: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); }
  .file-field { min-width: 0; display: grid; gap: 7px; color: var(--scw-text, #293847); font-size: 12px; font-weight: 700; }
  .file-field input { width: 100%; min-height: 40px; box-sizing: border-box; border: 1px solid var(--scw-line-strong, rgba(69, 95, 118, .28)); border-radius: 8px; background: rgba(251, 253, 254, .84); color: var(--scw-text, #293847); padding: 7px 10px; font: 12px/1.4 var(--wa-font-sans, sans-serif); }
  .file-handoff > p { margin: 0; color: var(--scw-muted, #667789); font-size: 11px; line-height: 1.5; }
  .selected-file { min-width: 0; display: flex; align-items: center; justify-content: space-between; gap: 14px; padding: 11px 0; }
  .selected-file > div { min-width: 0; display: grid; gap: 4px; }
  .selected-file strong { overflow: hidden; color: var(--scw-ink, #0d1722); font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
  .selected-file span { overflow-wrap: anywhere; color: var(--scw-muted, #667789); font-size: 10px; }
  .selected-file button { min-height: 34px; flex: 0 0 auto; border: 1px solid rgba(200, 66, 54, .2); border-radius: 8px; background: rgba(200, 66, 54, .05); color: #96352d; padding: 0 11px; font: 720 11px/1 var(--wa-font-sans, sans-serif); cursor: pointer; }
  .selected-file button:disabled { opacity: .5; cursor: not-allowed; }
  .handoff-note { min-width: 0; display: grid; grid-template-columns: max-content minmax(0, 1fr); gap: 10px 14px; padding: 11px 12px; border-radius: 8px; background: rgba(0, 143, 150, .055); color: var(--scw-muted, #667789); font-size: 11px; line-height: 1.55; }
  .handoff-note strong { color: var(--scw-accent-strong, #006f76); font-size: 11px; }
  .source-actions { align-items: center; padding-top: 2px; }
  .source-actions > span { color: var(--scw-muted, #667789); font-size: 11px; }
  .source-facts { display: flex; flex-wrap: wrap; gap: 8px 18px; padding: 2px 0; color: var(--scw-muted, #667789); font-size: 11px; }
  .source-facts span { display: inline-flex; align-items: center; gap: 6px; }
  .source-facts b { color: var(--scw-text, #293847); }
  .source-facts code { color: var(--scw-accent-strong, #006f76); font-family: var(--wa-font-mono, monospace); }
  .source-message { padding: 10px 12px; border: 1px solid var(--scw-line, rgba(92, 116, 137, .16)); border-radius: 8px; color: var(--scw-text, #293847); font-size: 12px; line-height: 1.45; }
  .source-message.error { border-color: rgba(200, 66, 54, .24); background: rgba(200, 66, 54, .07); color: #96352d; }
  .source-message.warning { border-color: rgba(182, 109, 0, .24); background: rgba(182, 109, 0, .07); color: #86530b; }
  .source-message.success { align-items: center; border-color: rgba(4, 150, 111, .22); background: rgba(4, 150, 111, .07); color: #087657; }
  .source-skeleton { display: grid; gap: 9px; }
  .source-skeleton span { height: 70px; border-radius: 8px; background: rgba(226, 234, 238, .72); }
  .source-empty, .source-detail-loading { display: grid; justify-items: center; gap: 7px; padding: 28px 18px; border: 1px dashed var(--scw-line-strong, rgba(69, 95, 118, .28)); border-radius: 10px; background: rgba(239, 246, 248, .48); color: var(--scw-muted, #667789); font-size: 12px; text-align: center; }
  .source-empty strong { color: var(--scw-text, #293847); }
  .source-empty p { max-width: 48ch; margin: 0; line-height: 1.5; }
  .workbench-empty { min-height: 300px; align-content: center; }
  @media (max-width: 960px) {
    .source-library-grid { grid-template-columns: minmax(0, 1fr); }
    .source-directory { position: static; }
    .source-list { max-height: 320px; }
  }
  @media (max-width: 760px) {
    .source-library-header, .source-workbench-head, .source-actions, .source-message.success { display: grid; grid-template-columns: minmax(0, 1fr); }
    .source-import-fields { --wa-control-h: 44px; grid-template-columns: minmax(0, 1fr); }
    .source-import-fields label.wide { grid-column: auto; }
    .source-head-actions { justify-content: stretch; }
    .source-head-actions :global(.btn), .source-actions :global(.btn), .source-library-header :global(.btn) { width: 100%; min-height: 44px; }
    .source-mode-switch { width: 100%; }
    .source-mode-switch button { min-height: 44px; flex: 1 1 0; }
    .selected-file { align-items: stretch; display: grid; }
    .selected-file button, .source-directory-head button, .source-message button, .source-import-fields input, .file-field input { min-height: 44px; }
    .handoff-note { grid-template-columns: minmax(0, 1fr); }
  }
</style>
