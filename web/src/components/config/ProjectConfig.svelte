<script lang="ts">
  import { onMount } from 'svelte';
  import Button from '../shared/Button.svelte';
  import TextInput from '../shared/TextInput.svelte';
  import Alert from '../shared/Alert.svelte';

  export let lastUpdated = '';
  export let syncProjects: string[] = [];

  interface ProjectConfig {
    id?: number;
    project_name: string;
    project_key: string;
    git_repos_json?: string;
    base_priority: string; // P0 - P5
    project_phase?: string; // POC, 交付, 运营, 售后
  }

  let projects: ProjectConfig[] = [];
  let loading = false;
  let errorMsg = '';
  let successMsg = '';
  let saving = false;

  // Form states
  let isEditing = false;
  let editingProject: ProjectConfig | null = null;
  let formProjectName = '';
  let formProjectKey = '';
  let formBasePriority = 'P2'; // Default to P2
  let formProjectPhase = '交付'; // Default to 交付

  // Dropdown controls
  let showKeyDropdown = false;

  $: availableKeys = syncProjects.filter(key => {
    return !projects.some(p => p.project_key.toUpperCase() === key.toUpperCase());
  });

  $: filteredKeys = availableKeys.filter(key => 
    key.toLowerCase().includes(formProjectKey.toLowerCase())
  );

  onMount(async () => {
    await fetchProjects();
  });

  async function fetchProjects() {
    loading = true;
    errorMsg = '';
    const token = localStorage.getItem('jwt_token');
    try {
      const res = await fetch('/api/projects/config', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (!res.ok) throw new Error('获取项目配置失败');
      projects = await res.json();
    } catch (err: any) {
      errorMsg = err.message || '加载项目配置失败';
    } finally {
      loading = false;
    }
  }

  function startCreate() {
    isEditing = true;
    editingProject = null;
    formProjectName = '';
    formProjectKey = '';
    formBasePriority = 'P2';
    formProjectPhase = '交付';
    errorMsg = '';
    successMsg = '';
  }

  function startEdit(project: ProjectConfig) {
    isEditing = true;
    editingProject = project;
    formProjectName = project.project_name;
    formProjectKey = project.project_key;
    formBasePriority = project.base_priority || 'P2';
    formProjectPhase = project.project_phase || '交付';
    errorMsg = '';
    successMsg = '';
  }

  async function saveProject() {
    if (!formProjectKey.trim() || !formProjectName.trim()) {
      errorMsg = '项目键与项目名称为必选项';
      return;
    }

    saving = true;
    errorMsg = '';
    successMsg = '';
    const token = localStorage.getItem('jwt_token');

    const payload = {
      project_key: formProjectKey.trim().toUpperCase(),
      project_name: formProjectName.trim(),
      base_priority: formBasePriority,
      project_phase: formProjectPhase,
      git_repos_json: "[]"
    };

    try {
      const res = await fetch('/api/projects/config', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify(payload)
      });

      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || '保存失败');
      }

      successMsg = editingProject ? '修改项目配置成功' : '新增项目配置成功';
      isEditing = false;
      editingProject = null;
      await fetchProjects();

      if (typeof window !== 'undefined') {
        window.dispatchEvent(new CustomEvent('project-config-updated'));
      }
    } catch (err: any) {
      errorMsg = err.message || '保存项目配置发生错误';
    } finally {
      saving = false;
    }
  }

  function cancelEdit() {
    isEditing = false;
    editingProject = null;
    errorMsg = '';
  }
</script>

<div class="project-config-container">
  {#if errorMsg}
    <Alert type="error" message={errorMsg} closable={true} on:close={() => errorMsg = ''} />
  {/if}
  {#if successMsg}
    <Alert type="success" message={successMsg} closable={true} on:close={() => successMsg = ''} />
  {/if}

  {#if !isEditing}
    <div class="card-header flex-header">
      <div>
        <h2>📝 项目集成与优先级管理</h2>
        <p>配置并同步 Jira 项目键（Project Key）的全局基准优先级（P0 - P5）与当前项目运作阶段。最强大脑将自动执行权重置顶和健康监控。</p>
      </div>
      <Button variant="primary" on:click={startCreate}>➕ 新建项目集成</Button>
    </div>

    {#if loading && projects.length === 0}
      <div class="loading-state font-mono">正在加载项目集成列表...</div>
    {:else if projects.length === 0}
      <div class="empty-state font-mono">
        <p>暂无自定义项目集成。新建项目以设定基准属性。</p>
      </div>
    {:else}
      <div class="table-responsive">
        <table>
          <thead>
            <tr>
              <th>项目键 (Key)</th>
              <th>项目名称</th>
              <th>运作阶段</th>
              <th>优先级</th>
              <th style="text-align: right;">操作</th>
            </tr>
          </thead>
          <tbody>
            {#each projects as project}
              <tr>
                <td class="font-mono text-bold highlight-key">{project.project_key}</td>
                <td>{project.project_name}</td>
                <td>
                  <span class="phase-badge phase-{project.project_phase || '交付'}">
                    {project.project_phase || '交付'}
                  </span>
                </td>
                <td>
                  <span class="priority-badge p-{project.base_priority.toLowerCase()}">
                    {project.base_priority}
                  </span>
                </td>
                <td style="text-align: right;">
                  <Button size="small" variant="ghost" on:click={() => startEdit(project)}>编辑</Button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {:else}
    <div class="card-header">
      <h2>{editingProject ? '编辑项目配置' : '新建项目配置'}</h2>
      <p>为项目绑定专用的 Jira Project Key，定义优先级权重与当前项目运行周期阶段。</p>
    </div>

    <div class="form-container">
      <div class="custom-select-wrapper">
        {#if editingProject}
          <TextInput
            id="project-key"
            label="Jira 项目键 (Project Key)"
            bind:value={formProjectKey}
            disabled={true}
            required={true}
            helperText="Jira 项目键保存后不可更改。"
          />
        {:else}
          <TextInput
            id="project-key"
            label="Jira 项目键 (Project Key)"
            placeholder="搜索或选择 Jira 中同步的项目键 (如: HIT)"
            bind:value={formProjectKey}
            required={true}
            on:focus={() => showKeyDropdown = true}
            on:blur={() => setTimeout(() => showKeyDropdown = false, 200)}
            helperText="已被其他项目集成配置占用的 Key 将不再列出。支持拼音与英文字符过滤。"
          />
          {#if showKeyDropdown}
            <div class="custom-select-dropdown">
              {#each filteredKeys as key}
                <button type="button" class="dropdown-item" on:click={() => { formProjectKey = key; showKeyDropdown = false; }}>
                  {key}
                </button>
              {:else}
                <div class="dropdown-empty">没有匹配的待配置项目键</div>
              {/each}
            </div>
          {/if}
        {/if}
      </div>

      <TextInput
        id="project-name"
        label="项目名称"
        placeholder=""
        bind:value={formProjectName}
        required={true}
        helperText="展示在大脑项目大盘与诊断报表中的项目中文名。"
      />

      <div class="form-group-row">
        <div class="form-group flex-1">
          <label class="form-label" for="project-phase">项目阶段 (Project Phase)</label>
          <select id="project-phase" class="select-input" bind:value={formProjectPhase}>
            <option value="POC">POC</option>
            <option value="交付">交付</option>
            <option value="运营">运营</option>
            <option value="售后">售后</option>
          </select>
        </div>

        <div class="form-group flex-1">
          <label class="form-label" for="project-priority">项目优先级 (Base Priority)</label>
          <select id="project-priority" class="select-input" bind:value={formBasePriority}>
            <option value="P0">P0 - 阻断高优</option>
            <option value="P1">P1 - 核心交付</option>
            <option value="P2">P2 - 持续优化</option>
            <option value="P3">P3 - 关注保障</option>
            <option value="P4">P4 - 日常维护</option>
            <option value="P5">P5 - 辅助支持</option>
          </select>
        </div>
      </div>

      <div class="form-actions">
        <Button variant="secondary" disabled={saving} on:click={cancelEdit}>取消</Button>
        <Button variant="primary" loading={saving} on:click={saveProject}>保存配置</Button>
      </div>
    </div>
  {/if}
</div>

<style>
  .project-config-container {
    animation: fadeIn 0.4s ease-out;
  }

  .flex-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    flex-wrap: wrap;
    margin-bottom: 20px;
  }

  .card-header h2 {
    font-size: 1.25rem;
    font-weight: 700;
    margin: 0 0 6px 0;
    background: linear-gradient(135deg, #ffffff 0%, #cbd5e1 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  .card-header p {
    font-size: 0.85rem;
    color: #64748b;
    margin: 0;
    line-height: 1.5;
  }

  .loading-state, .empty-state {
    padding: 32px;
    text-align: center;
    border: 1px dashed rgba(51, 65, 85, 0.4);
    border-radius: 8px;
    color: #64748b;
    background: rgba(15, 23, 42, 0.2);
  }

  /* Table styling */
  .table-responsive {
    overflow-x: auto;
    border-radius: 8px;
    border: 1px solid rgba(51, 65, 85, 0.4);
  }

  table {
    width: 100%;
    border-collapse: collapse;
    text-align: left;
  }

  th {
    background: #0b1329;
    color: #94a3b8;
    font-size: 0.8rem;
    font-weight: 700;
    padding: 12px 16px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.6);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  td {
    padding: 14px 16px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.4);
    font-size: 0.9rem;
    color: #e2e8f0;
  }

  tr:last-child td {
    border-bottom: none;
  }

  tr:hover td {
    background: rgba(30, 41, 59, 0.2);
  }

  .highlight-key {
    color: #38bdf8 !important;
    font-weight: 700;
  }

  /* Phase Badge styles */
  .phase-badge {
    display: inline-block;
    padding: 3px 8px;
    border-radius: 4px;
    font-size: 0.75rem;
    font-weight: 700;
  }
  .phase-badge.phase-POC {
    background: rgba(168, 85, 247, 0.15);
    color: #c084fc;
    border: 1px solid rgba(168, 85, 247, 0.3);
  }
  .phase-badge.phase-交付 {
    background: rgba(14, 165, 233, 0.15);
    color: #38bdf8;
    border: 1px solid rgba(14, 165, 233, 0.3);
  }
  .phase-badge.phase-运营 {
    background: rgba(16, 185, 129, 0.15);
    color: #34d399;
    border: 1px solid rgba(16, 185, 129, 0.3);
  }
  .phase-badge.phase-售后 {
    background: rgba(245, 158, 11, 0.15);
    color: #fbbf24;
    border: 1px solid rgba(245, 158, 11, 0.3);
  }

  /* Priority Badges */
  .priority-badge {
    display: inline-block;
    padding: 3px 8px;
    border-radius: 4px;
    font-size: 0.75rem;
    font-weight: 700;
    font-family: monospace;
  }

  .priority-badge.p-p0 {
    background: rgba(239, 68, 68, 0.15);
    color: #f87171;
    border: 1px solid rgba(239, 68, 68, 0.3);
    box-shadow: 0 0 6px rgba(239, 68, 68, 0.2);
  }
  .priority-badge.p-p1 {
    background: rgba(249, 115, 22, 0.15);
    color: #fb923c;
    border: 1px solid rgba(249, 115, 22, 0.3);
  }
  .priority-badge.p-p2 {
    background: rgba(245, 158, 11, 0.15);
    color: #fbbf24;
    border: 1px solid rgba(245, 158, 11, 0.3);
  }
  .priority-badge.p-p3 {
    background: rgba(59, 130, 246, 0.15);
    color: #60a5fa;
    border: 1px solid rgba(59, 130, 246, 0.3);
  }
  .priority-badge.p-p4 {
    background: rgba(99, 102, 241, 0.15);
    color: #818cf8;
    border: 1px solid rgba(99, 102, 241, 0.3);
  }
  .priority-badge.p-p5 {
    background: rgba(148, 163, 184, 0.15);
    color: #94a3b8;
    border: 1px solid rgba(148, 163, 184, 0.3);
  }

  /* Form design */
  .form-container {
    display: flex;
    flex-direction: column;
    gap: 20px;
    background: rgba(15, 23, 42, 0.2);
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 8px;
    padding: 24px;
    margin-top: 16px;
  }

  .form-group-row {
    display: flex;
    gap: 16px;
    width: 100%;
  }

  .flex-1 {
    flex: 1;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .form-label {
    font-size: 0.85rem;
    font-weight: 600;
    color: #cbd5e1;
  }

  .select-input {
    background: #0f172a;
    border: 1px solid rgba(51, 65, 85, 0.8);
    border-radius: 6px;
    color: #f1f5f9;
    padding: 10px 12px;
    font-size: 0.9rem;
    width: 100%;
    outline: none;
    transition: border-color 0.2s, box-shadow 0.2s;
  }

  .select-input:focus {
    border-color: #38bdf8;
    box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.15);
  }

  /* Searchable Select Dropdown */
  .custom-select-wrapper {
    position: relative;
    width: 100%;
  }

  .custom-select-dropdown {
    position: absolute;
    top: calc(100% - 10px);
    left: 0;
    right: 0;
    background: #0b1329;
    border: 1px solid rgba(56, 189, 248, 0.3);
    border-radius: 6px;
    box-shadow: 0 10px 25px rgba(0, 0, 0, 0.6);
    z-index: 1000;
    max-height: 200px;
    overflow-y: auto;
    padding: 6px 0;
  }

  .dropdown-item {
    display: block;
    width: 100%;
    text-align: left;
    background: transparent;
    border: none;
    padding: 8px 16px;
    font-size: 0.85rem;
    color: #e2e8f0;
    cursor: pointer;
    font-family: monospace;
  }

  .dropdown-item:hover {
    background: rgba(56, 189, 248, 0.12);
    color: #38bdf8;
  }

  .dropdown-empty {
    padding: 12px 16px;
    font-size: 0.8rem;
    color: #64748b;
    text-align: center;
  }

  .form-actions {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    margin-top: 12px;
    border-top: 1px solid rgba(51, 65, 85, 0.4);
    padding-top: 16px;
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }
</style>
