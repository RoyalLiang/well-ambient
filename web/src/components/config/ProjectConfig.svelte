<script lang="ts">
  import { onMount } from 'svelte';
  import Button from '../shared/Button.svelte';
  import TextInput from '../shared/TextInput.svelte';
  import Alert from '../shared/Alert.svelte';

  export let lastUpdated = '';

  interface ProjectConfig {
    id?: number;
    project_name: string;
    project_key: string;
    git_repos_json: string;
    base_priority: string; // P0, P1, P2
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
  let formBasePriority = 'P1';
  let formGitReposStr = '';

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
    formBasePriority = 'P1';
    formGitReposStr = '';
    errorMsg = '';
    successMsg = '';
  }

  function startEdit(project: ProjectConfig) {
    isEditing = true;
    editingProject = project;
    formProjectName = project.project_name;
    formProjectKey = project.project_key;
    formBasePriority = project.base_priority || 'P1';
    
    let repos: string[] = [];
    try {
      repos = JSON.parse(project.git_repos_json || '[]');
    } catch (_) {}
    formGitReposStr = repos.join(', ');
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

    const repos = formGitReposStr
      .split(',')
      .map(r => r.trim())
      .filter(r => r.length > 0);

    const payload = {
      project_key: formProjectKey.trim().toUpperCase(),
      project_name: formProjectName.trim(),
      base_priority: formBasePriority,
      git_repos_json: JSON.stringify(repos)
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

  function formatUpdated(value: string) {
    if (!value) return '暂无更新记录';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString();
  }

  function parseRepos(jsonStr: string): string[] {
    try {
      return JSON.parse(jsonStr || '[]');
    } catch (_) {
      return [];
    }
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
        <p>配置 Jira 项目键（Project Key）所关联的 Git 多仓路径，并设定项目的全局基准优先级（P0/P1/P2）。最强大脑将据此高亮和置顶相关任务。</p>
      </div>
      <Button variant="primary" on:click={startCreate}>➕ 新建项目集成</Button>
    </div>

    {#if loading && projects.length === 0}
      <div class="loading-state font-mono">正在加载项目集成列表...</div>
    {:else if projects.length === 0}
      <div class="empty-state font-mono">
        <p>暂无自定义项目集成。当 Jira 数据同步时，系统会自动在此生成默认项目卡片。</p>
      </div>
    {:else}
      <div class="table-responsive">
        <table>
          <thead>
            <tr>
              <th>项目键 (Key)</th>
              <th>项目名称</th>
              <th>优先级</th>
              <th>Git 关联多仓</th>
              <th style="text-align: right;">操作</th>
            </tr>
          </thead>
          <tbody>
            {#each projects as project}
              <tr>
                <td class="font-mono text-bold highlight-key">{project.project_key}</td>
                <td>{project.project_name}</td>
                <td>
                  <span class="priority-badge p-{project.base_priority.toLowerCase()}">
                    {project.base_priority}
                  </span>
                </td>
                <td>
                  <div class="repos-wrapper">
                    {#each parseRepos(project.git_repos_json) as repo}
                      <span class="repo-tag font-mono">{repo}</span>
                    {:else}
                      <span class="text-muted text-xs">⚠️ 暂无关联仓库</span>
                    {/each}
                  </div>
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
      <p>为项目设定专用的 Jira Project Key，关联多仓，并定义基准优先级。</p>
    </div>

    <div class="form-container">
      <TextInput
        id="project-key"
        label="Jira 项目键 (Project Key)"
        placeholder="例如: HIT, CRM"
        bind:value={formProjectKey}
        disabled={!!editingProject}
        required={true}
        helperText="Jira 项目的关键简称前缀，例如任务ID为 'HIT-101' 则项目键为 'HIT'。保存后不可更改。"
      />

      <TextInput
        id="project-name"
        label="项目名称"
        placeholder="例如: 最强大脑项目"
        bind:value={formProjectName}
        required={true}
        helperText="展示在大脑项目大盘与诊断报表中的项目中文名。"
      />

      <div class="form-group">
        <label class="form-label" for="project-priority">项目优先级 (Base Priority)</label>
        <select id="project-priority" class="select-input" bind:value={formBasePriority}>
          <option value="P0">P0 - 阻断高优（任务强制置顶，并渲染红色发光）</option>
          <option value="P1">P1 - 普通交付（默认，黄色发光）</option>
          <option value="P2">P2 - 持续优化（低优先，蓝色发光）</option>
        </select>
        <span class="helper-text">不同优先级会直接影响协同大盘排期与需求列表中，最强大脑打分计算的加权置顶排布优先级。</span>
      </div>

      <TextInput
        id="project-repos"
        label="关联 Git 仓库 (Git Repos)"
        placeholder="例如: group/repo1, group/repo2"
        bind:value={formGitReposStr}
        helperText="配置项目关联的代码多仓路径，支持多个，用英文逗号分隔。最强大脑将据此核对提交绑定与单测遥测状态。"
      />

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

  /* Badges & tags */
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
    background: rgba(245, 158, 11, 0.15);
    color: #fbbf24;
    border: 1px solid rgba(245, 158, 11, 0.3);
    box-shadow: 0 0 6px rgba(245, 158, 11, 0.1);
  }

  .priority-badge.p-p2 {
    background: rgba(59, 130, 246, 0.15);
    color: #60a5fa;
    border: 1px solid rgba(59, 130, 246, 0.3);
  }

  .repos-wrapper {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .repo-tag {
    background: rgba(30, 41, 59, 0.6);
    border: 1px solid rgba(51, 65, 85, 0.5);
    border-radius: 4px;
    padding: 2px 6px;
    font-size: 0.75rem;
    color: #cbd5e1;
  }

  .text-muted {
    color: #64748b;
  }

  .text-xs {
    font-size: 0.75rem;
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

  .helper-text {
    font-size: 0.75rem;
    color: #64748b;
    line-height: 1.4;
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
