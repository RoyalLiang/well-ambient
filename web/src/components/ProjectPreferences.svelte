<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import MultiSelect from './shared/MultiSelect.svelte';

  type PreferenceMode = 'all' | 'selected';

  interface ProjectOption {
    project_key: string;
    project_name: string;
  }

  interface ProjectPreferenceResponse {
    mode: PreferenceMode;
    project_keys: string[];
    projects: ProjectOption[];
  }

  const dispatch = createEventDispatcher<{ saved: ProjectPreferenceResponse }>();

  let loading = true;
  let saving = false;
  let editing = false;
  let errorMessage = '';
  let successMessage = '';
  let mode: PreferenceMode = 'all';
  let selectedProjectKeys: string[] = [];
  let savedMode: PreferenceMode = 'all';
  let savedProjectKeys: string[] = [];
  let projects: ProjectOption[] = [];

  $: projectOptions = projects.map((project) => ({
    value: project.project_key,
    label: project.project_name,
    meta: project.project_key
  }));
  $: selectedNames = savedProjectKeys.map((key) => {
    const project = projects.find((item) => item.project_key === key);
    return project?.project_name || key;
  });
  $: selectionSummary = savedMode === 'all'
    ? '全部项目'
    : selectedNames.length > 0
      ? selectedNames.join('、')
      : '尚未选择项目';
  $: canSave = !saving && (mode === 'all' || selectedProjectKeys.length > 0);

  async function loadPreferences() {
    loading = true;
    errorMessage = '';
    try {
      const response = await fetch('/api/me/project-preferences');
      if (!response.ok) throw new Error(await response.text() || '读取失败');
      applyResponse(await response.json());
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : '暂时无法读取项目偏好';
    } finally {
      loading = false;
    }
  }

  function applyResponse(response: ProjectPreferenceResponse) {
    projects = Array.isArray(response.projects) ? response.projects : [];
    savedMode = response.mode === 'selected' ? 'selected' : 'all';
    savedProjectKeys = Array.isArray(response.project_keys) ? [...response.project_keys] : [];
    mode = savedMode;
    selectedProjectKeys = [...savedProjectKeys];
  }

  function beginEditing() {
    mode = savedMode;
    selectedProjectKeys = [...savedProjectKeys];
    errorMessage = '';
    successMessage = '';
    editing = true;
  }

  function cancelEditing() {
    mode = savedMode;
    selectedProjectKeys = [...savedProjectKeys];
    errorMessage = '';
    editing = false;
  }

  async function savePreferences() {
    if (!canSave) return;
    saving = true;
    errorMessage = '';
    successMessage = '';
    try {
      const response = await fetch('/api/me/project-preferences', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ project_keys: mode === 'all' ? [] : selectedProjectKeys })
      });
      if (!response.ok) throw new Error(await response.text() || '保存失败');
      const payload: ProjectPreferenceResponse = await response.json();
      applyResponse(payload);
      editing = false;
      successMessage = '项目偏好已更新，当前页面数据已按新范围刷新。';
      dispatch('saved', payload);
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : '项目偏好保存失败';
    } finally {
      saving = false;
    }
  }

  onMount(() => {
    void loadPreferences();
  });
</script>

<section class="preference-section" aria-labelledby="project-preference-title">
  <div class="section-heading">
    <div>
      <h3 id="project-preference-title">项目偏好</h3>
      <p>控制需求、Jira 与任务视图中的项目范围。</p>
    </div>
    {#if !loading && !editing}
      <button type="button" class="edit-button" on:click={beginEditing}>设置</button>
    {/if}
  </div>

  {#if loading}
    <div class="loading-row" aria-live="polite"><span></span>正在读取偏好</div>
  {:else if editing}
    <div class="preference-form">
      <div class="mode-options" role="radiogroup" aria-label="项目查看范围">
        <button
          type="button"
          class:active={mode === 'all'}
          role="radio"
          aria-checked={mode === 'all'}
          on:click={() => mode = 'all'}
        >
          <span class="radio-dot" aria-hidden="true"></span>
          <span><strong>全部项目</strong><small>包含后续新增项目</small></span>
        </button>
        <button
          type="button"
          class:active={mode === 'selected'}
          role="radio"
          aria-checked={mode === 'selected'}
          on:click={() => mode = 'selected'}
        >
          <span class="radio-dot" aria-hidden="true"></span>
          <span><strong>指定项目</strong><small>仅查看选择的项目</small></span>
        </button>
      </div>

      {#if mode === 'selected'}
        <MultiSelect
          id="profile-project-preferences"
          label="可见项目"
          options={projectOptions}
          bind:values={selectedProjectKeys}
          placeholder="选择一个或多个项目"
          searchPlaceholder="搜索项目"
          emptyText="没有可选项目"
          helperText="至少选择一个项目；页面原有筛选会继续在此范围内生效。"
          compact={true}
          required={true}
        />
      {/if}

      {#if errorMessage}<p class="feedback error" role="alert">{errorMessage}</p>{/if}
      {#if mode === 'selected' && selectedProjectKeys.length === 0}
        <p class="selection-warning">请选择至少一个项目后再保存。</p>
      {/if}

      <div class="form-actions">
        <button type="button" class="secondary-button" disabled={saving} on:click={cancelEditing}>取消</button>
        <button type="button" class="save-button" disabled={!canSave} on:click={savePreferences}>
          {saving ? '保存中…' : '保存偏好'}
        </button>
      </div>
    </div>
  {:else}
    <div class="preference-summary">
      <span class:limited={savedMode === 'selected'}>{savedMode === 'all' ? '全部' : `${savedProjectKeys.length} 个`}</span>
      <p title={selectionSummary}>{selectionSummary}</p>
    </div>
    {#if errorMessage}<p class="feedback error" role="alert">{errorMessage}</p>{/if}
    {#if successMessage}<p class="feedback success" aria-live="polite">{successMessage}</p>{/if}
  {/if}
</section>

<style>
  .preference-section {
    padding: 14px 0;
    border-top: 1px solid rgba(121, 139, 159, 0.14);
    border-bottom: 1px solid rgba(121, 139, 159, 0.14);
  }

  .section-heading,
  .form-actions,
  .preference-summary,
  .loading-row {
    display: flex;
    align-items: center;
  }

  .section-heading {
    justify-content: space-between;
    gap: 14px;
  }

  h3,
  p {
    margin: 0;
  }

  h3 {
    color: var(--wa-text-strong, #0d1722);
    font-size: 0.82rem;
    font-weight: 780;
  }

  .section-heading p {
    margin-top: 3px;
    color: var(--wa-text-muted, #667789);
    font-size: 0.69rem;
    line-height: 1.45;
  }

  button {
    font-family: inherit;
  }

  .edit-button {
    min-width: 52px;
    min-height: 32px;
    border: 1px solid rgba(23, 111, 102, 0.22);
    border-radius: 8px;
    background: #edf7f5;
    color: #176f66;
    font-size: 0.72rem;
    font-weight: 760;
    cursor: pointer;
  }

  .preference-summary {
    gap: 9px;
    margin-top: 10px;
    min-width: 0;
  }

  .preference-summary > span {
    flex: 0 0 auto;
    border-radius: 999px;
    padding: 4px 8px;
    background: #edf7f5;
    color: #176f66;
    font-size: 0.66rem;
    font-weight: 780;
  }

  .preference-summary > span.limited {
    background: #eef3fb;
    color: #3c6293;
  }

  .preference-summary p {
    min-width: 0;
    overflow: hidden;
    color: var(--wa-text-main, #293847);
    font-size: 0.75rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .preference-form {
    display: grid;
    gap: 12px;
    margin-top: 12px;
  }

  .mode-options {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 7px;
  }

  .mode-options > button {
    min-width: 0;
    min-height: 56px;
    display: grid;
    grid-template-columns: 16px minmax(0, 1fr);
    align-items: center;
    gap: 7px;
    border: 1px solid rgba(121, 139, 159, 0.18);
    border-radius: 9px;
    padding: 8px;
    background: rgba(247, 250, 252, 0.82);
    color: var(--wa-text-main, #293847);
    text-align: left;
    cursor: pointer;
  }

  .mode-options > button.active {
    border-color: #8fc5bd;
    background: #edf7f5;
  }

  .mode-options strong,
  .mode-options small {
    display: block;
  }

  .mode-options strong {
    font-size: 0.72rem;
  }

  .mode-options small {
    margin-top: 2px;
    color: var(--wa-text-muted, #667789);
    font-size: 0.61rem;
    line-height: 1.3;
  }

  .radio-dot {
    width: 14px;
    height: 14px;
    border: 1px solid #aab8c1;
    border-radius: 50%;
    background: #ffffff;
    box-shadow: inset 0 0 0 3px #ffffff;
  }

  .active .radio-dot {
    border-color: #268b7f;
    background: #268b7f;
  }

  .form-actions {
    justify-content: flex-end;
    gap: 8px;
  }

  .form-actions button {
    min-height: 34px;
    border-radius: 8px;
    padding: 0 12px;
    font-size: 0.72rem;
    font-weight: 760;
    cursor: pointer;
  }

  .secondary-button {
    border: 1px solid rgba(121, 139, 159, 0.2);
    background: #ffffff;
    color: var(--wa-text-main, #293847);
  }

  .save-button {
    border: 1px solid #176f66;
    background: #176f66;
    color: #ffffff;
  }

  button:disabled {
    opacity: 0.46;
    cursor: not-allowed;
  }

  button:focus-visible {
    outline: 3px solid rgba(38, 139, 127, 0.18);
    outline-offset: 2px;
  }

  .feedback,
  .selection-warning {
    font-size: 0.68rem;
    line-height: 1.45;
  }

  .feedback {
    margin-top: 9px;
  }

  .feedback.error,
  .selection-warning {
    color: var(--wa-danger, #b84238);
  }

  .feedback.success {
    color: #176f66;
  }

  .loading-row {
    gap: 8px;
    min-height: 36px;
    margin-top: 8px;
    color: var(--wa-text-muted, #667789);
    font-size: 0.72rem;
  }

  .loading-row span {
    width: 13px;
    height: 13px;
    border: 2px solid #c9d6db;
    border-top-color: #268b7f;
    border-radius: 50%;
    animation: spin 700ms linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  @media (max-width: 560px) {
    .edit-button,
    .form-actions button {
      min-height: 44px;
    }

    .mode-options {
      grid-template-columns: 1fr;
    }

    .mode-options > button {
      min-height: 54px;
    }
  }
</style>
