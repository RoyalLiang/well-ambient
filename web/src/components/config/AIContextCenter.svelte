<script lang="ts">
  import { onMount } from 'svelte';
  import Button from '../shared/Button.svelte';

  interface AIContextFeature {
    feature: string;
    type: string;
    status: string;
    configurable: string;
    scenario: string;
    description: string;
  }

  interface AIContextDiff {
    against_id?: number;
    against_version?: number;
    added: string[];
    removed: string[];
    changed: string[];
    added_count: number;
    removed_count: number;
    changed_count: number;
    summary: string;
  }

  interface AIContextProfile {
    id: number;
    module_name: string;
    source_filename: string;
    source_sheet: string;
    summary: string;
    prompt_summary: string;
    feature_count: number;
    configurable_count: number;
    non_configurable_count: number;
    status_breakdown: Record<string, number>;
    type_breakdown: Record<string, number>;
    feature_snapshot: AIContextFeature[];
    enabled: boolean;
    version: number;
    diff?: AIContextDiff;
    created_at: string;
    updated_at: string;
  }

  let profiles: AIContextProfile[] = [];
  let selectedProfileID: number | null = null;
  let selectedFile: File | null = null;
  let moduleName = '路径规划模块';
  let loading = false;
  let importing = false;
  let togglingID: number | null = null;
  let error = '';
  let success = '';
  let latestDiff: AIContextDiff | null = null;
  let latestImportProfileID: number | null = null;

  $: selectedProfile = profiles.find((profile) => profile.id === selectedProfileID) || profiles[0];
  $: enabledCount = profiles.filter((profile) => profile.enabled).length;
  $: selectedDiff = selectedProfile?.diff || (selectedProfile?.id === latestImportProfileID ? latestDiff : null);

  onMount(() => {
    fetchProfiles();
  });

  async function fetchProfiles() {
    loading = true;
    error = '';
    try {
      const res = await fetch('/api/ai-context/profiles');
      if (!res.ok) throw new Error(await res.text());
      profiles = await res.json();
      if (!selectedProfileID && profiles.length > 0) {
        selectedProfileID = profiles[0].id;
      }
    } catch (e: any) {
      error = e.message || '上下文画像加载失败';
    } finally {
      loading = false;
    }
  }

  function handleFileChange(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    selectedFile = input.files?.[0] || null;
    if (selectedFile && (!moduleName || moduleName === '未命名模块')) {
      moduleName = selectedFile.name.replace(/\.[^.]+$/, '');
    }
    error = '';
    success = '';
  }

  async function importProfile() {
    if (!selectedFile) {
      error = '请先选择 Excel 或 CSV 文件';
      return;
    }
    importing = true;
    error = '';
    success = '';
    latestDiff = null;

    const formData = new FormData();
    formData.append('file', selectedFile);
    formData.append('module_name', moduleName.trim() || selectedFile.name.replace(/\.[^.]+$/, ''));

    try {
      const res = await fetch('/api/ai-context/import', {
        method: 'POST',
        body: formData
      });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      latestDiff = data.diff || null;
      latestImportProfileID = data.profile?.id || null;
      selectedProfileID = latestImportProfileID;
      success = `已生成 ${data.profile?.module_name || '模块'} v${data.profile?.version || ''} 画像`;
      await fetchProfiles();
    } catch (e: any) {
      error = e.message || '导入失败';
    } finally {
      importing = false;
    }
  }

  async function toggleProfile(profile: AIContextProfile) {
    togglingID = profile.id;
    error = '';
    success = '';
    try {
      const res = await fetch(`/api/ai-context/profiles/${profile.id}/toggle`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: !profile.enabled })
      });
      if (!res.ok) throw new Error(await res.text());
      await fetchProfiles();
      success = `${profile.module_name} v${profile.version} 已${profile.enabled ? '停用' : '启用'}`;
    } catch (e: any) {
      error = e.message || '状态更新失败';
    } finally {
      togglingID = null;
    }
  }

  function formatDate(value: string) {
    if (!value) return '-';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString();
  }

  function breakdownEntries(values: Record<string, number>) {
    return Object.entries(values || {}).sort((a, b) => b[1] - a[1]).slice(0, 5);
  }

  function diffItems(items: string[]) {
    if (!items || items.length === 0) return '无';
    return items.slice(0, 5).join(' / ') + (items.length > 5 ? ` / +${items.length - 5}` : '');
  }

  function handleProfileKeydown(event: KeyboardEvent, profileID: number) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      selectedProfileID = profileID;
    }
  }
</script>

<section class="context-center">
  <div class="center-header">
    <div>
      <span class="center-kicker">Context Registry</span>
      <h4>AI 上下文配置中心</h4>
    </div>
    <div class="center-stats font-mono">
      <span>{profiles.length} versions</span>
      <span>{enabledCount} active</span>
    </div>
  </div>

  <div class="upload-grid">
    <label class="module-field">
      <span>模块名称</span>
      <input bind:value={moduleName} placeholder="例如：路径规划模块" />
    </label>

    <label class="file-field">
      <input type="file" accept=".xlsx,.csv" on:change={handleFileChange} />
      <span class="file-title">{selectedFile ? selectedFile.name : '上传 Excel / CSV'}</span>
      <span class="file-meta">{selectedFile ? `${Math.max(1, Math.round(selectedFile.size / 1024))} KB` : 'first sheet parser'}</span>
    </label>

    <div class="upload-action">
      <Button variant="primary" loading={importing} on:click={importProfile}>
        生成模块画像
      </Button>
    </div>
  </div>

  {#if error}
    <div class="message error font-mono">{error}</div>
  {/if}
  {#if success}
    <div class="message success font-mono">{success}</div>
  {/if}

  <div class="center-body">
    <div class="profile-list">
      {#if loading}
        <div class="empty-state">加载上下文画像...</div>
      {:else if profiles.length === 0}
        <div class="empty-state">暂无模块画像</div>
      {:else}
        {#each profiles as profile}
          <div
            role="button"
            tabindex="0"
            class="profile-row {selectedProfile?.id === profile.id ? 'active' : ''}"
            on:click={() => selectedProfileID = profile.id}
            on:keydown={(event) => handleProfileKeydown(event, profile.id)}
          >
            <div class="profile-main">
              <span class="profile-title">{profile.module_name} v{profile.version}</span>
              <span class="profile-source font-mono">{profile.source_filename} · {profile.source_sheet}</span>
            </div>
            <div class="profile-meta">
              <span class="profile-count">{profile.feature_count} 项</span>
              <span class="state-pill {profile.enabled ? 'enabled' : ''}">{profile.enabled ? '启用' : '停用'}</span>
            </div>
            <button
              type="button"
              class="toggle-btn"
              disabled={togglingID === profile.id}
              on:click|stopPropagation={() => toggleProfile(profile)}
            >
              {profile.enabled ? '停用' : '启用'}
            </button>
          </div>
        {/each}
      {/if}
    </div>

    <div class="profile-detail">
      {#if selectedProfile}
        <div class="detail-top">
          <div>
            <h5>{selectedProfile.module_name} v{selectedProfile.version}</h5>
            <p>{formatDate(selectedProfile.updated_at)} · {selectedProfile.feature_count} 项能力 · 可配置 {selectedProfile.configurable_count}</p>
          </div>
          <span class="state-pill {selectedProfile.enabled ? 'enabled' : ''}">{selectedProfile.enabled ? 'Prompt 生效中' : '未注入 Prompt'}</span>
        </div>

        <div class="breakdown-row">
          {#each breakdownEntries(selectedProfile.status_breakdown) as [label, count]}
            <span>{label} {count}</span>
          {/each}
          {#each breakdownEntries(selectedProfile.type_breakdown) as [label, count]}
            <span>{label} {count}</span>
          {/each}
        </div>

        <div class="prompt-preview">
          <div class="preview-title">
            <span>Prompt 摘要预览</span>
            <span class="font-mono">{selectedProfile.prompt_summary.length} chars</span>
          </div>
          <pre class="font-mono">{selectedProfile.prompt_summary}</pre>
        </div>

        {#if selectedDiff}
          <div class="diff-panel">
            <div class="diff-title">
              <span>版本差异</span>
              <span class="font-mono">{selectedDiff.summary}</span>
            </div>
            <div class="diff-grid">
              <div>
                <b>新增 {selectedDiff.added_count}</b>
                <p>{diffItems(selectedDiff.added)}</p>
              </div>
              <div>
                <b>移除 {selectedDiff.removed_count}</b>
                <p>{diffItems(selectedDiff.removed)}</p>
              </div>
              <div>
                <b>变化 {selectedDiff.changed_count}</b>
                <p>{diffItems(selectedDiff.changed)}</p>
              </div>
            </div>
          </div>
        {/if}
      {:else}
        <div class="empty-state">选择或导入一个模块画像后查看摘要</div>
      {/if}
    </div>
  </div>
</section>

<style>
  .context-center {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 14px;
    background: rgba(2, 6, 23, 0.38);
    border: 1px solid rgba(51, 65, 85, 0.58);
    border-radius: 8px;
  }

  .center-header,
  .detail-top,
  .preview-title,
  .diff-title {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 12px;
  }

  .center-kicker {
    color: #a78bfa;
    font-size: 0.65rem;
    font-weight: 800;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .center-header h4,
  .detail-top h5 {
    margin: 3px 0 0 0;
    color: #e2e8f0;
    font-size: 0.95rem;
    font-weight: 700;
  }

  .center-stats {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 8px;
  }

  .center-stats span,
  .breakdown-row span {
    border: 1px solid rgba(51, 65, 85, 0.5);
    background: rgba(15, 23, 42, 0.54);
    border-radius: 4px;
    color: #94a3b8;
    font-size: 0.68rem;
    padding: 4px 7px;
  }

  .upload-grid {
    display: grid;
    grid-template-columns: minmax(180px, 0.9fr) minmax(220px, 1.3fr) auto;
    gap: 10px;
    align-items: end;
  }

  .module-field,
  .file-field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .module-field span {
    color: #94a3b8;
    font-size: 0.75rem;
    font-weight: 700;
  }

  .module-field input {
    height: 38px;
    box-sizing: border-box;
    width: 100%;
    background: rgba(2, 6, 23, 0.62);
    border: 1px solid rgba(71, 85, 105, 0.62);
    border-radius: 6px;
    color: #e2e8f0;
    font: inherit;
    font-size: 0.82rem;
    outline: none;
    padding: 0 10px;
  }

  .module-field input:focus {
    border-color: rgba(167, 139, 250, 0.72);
    box-shadow: 0 0 0 3px rgba(167, 139, 250, 0.1);
  }

  .file-field {
    min-height: 38px;
    justify-content: center;
    box-sizing: border-box;
    padding: 8px 10px;
    border: 1px dashed rgba(129, 140, 248, 0.44);
    border-radius: 6px;
    background: rgba(15, 23, 42, 0.45);
    cursor: pointer;
  }

  .file-field input {
    display: none;
  }

  .file-title {
    color: #e2e8f0;
    font-size: 0.82rem;
    font-weight: 700;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .file-meta {
    color: #64748b;
    font-size: 0.68rem;
  }

  .upload-action {
    display: flex;
    justify-content: flex-end;
  }

  .message {
    border-radius: 6px;
    padding: 8px 10px;
    font-size: 0.72rem;
    white-space: pre-wrap;
  }

  .message.error {
    border: 1px solid rgba(248, 113, 113, 0.28);
    background: rgba(127, 29, 29, 0.15);
    color: #fca5a5;
  }

  .message.success {
    border: 1px solid rgba(52, 211, 153, 0.26);
    background: rgba(6, 78, 59, 0.16);
    color: #86efac;
  }

  .center-body {
    display: grid;
    grid-template-columns: minmax(230px, 0.72fr) minmax(0, 1.28fr);
    gap: 12px;
    min-height: 300px;
  }

  .profile-list,
  .profile-detail {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.44);
    border-radius: 8px;
    background: rgba(15, 23, 42, 0.3);
  }

  .profile-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 10px;
    max-height: 430px;
    overflow-y: auto;
  }

  .profile-row {
    position: relative;
    width: 100%;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 8px 10px;
    text-align: left;
    padding: 10px;
    border: 1px solid rgba(51, 65, 85, 0.45);
    border-radius: 7px;
    background: rgba(2, 6, 23, 0.34);
    color: inherit;
    cursor: pointer;
    box-sizing: border-box;
  }

  .profile-row.active {
    border-color: rgba(167, 139, 250, 0.56);
    background: rgba(88, 28, 135, 0.16);
  }

  .profile-row:focus-visible {
    outline: 2px solid rgba(167, 139, 250, 0.6);
    outline-offset: 2px;
  }

  .profile-main,
  .profile-meta {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }

  .profile-title {
    color: #e2e8f0;
    font-size: 0.82rem;
    font-weight: 800;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .profile-source {
    color: #64748b;
    font-size: 0.66rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .profile-count {
    color: #94a3b8;
    font-size: 0.7rem;
    text-align: right;
  }

  .state-pill {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 22px;
    white-space: nowrap;
    border-radius: 999px;
    border: 1px solid rgba(148, 163, 184, 0.22);
    background: rgba(148, 163, 184, 0.08);
    color: #94a3b8;
    font-size: 0.68rem;
    font-weight: 800;
    padding: 2px 8px;
  }

  .state-pill.enabled {
    border-color: rgba(52, 211, 153, 0.28);
    background: rgba(6, 78, 59, 0.2);
    color: #86efac;
  }

  .toggle-btn {
    grid-column: 1 / -1;
    justify-self: flex-end;
    background: rgba(15, 23, 42, 0.7);
    border: 1px solid rgba(51, 65, 85, 0.7);
    border-radius: 5px;
    color: #cbd5e1;
    cursor: pointer;
    font-size: 0.68rem;
    font-weight: 800;
    padding: 5px 8px;
  }

  .toggle-btn:hover {
    border-color: rgba(167, 139, 250, 0.5);
    color: #ede9fe;
  }

  .profile-detail {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 12px;
  }

  .detail-top p {
    margin: 4px 0 0 0;
    color: #64748b;
    font-size: 0.72rem;
  }

  .breakdown-row {
    display: flex;
    flex-wrap: wrap;
    gap: 7px;
  }

  .prompt-preview,
  .diff-panel {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.42);
    border-radius: 8px;
    background: rgba(2, 6, 23, 0.28);
  }

  .preview-title,
  .diff-title {
    padding: 9px 10px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.36);
    color: #cbd5e1;
    font-size: 0.75rem;
    font-weight: 800;
  }

  .preview-title span:last-child,
  .diff-title span:last-child {
    color: #64748b;
    font-weight: 600;
  }

  .prompt-preview pre {
    margin: 0;
    max-height: 210px;
    overflow: auto;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    color: #cbd5e1;
    font-size: 0.72rem;
    line-height: 1.55;
    padding: 10px;
  }

  .diff-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
    padding: 10px;
  }

  .diff-grid div {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.35);
    border-radius: 6px;
    padding: 8px;
    background: rgba(15, 23, 42, 0.4);
  }

  .diff-grid b {
    display: block;
    color: #e2e8f0;
    font-size: 0.75rem;
    margin-bottom: 4px;
  }

  .diff-grid p {
    margin: 0;
    color: #64748b;
    font-size: 0.7rem;
    line-height: 1.45;
    overflow-wrap: anywhere;
  }

  .empty-state {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 120px;
    color: #64748b;
    font-size: 0.78rem;
    text-align: center;
    padding: 16px;
  }

  .profile-list,
  .prompt-preview pre {
    scrollbar-width: thin;
    scrollbar-color: rgba(167, 139, 250, 0.44) rgba(15, 23, 42, 0.72);
  }

  .profile-list::-webkit-scrollbar,
  .prompt-preview pre::-webkit-scrollbar {
    width: 8px;
    height: 8px;
  }

  .profile-list::-webkit-scrollbar-track,
  .prompt-preview pre::-webkit-scrollbar-track {
    background: rgba(2, 6, 23, 0.42);
    border-radius: 999px;
  }

  .profile-list::-webkit-scrollbar-thumb,
  .prompt-preview pre::-webkit-scrollbar-thumb {
    background: linear-gradient(180deg, rgba(167, 139, 250, 0.56), rgba(56, 189, 248, 0.36));
    border: 2px solid rgba(2, 6, 23, 0.42);
    border-radius: 999px;
  }

  .font-mono {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  @media (max-width: 980px) {
    .upload-grid,
    .center-body,
    .diff-grid {
      grid-template-columns: minmax(0, 1fr);
    }

    .upload-action {
      justify-content: flex-start;
    }
  }
</style>
