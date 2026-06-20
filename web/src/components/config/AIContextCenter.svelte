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
    implemented_count?: number;
    partial_count?: number;
    not_implemented_count?: number;
    unknown_count?: number;
    configurable_count: number;
    non_configurable_count: number;
    implementation_breakdown?: Record<string, number>;
    status_breakdown: Record<string, number>;
    type_breakdown: Record<string, number>;
    feature_snapshot: AIContextFeature[];
    enabled: boolean;
    version: number;
    diff?: AIContextDiff;
    created_at: string;
    updated_at: string;
  }

  const IMPLEMENTED = '已实现';
  const PARTIAL = '部分可用';
  const NOT_IMPLEMENTED = '未实现';
  const UNKNOWN = '未标注';
  const implementationBuckets = [IMPLEMENTED, PARTIAL, NOT_IMPLEMENTED, UNKNOWN];

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

  $: selectedProfile = profiles.find((profile) => profile.id === selectedProfileID) || profiles[0] || null;
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
      error = e.message || '模块能力画像加载失败';
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
      error = '请先选择能力状态清单文件（CSV/XLSX 均可，文件只是导入通道）';
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
      success = `已更新 ${data.profile?.module_name || '模块'} v${data.profile?.version || ''} 的实现状态画像`;
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

  function classifyImplementation(status: string) {
    const normalized = (status || '').toLowerCase().trim().replace(/[\s/_-]/g, '');
    if (!normalized || includesAny(normalized, ['未标注', '未知', '不明确', '待确认', 'unknown', 'na', 'n/a'])) {
      return UNKNOWN;
    }
    if (includesAny(normalized, ['未实现', '未开发', '待开发', '不可用', '规划中', '计划中', 'backlog', 'planned', 'todo', 'notimplemented', 'notstarted'])) {
      return NOT_IMPLEMENTED;
    }
    if (includesAny(normalized, ['部分', '开发中', '进行中', '验证中', '待测试', '灰度', '已初步验证', '初步验证', 'partial', 'inprogress', 'testing', 'beta', 'wip'])) {
      return PARTIAL;
    }
    if (includesAny(normalized, ['已实现', '已耐久', '已完成', '开发完成', '已开发完成', '已上线', '已发布', '可用', 'done', 'implemented', 'complete', 'completed', 'production', 'ga', 'available'])) {
      return IMPLEMENTED;
    }
    return UNKNOWN;
  }

  function includesAny(value: string, needles: string[]) {
    return needles.some((needle) => value.includes(needle));
  }

  function countByBucket(profile: AIContextProfile, bucket: string) {
    if (profile.implementation_breakdown && typeof profile.implementation_breakdown[bucket] === 'number') {
      return profile.implementation_breakdown[bucket];
    }
    return (profile.feature_snapshot || []).filter((feature) => classifyImplementation(feature.status) === bucket).length;
  }

  function bucketFeatures(profile: AIContextProfile, bucket: string, limit = 8) {
    return (profile.feature_snapshot || [])
      .filter((feature) => classifyImplementation(feature.status) === bucket)
      .slice(0, limit);
  }

  function bucketOverflow(profile: AIContextProfile, bucket: string, limit = 8) {
    return Math.max(0, countByBucket(profile, bucket) - limit);
  }

  function bucketClass(bucket: string) {
    if (bucket === IMPLEMENTED) return 'implemented';
    if (bucket === PARTIAL) return 'partial';
    if (bucket === NOT_IMPLEMENTED) return 'not-implemented';
    return 'unknown';
  }

  function bucketHint(bucket: string) {
    if (bucket === IMPLEMENTED) return '按复用、配置、联调估算';
    if (bucket === PARTIAL) return '补齐缺口并验证完成度';
    if (bucket === NOT_IMPLEMENTED) return '按新增设计与实现估算';
    return '进入 missing_info 人工确认';
  }

  function sourceLabel(profile: AIContextProfile) {
    if (!profile.source_filename) return '配置同步';
    return `${profile.source_filename}${profile.source_sheet ? ` · ${profile.source_sheet}` : ''}`;
  }

  function fileSizeLabel(file: File | null) {
    if (!file) return '支持 CSV / XLSX，字段只需包含功能名称与实现状态';
    return `${Math.max(1, Math.round(file.size / 1024))} KB`;
  }

  function diffItems(items: string[]) {
    if (!items || items.length === 0) return '无';
    return items.slice(0, 5).join(' / ') + (items.length > 5 ? ` / +${items.length - 5}` : '');
  }
</script>

<section class="context-center">
  <div class="center-header">
    <div>
      <span class="center-kicker">Implementation Registry</span>
      <h4>AI 上下文配置中心</h4>
      <p>这里沉淀的是模块能力是否已实现，供 AI 解构时校准复用成本与新增成本；文件只是导入方式，长期可由配置库、接口或人工编辑同步。</p>
    </div>
    <div class="center-stats font-mono">
      <span>{profiles.length} versions</span>
      <span>{enabledCount} active</span>
    </div>
  </div>

  <div class="intake-panel">
    <div class="intake-copy">
      <span class="panel-kicker">Context intake</span>
      <strong>导入能力实现状态</strong>
      <p>只识别“功能名称 + 实现状态”的核心口径，类型、场景、备注只保留为原始快照，不进入估算主判断。</p>
    </div>

    <div class="intake-controls">
      <label class="module-field">
        <span>模块名称</span>
        <input bind:value={moduleName} placeholder="例如：路径规划模块" />
      </label>

      <label class="file-field">
        <input type="file" accept=".xlsx,.csv" on:change={handleFileChange} />
        <span class="file-title">{selectedFile ? selectedFile.name : '选择能力状态清单'}</span>
        <span class="file-meta">{fileSizeLabel(selectedFile)}</span>
      </label>

      <div class="upload-action">
        <Button variant="primary" loading={importing} on:click={importProfile}>
          更新画像
        </Button>
      </div>
    </div>
  </div>

  {#if error}
    <div class="message error font-mono">{error}</div>
  {/if}
  {#if success}
    <div class="message success font-mono">{success}</div>
  {/if}

  <div class="center-body">
    <aside class="profile-list">
      {#if loading}
        <div class="empty-state">加载模块能力画像...</div>
      {:else if profiles.length === 0}
        <div class="empty-state">暂无模块画像</div>
      {:else}
        {#each profiles as profile}
          <div class="profile-row {selectedProfile?.id === profile.id ? 'active' : ''}">
            <button type="button" class="profile-select" on:click={() => selectedProfileID = profile.id}>
              <span class="profile-title">{profile.module_name} v{profile.version}</span>
              <span class="profile-source font-mono">{countByBucket(profile, IMPLEMENTED)}/{profile.feature_count} 已实现 · {formatDate(profile.updated_at)}</span>
              <span class="profile-summary">{profile.summary}</span>
            </button>
            <div class="profile-side">
              <span class="state-pill {profile.enabled ? 'enabled' : ''}">{profile.enabled ? '启用' : '停用'}</span>
              <button
                type="button"
                class="toggle-btn"
                disabled={togglingID === profile.id}
                on:click={() => toggleProfile(profile)}
              >
                {profile.enabled ? '停用' : '启用'}
              </button>
            </div>
          </div>
        {/each}
      {/if}
    </aside>

    <div class="profile-detail">
      {#if selectedProfile}
        <div class="detail-top">
          <div>
            <h5>{selectedProfile.module_name} v{selectedProfile.version}</h5>
            <p>{selectedProfile.summary}</p>
          </div>
          <span class="state-pill {selectedProfile.enabled ? 'enabled' : ''}">{selectedProfile.enabled ? 'Prompt 生效中' : '未注入 Prompt'}</span>
        </div>

        <div class="source-strip">
          <span>来源记录</span>
          <strong class="font-mono">{sourceLabel(selectedProfile)}</strong>
          <span>最近更新</span>
          <strong>{formatDate(selectedProfile.updated_at)}</strong>
        </div>

        <div class="metric-strip">
          {#each implementationBuckets as bucket}
            <div class="metric-card {bucketClass(bucket)}">
              <span>{bucket}</span>
              <strong>{countByBucket(selectedProfile, bucket)}</strong>
              <small>{bucketHint(bucket)}</small>
            </div>
          {/each}
        </div>

        <div class="capability-lanes">
          {#each implementationBuckets as bucket}
            <section class="capability-lane {bucketClass(bucket)}">
              <div class="lane-title">
                <span>{bucket}</span>
                <strong>{countByBucket(selectedProfile, bucket)}</strong>
              </div>
              <ul class="lane-list">
                {#if bucketFeatures(selectedProfile, bucket).length === 0}
                  <li class="lane-empty">无记录</li>
                {:else}
                  {#each bucketFeatures(selectedProfile, bucket) as feature}
                    <li>
                      <span>{feature.feature}</span>
                      <em>{feature.status || bucket}</em>
                    </li>
                  {/each}
                  {#if bucketOverflow(selectedProfile, bucket) > 0}
                    <li class="lane-overflow">还有 {bucketOverflow(selectedProfile, bucket)} 项</li>
                  {/if}
                {/if}
              </ul>
            </section>
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
              <span>实现状态差异</span>
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
                <b>状态变化 {selectedDiff.changed_count}</b>
                <p>{diffItems(selectedDiff.changed)}</p>
              </div>
            </div>
          </div>
        {/if}
      {:else}
        <div class="empty-state">选择或导入一个模块画像后查看实现状态</div>
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
  .diff-title,
  .lane-title {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 12px;
  }

  .center-kicker,
  .panel-kicker {
    color: #38bdf8;
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

  .center-header p,
  .intake-copy p,
  .detail-top p {
    margin: 5px 0 0 0;
    color: #94a3b8;
    font-size: 0.75rem;
    line-height: 1.5;
  }

  .center-stats {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 8px;
  }

  .center-stats span {
    border: 1px solid rgba(51, 65, 85, 0.5);
    background: rgba(15, 23, 42, 0.54);
    border-radius: 4px;
    color: #94a3b8;
    font-size: 0.68rem;
    padding: 4px 7px;
  }

  .intake-panel {
    display: grid;
    grid-template-columns: minmax(220px, 0.8fr) minmax(0, 1.2fr);
    gap: 12px;
    align-items: stretch;
    border: 1px solid rgba(51, 65, 85, 0.44);
    border-radius: 8px;
    background: rgba(15, 23, 42, 0.3);
    padding: 12px;
  }

  .intake-copy {
    min-width: 0;
  }

  .intake-copy strong {
    display: block;
    margin-top: 4px;
    color: #e2e8f0;
    font-size: 0.9rem;
  }

  .intake-controls {
    display: grid;
    grid-template-columns: minmax(160px, 0.85fr) minmax(220px, 1.15fr) auto;
    gap: 10px;
    align-items: end;
  }

  .module-field,
  .file-field {
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-width: 0;
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
    border-color: rgba(56, 189, 248, 0.72);
    box-shadow: 0 0 0 3px rgba(56, 189, 248, 0.1);
  }

  .file-field {
    min-height: 38px;
    justify-content: center;
    box-sizing: border-box;
    padding: 8px 10px;
    border: 1px dashed rgba(56, 189, 248, 0.42);
    border-radius: 6px;
    background: rgba(2, 6, 23, 0.34);
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
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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
    grid-template-columns: minmax(250px, 0.72fr) minmax(0, 1.28fr);
    gap: 12px;
    min-height: 320px;
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
    max-height: 520px;
    overflow-y: auto;
  }

  .profile-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 10px;
    align-items: stretch;
    padding: 10px;
    border: 1px solid rgba(51, 65, 85, 0.45);
    border-radius: 7px;
    background: rgba(2, 6, 23, 0.34);
    box-sizing: border-box;
  }

  .profile-row.active {
    border-color: rgba(56, 189, 248, 0.56);
    background: rgba(8, 47, 73, 0.18);
  }

  .profile-select {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 5px;
    padding: 0;
    border: 0;
    background: transparent;
    color: inherit;
    text-align: left;
    cursor: pointer;
  }

  .profile-select:focus-visible,
  .toggle-btn:focus-visible {
    outline: 2px solid rgba(56, 189, 248, 0.66);
    outline-offset: 2px;
  }

  .profile-title {
    color: #e2e8f0;
    font-size: 0.82rem;
    font-weight: 800;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .profile-source,
  .profile-summary {
    color: #64748b;
    font-size: 0.66rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .profile-summary {
    white-space: normal;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
  }

  .profile-side {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    justify-content: space-between;
    gap: 8px;
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
    border-color: rgba(56, 189, 248, 0.5);
    color: #e0f2fe;
  }

  .profile-detail {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 12px;
  }

  .source-strip {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto minmax(0, 0.8fr);
    gap: 7px 10px;
    align-items: center;
    padding: 9px 10px;
    border: 1px solid rgba(51, 65, 85, 0.38);
    border-radius: 7px;
    background: rgba(2, 6, 23, 0.24);
  }

  .source-strip span {
    color: #64748b;
    font-size: 0.68rem;
    font-weight: 800;
  }

  .source-strip strong {
    min-width: 0;
    color: #cbd5e1;
    font-size: 0.7rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .metric-strip {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 8px;
  }

  .metric-card {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.44);
    border-radius: 7px;
    background: rgba(2, 6, 23, 0.28);
    padding: 10px;
  }

  .metric-card span {
    color: #94a3b8;
    font-size: 0.7rem;
    font-weight: 800;
  }

  .metric-card strong {
    display: block;
    margin-top: 4px;
    color: #e2e8f0;
    font-size: 1.25rem;
  }

  .metric-card small {
    display: block;
    margin-top: 3px;
    color: #64748b;
    font-size: 0.66rem;
    line-height: 1.35;
  }

  .metric-card.implemented {
    border-color: rgba(52, 211, 153, 0.25);
  }

  .metric-card.partial {
    border-color: rgba(56, 189, 248, 0.25);
  }

  .metric-card.not-implemented {
    border-color: rgba(251, 191, 36, 0.26);
  }

  .metric-card.unknown {
    border-color: rgba(248, 113, 113, 0.22);
  }

  .capability-lanes {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }

  .capability-lane,
  .prompt-preview,
  .diff-panel {
    min-width: 0;
    border: 1px solid rgba(51, 65, 85, 0.42);
    border-radius: 8px;
    background: rgba(2, 6, 23, 0.28);
  }

  .capability-lane {
    padding: 10px;
  }

  .lane-title span {
    color: #cbd5e1;
    font-size: 0.75rem;
    font-weight: 800;
  }

  .lane-title strong {
    color: #94a3b8;
    font-size: 0.75rem;
  }

  .lane-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin: 9px 0 0 0;
    padding: 0;
    max-height: 156px;
    overflow-y: auto;
    list-style: none;
  }

  .lane-list li {
    min-width: 0;
    display: flex;
    justify-content: space-between;
    gap: 8px;
    padding: 6px 7px;
    border-radius: 5px;
    background: rgba(15, 23, 42, 0.48);
  }

  .lane-list span {
    min-width: 0;
    color: #cbd5e1;
    font-size: 0.7rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .lane-list em {
    flex: none;
    color: #64748b;
    font-size: 0.66rem;
    font-style: normal;
  }

  .lane-list .lane-empty,
  .lane-list .lane-overflow {
    justify-content: center;
    color: #64748b;
    font-size: 0.7rem;
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
  .lane-list,
  .prompt-preview pre {
    scrollbar-width: thin;
    scrollbar-color: rgba(56, 189, 248, 0.5) rgba(15, 23, 42, 0.72);
  }

  .profile-list::-webkit-scrollbar,
  .lane-list::-webkit-scrollbar,
  .prompt-preview pre::-webkit-scrollbar {
    width: 8px;
    height: 8px;
  }

  .profile-list::-webkit-scrollbar-track,
  .lane-list::-webkit-scrollbar-track,
  .prompt-preview pre::-webkit-scrollbar-track {
    background: rgba(2, 6, 23, 0.42);
    border-radius: 999px;
  }

  .profile-list::-webkit-scrollbar-thumb,
  .lane-list::-webkit-scrollbar-thumb,
  .prompt-preview pre::-webkit-scrollbar-thumb {
    background: linear-gradient(180deg, rgba(56, 189, 248, 0.62), rgba(52, 211, 153, 0.34));
    border: 2px solid rgba(2, 6, 23, 0.42);
    border-radius: 999px;
  }

  .font-mono {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  @media (max-width: 1080px) {
    .intake-panel,
    .intake-controls,
    .center-body,
    .source-strip,
    .metric-strip,
    .capability-lanes,
    .diff-grid {
      grid-template-columns: minmax(0, 1fr);
    }

    .upload-action {
      justify-content: flex-start;
    }
  }
</style>
