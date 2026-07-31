<script lang="ts">
  import { onMount } from 'svelte';
  import Button from '../shared/Button.svelte';
  import TextInput from '../shared/TextInput.svelte';
  import Alert from '../shared/Alert.svelte';
  import Select from '../shared/Select.svelte';
  import { resetSettingsWorkspaceScroll } from '../../lib/settings-ui';
  import { fetchDeliveryDirectory } from '../../lib/delivery-directory';

  const phaseOptions = [
    { value: 'POC', label: 'POC' },
    { value: '交付', label: '交付' },
    { value: '运营', label: '运营' },
    { value: '售后', label: '售后' }
  ];

  const priorityOptions = [
    { value: 'P0', label: 'P0 - 阻断高优' },
    { value: 'P1', label: 'P1 - 核心交付' },
    { value: 'P2', label: 'P2 - 持续优化' },
    { value: 'P3', label: 'P3 - 关注保障' },
    { value: 'P4', label: 'P4 - 日常维护' },
    { value: 'P5', label: 'P5 - 辅助支持' }
  ];

  export let lastUpdated = '';
  export let syncProjects: string[] = [];

  interface ProjectConfig {
    id?: number;
    project_name: string;
    project_key: string;
    git_repos_json?: string;
    base_priority: string; // P0 - P5
    project_phase?: string; // POC, 交付, 运营, 售后
    base_score?: number;
    base_score_weight?: number;
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
  let formBaseScore = '60.0';
  let formBaseScoreWeightPercent = '10';

  let jiraProjectMap: {[key: string]: string} = {};

  $: availableKeys = syncProjects.filter(key => {
    return !projects.some(p => p.project_key.toUpperCase() === key.toUpperCase());
  });

  $: projectKeyOptions = availableKeys.map((key) => {
    const projectName = cleanProjectName(jiraProjectMap[key.toUpperCase()] || '');
    return {
      value: key,
      label: projectName ? `${projectName} (${key})` : key,
      meta: projectName ? key : 'Jira Project Key'
    };
  });

  $: highPriorityCount = projects.filter((project) => ['P0', 'P1'].includes(project.base_priority)).length;
  $: projectStatusTone = loading ? 'checking' : errorMsg ? 'error' : projects.length > 0 ? 'success' : 'unchecked';
  $: projectStatusLabel = loading ? '加载中' : errorMsg ? '加载失败' : projects.length > 0 ? '映射可用' : '尚未配置';
  $: projectStatusMessage = loading
    ? '正在读取项目映射与 Jira 项目名称。'
    : errorMsg
      ? errorMsg
      : projects.length > 0
        ? `已维护 ${projects.length} 个项目映射，其中 ${highPriorityCount} 个为 P0/P1。`
        : '尚未创建项目映射。新增项目后，优先级与阶段会用于排期和健康度计算。';

  onMount(async () => {
    await Promise.all([fetchProjects(), fetchJiraProjectMap()]);
  });

  async function fetchJiraProjectMap() {
    try {
      const directory = await fetchDeliveryDirectory();
      jiraProjectMap = Object.fromEntries(
        directory.projects.map((project) => [project.project_key, project.project_name || project.project_key])
      );
    } catch (err) {
      console.error('Failed to fetch shared project directory for project autocomplete:', err);
    }
  }

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
    formBaseScore = '60.0';
    formBaseScoreWeightPercent = '10';
    errorMsg = '';
    successMsg = '';
    resetSettingsWorkspaceScroll();
  }

  function cleanProjectName(name: string): string {
    if (!name) return '';
    const lastOpenParen = name.lastIndexOf('(');
    const lastCloseParen = name.lastIndexOf(')');
    if (lastOpenParen > 0 && lastCloseParen > lastOpenParen) {
      const potentialKey = name.substring(lastOpenParen + 1, lastCloseParen).trim();
      if (potentialKey && /^[A-Z0-9]{2,10}$/i.test(potentialKey)) {
        return name.substring(0, lastOpenParen).trim();
      }
    }
    return name;
  }

  function startEdit(project: ProjectConfig) {
    isEditing = true;
    editingProject = project;
    const keyUpper = project.project_key.toUpperCase();
    formProjectName = cleanProjectName(jiraProjectMap[keyUpper] || project.project_name);
    formProjectKey = project.project_key;
    formBasePriority = project.base_priority || 'P2';
    formProjectPhase = project.project_phase || '交付';
    formBaseScore = String(project.base_score !== undefined ? project.base_score : 60.0);
    formBaseScoreWeightPercent = String(project.base_score_weight !== undefined ? Math.round(project.base_score_weight * 100) : 10);
    errorMsg = '';
    successMsg = '';
    resetSettingsWorkspaceScroll();
  }

  function handleProjectKeyChange(event: CustomEvent<string>) {
    const key = event.detail;
    const projectName = cleanProjectName(jiraProjectMap[key.toUpperCase()] || '');
    if (projectName) formProjectName = projectName;
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

    const parsedBaseScore = Number.parseFloat(formBaseScore);
    const parsedBaseScoreWeight = Number.parseFloat(formBaseScoreWeightPercent);

    const payload = {
      project_key: formProjectKey.trim().toUpperCase(),
      project_name: formProjectName.trim(),
      base_priority: formBasePriority,
      project_phase: formProjectPhase,
      base_score: Number.isFinite(parsedBaseScore) ? parsedBaseScore : 60,
      base_score_weight: Number.isFinite(parsedBaseScoreWeight) ? parsedBaseScoreWeight / 100 : 0.1,
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

  function formatUpdated(value: string) {
    if (!value) return '暂无版本记录';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString();
  }
</script>

<div class="scw-workbench">
  {#if errorMsg}
    <Alert type="error" message={errorMsg} closable={true} on:close={() => errorMsg = ''} />
  {/if}
  {#if successMsg}
    <Alert type="success" message={successMsg} closable={true} on:close={() => successMsg = ''} />
  {/if}

  {#if !isEditing}
    <section class="scw-overview" aria-label="项目优先级配置">
      <header class="scw-header">
        <div class="scw-header-copy">
          <span class="scw-kicker">项目映射</span>
          <h4>项目优先级与阶段</h4>
          <p>维护 Jira Project Key、项目阶段和基准优先级，供排期、健康度与治理规则统一引用。</p>
        </div>
        <div class="scw-section-actions"><Button variant="primary" on:click={startCreate}>新建项目映射</Button></div>
      </header>

      <div class="scw-status tone-{projectStatusTone}">
        <div class="scw-status-main">
          <span>映射状态</span>
          <strong>{projectStatusLabel}</strong>
        </div>
        <p>{projectStatusMessage}</p>
      </div>

      <div class="scw-metrics" aria-label="项目配置概览">
        <div class="scw-metric"><span>已配置</span><strong>{projects.length} 个项目</strong></div>
        <div class="scw-metric"><span>待映射</span><strong>{availableKeys.length} 个 Jira Key</strong></div>
        <div class="scw-metric"><span>高优项目</span><strong>{highPriorityCount} 个 P0/P1</strong></div>
        <div class="scw-metric"><span>最近更新</span><strong>{formatUpdated(lastUpdated)}</strong></div>
      </div>

    {#if loading && projects.length === 0}
      <div class="scw-skeleton" aria-label="项目映射加载中"><span></span><span></span><span></span></div>
    {:else if projects.length === 0}
      <div class="scw-empty">
        <strong>暂无项目映射</strong>
        <p>先从 Jira 同步项目中选择一个 Project Key，再设置阶段、优先级和健康度基准。</p>
        <div class="scw-section-actions"><Button variant="primary" on:click={startCreate}>新建项目映射</Button></div>
      </div>
    {:else}
      <div class="scw-table-wrap">
        <table class="scw-table">
          <thead>
            <tr>
              <th>项目 (Project)</th>
              <th>运作阶段</th>
              <th>优先级</th>
              <th>基础分 (权重)</th>
              <th class="numeric">操作</th>
            </tr>
          </thead>
          <tbody>
            {#each projects as project}
              <tr>
                <td class="scw-primary-cell">
                  {#if jiraProjectMap[project.project_key.toUpperCase()]}
                    <strong>{cleanProjectName(jiraProjectMap[project.project_key.toUpperCase()])}</strong><span class="scw-mono">{project.project_key}</span>
                  {:else}
                    <strong>{cleanProjectName(project.project_name)}</strong><span class="scw-mono">{project.project_key}</span>
                  {/if}
                </td>
                <td>
                  <span class="scw-badge">
                    {project.project_phase || '交付'}
                  </span>
                </td>
                <td>
                  <span class="scw-badge {['P0', 'P1'].includes(project.base_priority) ? 'danger' : project.base_priority === 'P2' ? 'warning' : ''}">
                    {project.base_priority}
                  </span>
                </td>
                <td class="numeric">
                  <span class="scw-mono scw-tabular">
                    {project.base_score !== undefined ? project.base_score : 60}分 
                    ({project.base_score_weight !== undefined ? Math.round(project.base_score_weight * 100) : 10}%)
                  </span>
                </td>
                <td class="numeric">
                  <Button size="small" variant="ghost" on:click={() => startEdit(project)}>编辑</Button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
    </section>
  {:else}
    <section class="scw-editor" aria-label={editingProject ? '编辑项目配置' : '新建项目配置'}>
      <header class="scw-header">
        <div class="scw-header-copy">
          <span class="scw-kicker">项目映射</span>
          <h4>{editingProject ? '编辑项目配置' : '新建项目配置'}</h4>
          <p>绑定 Jira Project Key，并设置项目阶段、优先级和健康度基础参数。</p>
        </div>
      </header>

    <div class="scw-form-grid">
      <div class="wide">
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
          <Select
            id="project-key"
            label="Jira 项目键 (Project Key)"
            placeholder="搜索或选择 Jira 中同步的项目键 (如: HIT)"
            bind:value={formProjectKey}
            options={projectKeyOptions}
            searchable={true}
            compact={true}
            required={true}
            on:change={handleProjectKeyChange}
            helperText="已被其他项目集成配置占用的 Key 将不再列出。支持拼音与英文字符过滤。"
          />
        {/if}
      </div>

      <div class="wide">
        <TextInput
          id="project-name"
          label="项目名称"
          placeholder=""
          bind:value={formProjectName}
          required={true}
          helperText="展示在项目大盘与诊断报表中的项目中文名。"
        />
      </div>

      <div class="scw-form-grid wide">
        <div class="flex-1">
          <Select
            id="project-phase"
            label="项目阶段 (Project Phase)"
            bind:value={formProjectPhase}
            options={phaseOptions}
            searchable={false}
            compact={true}
          />
        </div>

        <div class="flex-1">
          <Select
            id="project-priority"
            label="项目优先级 (Base Priority)"
            bind:value={formBasePriority}
            options={priorityOptions}
            searchable={false}
            compact={true}
          />
        </div>
      </div>

      <div class="scw-form-grid wide">
        <div class="flex-1">
          <TextInput
            id="project-base-score"
            label="项目基础分 (Base Score)"
            type="number"
            bind:value={formBaseScore}
            required={true}
            helperText="项目基础打分（0-100分），作为健康度的计算起点。"
          />
        </div>

        <div class="flex-1">
          <TextInput
            id="project-base-score-weight"
            label="基础分权重 (Base Weight, %)"
            type="number"
            bind:value={formBaseScoreWeightPercent}
            required={true}
            helperText="基础分占综合健康度 PHDI 的计算权重比例（0-100%）。"
          />
        </div>
      </div>

      <div class="scw-actions wide">
        <Button variant="secondary" disabled={saving} on:click={cancelEdit}>取消</Button>
        <Button variant="primary" loading={saving} on:click={saveProject}>保存配置</Button>
      </div>
    </div>
    </section>
  {/if}
</div>

<style>
  .flex-1 {
    min-width: 0;
  }
</style>
