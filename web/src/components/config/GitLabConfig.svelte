<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import TextInput from '../shared/TextInput.svelte';
  import Switch from '../shared/Switch.svelte';
  import Button from '../shared/Button.svelte';
  import Alert from '../shared/Alert.svelte';
  import { resetSettingsWorkspaceScroll } from '../../lib/settings-ui';

  const dispatch = createEventDispatcher();

  export let config: {
    enabled?: boolean;
    base_url: string;
    secret_token: string;
    api_token?: string;
    repos: Array<{ name: string; path: string; project_id: string }>;
  } = { enabled: false, base_url: '', secret_token: '', api_token: '', repos: [] };

  export let saving = false;
  export let saveError = '';
  export let saveSuccess = false;
  export let lastUpdated = '';

  let currentStep = 1;
  const steps = ['连接', '仓库', '确认'];
  let editing = false;
  let showAPITokenEditor = !config.api_token;
  let showSecretEditor = !config.secret_token;

  // Step 1 states
  let enabled = config.enabled ?? false;
  let baseURL = config.base_url || '';
  let apiToken = config.api_token || '';
  let testingConnection = false;
  let testError = '';
  let testSuccess = '';
  let testDetails = '';

  // Step 2 states
  let secretToken = config.secret_token || '';
  let repoList = [...(config.repos || [])];
  let newRepoName = '';
  let newRepoPath = '';
  let newRepoID = '';
  let repoError = '';

  // GitLab Autoload states
  let availableProjects: Array<{ id: number; name: string; path_with_namespace: string; path: string }> = [];
  let loadingProjects = false;
  let loadProjectsError = '';
  let projectSearchQuery = '';

  type WebhookProjectResult = {
    project_id?: string;
    name?: string;
    path?: string;
    status: string;
    message: string;
    hook_id?: number;
  };

  let webhookSyncLoading = false;
  let webhookStatusLoading = false;
  let webhookSyncError = '';
  let webhookResults: WebhookProjectResult[] = [];
  let webhookResultURL = '';

  $: webhookReady = !!(baseURL && apiToken && secretToken && repoList.length > 0);
  $: webhookSummary = summarizeWebhookResults(webhookResults);
  $: isConfigured = !!(baseURL || apiToken || secretToken || repoList.length > 0);
  $: filteredAvailableProjects = availableProjects
    .filter((project) => {
      const query = projectSearchQuery.trim().toLowerCase();
      if (!query) return true;
      return project.name.toLowerCase().includes(query)
        || project.path_with_namespace.toLowerCase().includes(query)
        || project.path.toLowerCase().includes(query);
    })
    .slice(0, 8);
  $: configurationIssues = [
    !baseURL ? 'GitLab URL' : '',
    !secretToken ? 'Webhook Secret' : '',
    repoList.length === 0 ? '仓库映射' : ''
  ].filter(Boolean);
  $: healthTone = testingConnection
    ? 'checking'
    : configurationIssues.length > 0
      ? 'incomplete'
      : testError
        ? 'error'
        : testSuccess
          ? 'success'
          : 'unchecked';
  $: healthLabel = healthStatusLabel(healthTone);
  $: healthMessage = healthStatusMessage(healthTone);
  $: if (!editing && !saveSuccess) {
    enabled = config.enabled ?? false;
    baseURL = config.base_url || '';
    apiToken = config.api_token || '';
    secretToken = config.secret_token || '';
    repoList = [...(config.repos || [])];
    showAPITokenEditor = !config.api_token;
    showSecretEditor = !config.secret_token;
  }

  function formatUpdated(value: string) {
    if (!value) return '暂无版本记录';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString();
  }

  function openEditor() {
    editing = true;
    currentStep = 1;
    testError = '';
    testSuccess = '';
    testDetails = '';
    resetSettingsWorkspaceScroll();
  }

  function finishClose() {
    editing = false;
    dispatch('close');
  }

  async function fetchGitLabProjects() {
    if (!baseURL || !apiToken) {
      loadProjectsError = '请确保已在第一步填写 GitLab URL 并配置了 API 访问令牌';
      return;
    }
    loadingProjects = true;
    loadProjectsError = '';
    try {
      const res = await fetch(`/api/gitlab/projects?base_url=${encodeURIComponent(baseURL)}&api_token=${encodeURIComponent(apiToken)}`);
      if (res.ok) {
        availableProjects = await res.json();
      } else {
        const text = await res.text();
        loadProjectsError = `拉取失败: ${text}`;
      }
    } catch (e: any) {
      loadProjectsError = `请求失败: ${e.message}`;
    } finally {
      loadingProjects = false;
    }
  }

  function addAutoloadedProject(p: any) {
    repoError = '';
    const newRepoID = String(p.id);
    if (repoList.some(r => r.project_id === newRepoID)) {
      repoError = `仓库 ${p.name} (ID: ${newRepoID}) 已经存在`;
      return;
    }
    repoList = [
      ...repoList,
      {
        name: p.name,
        path: p.path_with_namespace || p.path,
        project_id: newRepoID
      }
    ];
  }

  // Webhook URL display helper
  let webhookURL = '';

  onMount(() => {
    // Generate webhook URL based on the current domain
    const host = window.location.origin;
    webhookURL = `${host}/api/webhook/gitlab`;
  });

  function generateRandomToken() {
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
    let token = '';
    for (let i = 0; i < 24; i++) {
      token += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    secretToken = token;
  }

  async function testConnection() {
    if (!baseURL) {
      testError = '请填写 GitLab 连接地址';
      return;
    }

    testingConnection = true;
    testError = '';
    testSuccess = '';
    testDetails = '';

    try {
      const res = await fetch('/api/config/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: 'gitlab',
          gitlab: {
            base_url: baseURL,
            secret_token: secretToken
          }
        })
      });

      if (!res.ok) throw new Error(`HTTP 错误: ${res.status}`);
      const data = await res.json();

      if (data.success) {
        testSuccess = data.message;
        testDetails = data.details || '';
      } else {
        testError = data.message;
        testDetails = data.details || '';
      }
    } catch (e: any) {
      testError = '连接测试请求失败，请检查网络或后端状态';
      testDetails = e.message;
    } finally {
      testingConnection = false;
    }
  }

  function addRepo() {
    repoError = '';
    if (!newRepoName || !newRepoPath || !newRepoID) {
      repoError = '请填写完整的仓库名称、路径及 Project ID';
      return;
    }

    // Check if project_id or path already exists
    if (repoList.some(r => r.project_id === newRepoID)) {
      repoError = `Project ID: ${newRepoID} 已经存在`;
      return;
    }

    repoList = [
      ...repoList,
      {
        name: newRepoName,
        path: newRepoPath,
        project_id: newRepoID
      }
    ];

    // Reset inputs
    newRepoName = '';
    newRepoPath = '';
    newRepoID = '';
  }

  function removeRepo(index: number) {
    repoList = repoList.filter((_, i) => i !== index);
  }

  let copied = false;
  function copyWebhookURL() {
    navigator.clipboard.writeText(webhookURL);
    copied = true;
    setTimeout(() => {
      copied = false;
    }, 2000);
  }

  function summarizeWebhookResults(results: WebhookProjectResult[]) {
    return results.reduce((summary, item) => {
      const key = item.status || 'unknown';
      summary[key] = (summary[key] || 0) + 1;
      return summary;
    }, {} as Record<string, number>);
  }

  function webhookStatusLabel(status: string) {
    const labels: Record<string, string> = {
      ok: '正常',
      created: '已创建',
      updated: '已更新',
      missing: '未安装',
      drift: '配置漂移',
      skipped: '已跳过',
      error: '失败'
    };
    return labels[status] || status || '未知';
  }

  function healthStatusLabel(status: string) {
    const labels: Record<string, string> = {
      checking: '检测中',
      incomplete: '配置不完整',
      error: '检测失败',
      success: '检测通过',
      unchecked: '未检测'
    };
    return labels[status] || '未检测';
  }

  function healthStatusMessage(status: string) {
    if (status === 'checking') return '正在请求 GitLab 连接测试。';
    if (status === 'incomplete') return `缺少 ${configurationIssues.join('、')}，请补齐后再巡检。`;
    if (status === 'error') return testError || '最近一次检测失败，请查看错误详情。';
    if (status === 'success') return testSuccess || '最近一次检测通过。';
    return '尚未执行健康检查，不默认判定为已就绪。';
  }

  async function callWebhookAutomation(mode: 'status' | 'ensure') {
    webhookSyncError = '';
    if (!webhookReady) {
      webhookSyncError = '请先保存 GitLab URL、API Token、Secret Token 与仓库清单后再执行 Webhook 自动配置。';
      return;
    }

    if (mode === 'status') {
      webhookStatusLoading = true;
    } else {
      webhookSyncLoading = true;
    }

    try {
      const params = new URLSearchParams({ webhook_url: webhookURL });
      const endpoint = mode === 'status'
        ? `/api/gitlab/webhooks/status?${params.toString()}`
        : '/api/gitlab/webhooks/ensure';
      const res = await fetch(endpoint, {
        method: mode === 'status' ? 'GET' : 'POST',
        headers: mode === 'status' ? undefined : { 'Content-Type': 'application/json' },
        body: mode === 'status' ? undefined : JSON.stringify({
          webhook_url: webhookURL,
          repos: repoList
        })
      });

      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `HTTP ${res.status}`);
      }

      const data = await res.json();
      webhookResults = data.results || [];
      webhookResultURL = data.webhook_url || webhookURL;
    } catch (e: any) {
      webhookSyncError = e.message || 'Webhook 自动配置请求失败';
    } finally {
      webhookStatusLoading = false;
      webhookSyncLoading = false;
    }
  }

  async function saveConfig(isToggle = false) {
    const updatedGitLab = {
      enabled: enabled,
      base_url: baseURL,
      secret_token: secretToken,
      api_token: apiToken,
      repos: repoList
    };

    dispatch('save', {
      key: 'gitlab',
      data: updatedGitLab,
      isToggle: isToggle
    });
  }

  function nextStep() {
    if (currentStep === 1) {
      if (!baseURL) {
        testError = '请填写 GitLab 连接地址';
        return;
      }
      currentStep = 2;
    } else if (currentStep === 2) {
      currentStep = 3;
    }
  }

  function prevStep() {
    if (currentStep > 1) {
      currentStep -= 1;
    }
  }
</script>

<div class="gitlab-workbench" class:editing>
  {#if saveSuccess}
    <Alert type="success" title="配置已保存" message="GitLab 连接、凭证和仓库监听清单已更新。版本审计会记录本次变更。" />
  {/if}

  {#if !editing && isConfigured}
    <section class="config-overview" aria-label="GitLab 配置状态">
      <header class="config-section-header">
        <div>
          <span class="section-kicker">仓库同步</span>
          <h4>GitLab 配置状态</h4>
          <p>默认以只读摘要呈现敏感配置，健康状态必须来自真实检查或明确的配置完整性判断。</p>
        </div>
        <div class="switch-action">
          <span>启用 GitLab 同步</span>
          <Switch id="gitlab-overview-toggle" label="启用 GitLab 同步" bind:checked={enabled} on:change={() => saveConfig(true)} />
        </div>
      </header>

      <div class="state-banner tone-{healthTone}">
        <div>
          <span>链路健康</span>
          <strong>{healthLabel}</strong>
        </div>
        <p>{healthMessage}</p>
      </div>

      <div class="overview-grid">
        <div class="overview-row">
          <span>GitLab 基础 URL 地址</span>
          <strong class="font-mono">{baseURL || '-'}</strong>
        </div>
        <div class="overview-row">
          <span>项目仓库绑定</span>
          <strong>{repoList.length} 个仓库</strong>
        </div>
        <div class="overview-row">
          <span>API 访问令牌</span>
          <strong>{apiToken ? '已配置，已脱敏' : '未配置'}</strong>
        </div>
        <div class="overview-row">
          <span>Webhook 密钥凭证</span>
          <strong>{secretToken ? '已配置，已脱敏' : '未配置'}</strong>
        </div>
        <div class="overview-row">
          <span>最近更新时间</span>
          <strong>{formatUpdated(lastUpdated)}</strong>
        </div>
        <div class="overview-row">
          <span>Webhook 自动化</span>
          <strong>{webhookReady ? '可巡检/安装' : '待补齐配置'}</strong>
        </div>
      </div>

      {#if testDetails}
        <pre class="details-pre font-mono">{testDetails}</pre>
      {/if}

      <section class="config-section">
        <div class="config-section-title">
          <div>
            <h5>被监听项目仓库</h5>
            <p>首屏直接显示仓库事实，不再只显示一个总数。</p>
          </div>
          <span class="repo-count">{repoList.length} repos</span>
        </div>

        {#if repoList.length === 0}
          <div class="empty-repos">暂无映射仓库。进入编辑后可手动添加或从 GitLab 拉取项目。</div>
        {:else}
          <div class="table-container repo-overview-table">
            <table class="repo-table">
              <thead>
                <tr>
                  <th>项目名称</th>
                  <th>GitLab 路径</th>
                  <th>Project ID</th>
                  <th>Webhook</th>
                </tr>
              </thead>
              <tbody>
                {#each repoList as repo}
                  <tr>
                    <td class="repo-main">
                      <strong>{repo.name || '-'}</strong>
                      <span>{repo.path || '-'}</span>
                    </td>
                    <td class="font-mono">{repo.path || '-'}</td>
                    <td class="font-mono tabular">{repo.project_id || '-'}</td>
                    <td><span class="webhook-status-pill {webhookReady ? 'ok' : 'missing'}">{webhookReady ? '可巡检' : '待配置'}</span></td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </section>

      <section class="webhook-automation-panel">
        <div class="webhook-auto-header">
          <div>
            <h5>Webhook 自动化</h5>
            <p>Payload URL、状态巡检与安装更新放在同一个操作区。</p>
          </div>
          <div class="webhook-auto-actions">
            <Button variant="secondary" loading={webhookStatusLoading} disabled={!webhookReady || webhookSyncLoading} on:click={() => callWebhookAutomation('status')}>
              巡检状态
            </Button>
            <Button variant="primary" loading={webhookSyncLoading} disabled={!webhookReady || webhookStatusLoading} on:click={() => callWebhookAutomation('ensure')}>
              安装/更新
            </Button>
          </div>
        </div>

        <div class="webhook-display compact">
          <div class="webhook-label-row">
            <span class="webhook-label">Payload URL</span>
            <button class="copy-link" type="button" on:click={copyWebhookURL}>{copied ? '已复制' : '复制链接'}</button>
          </div>
          <div class="webhook-url-box font-mono">{webhookResultURL || webhookURL || '等待浏览器生成地址'}</div>
        </div>

        {#if Object.keys(webhookSummary).length > 0}
          <div class="webhook-auto-meta">
            {#each Object.entries(webhookSummary) as [status, count]}
              <span>{webhookStatusLabel(status)} {count}</span>
            {/each}
          </div>
        {/if}

        {#if webhookSyncError}
          <div class="webhook-auto-error">{webhookSyncError}</div>
        {/if}

        {#if webhookResults.length > 0}
          <div class="webhook-result-table">
            <table>
              <thead>
                <tr>
                  <th>仓库</th>
                  <th>Project</th>
                  <th>状态</th>
                  <th>说明</th>
                </tr>
              </thead>
              <tbody>
                {#each webhookResults as item}
                  <tr>
                    <td>
                      <span class="repo-name-cell">{item.name || item.path || item.project_id || '-'}</span>
                      {#if item.path}
                        <span class="repo-path-cell font-mono">{item.path}</span>
                      {/if}
                    </td>
                    <td class="font-mono tabular">{item.project_id || item.hook_id || '-'}</td>
                    <td>
                      <span class="webhook-status-pill {item.status}">{webhookStatusLabel(item.status)}</span>
                    </td>
                    <td>{item.message}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}

        {#if !webhookReady}
          <div class="webhook-auto-hint">需要保存 GitLab URL、API Token、Secret Token 与至少 1 个仓库后才能自动安装。</div>
        {/if}
      </section>

      <div class="overview-actions">
        <Button variant="secondary" loading={testingConnection} on:click={testConnection}>健康检查</Button>
        <Button variant="primary" on:click={openEditor}>编辑配置</Button>
      </div>
    </section>
  {:else}
    <section class="edit-workbench" aria-label="编辑 GitLab 配置">
      <div class="workflow-steps" aria-label="配置步骤">
        {#each steps as step, index}
          {@const stepNum = index + 1}
          <button
            type="button"
            class:active={currentStep === stepNum}
            class:completed={currentStep > stepNum}
            on:click={() => currentStep = stepNum}
          >
            <span>{stepNum}</span>
            {step}
          </button>
        {/each}
      </div>

      {#if currentStep === 1}
        <div class="step-content">
          <div class="info-block">
            <h4>连接与凭证</h4>
            <p>先维护 GitLab 地址和 API Token。Webhook 接收地址可直接复制到 GitLab 项目设置。</p>
          </div>

          <div class="form-grid">
            <TextInput
              id="gitlab-url"
              label="GitLab 基础 URL 地址"
              placeholder="https://gitlab.yourdomain.com"
              bind:value={baseURL}
              required
              helperText="用于连接测试、仓库拉取和 Webhook 地址校验。"
              error={testError && !baseURL ? testError : ''}
            />

            {#if showAPITokenEditor}
              <TextInput
                id="gitlab-api-token"
                label="GitLab API 访问令牌"
                placeholder="输入具有 read_api 权限的 Personal Access Token"
                type="password"
                bind:value={apiToken}
                helperText="用于自动拉取项目和执行 Webhook 自动化。"
              />
            {:else}
              <div class="credential-collapsed">
                <div>
                  <span>GitLab API Token</span>
                  <strong>已配置，默认脱敏</strong>
                </div>
                <button type="button" on:click={() => showAPITokenEditor = true}>更换令牌</button>
              </div>
            {/if}
          </div>

          <div class="webhook-display">
            <div class="webhook-label-row">
              <span class="webhook-label">Payload URL</span>
              <button class="copy-link" type="button" on:click={copyWebhookURL}>{copied ? '已复制' : '复制链接'}</button>
            </div>
            <div class="webhook-url-box font-mono">{webhookURL || '等待浏览器生成地址'}</div>
            <p class="webhook-help">将此地址填写到 GitLab Webhooks 的 URL 字段。</p>
          </div>

          {#if testSuccess}
            <Alert type="success" title="连接成功" message={testSuccess}>
              {#if testDetails}
                <pre class="details-pre font-mono">{testDetails}</pre>
              {/if}
            </Alert>
          {/if}

          {#if testError && testDetails}
            <Alert type="error" title="连接失败" message={testError}>
              <pre class="details-pre font-mono">{testDetails}</pre>
            </Alert>
          {/if}

          <div class="actions">
            <Button variant="secondary" loading={testingConnection} on:click={testConnection}>
              测试连接
            </Button>
            <Button variant="primary" on:click={nextStep}>
              下一步
            </Button>
          </div>
        </div>
      {:else if currentStep === 2}
        <div class="step-content">
          <div class="info-block">
            <h4>仓库与 Webhook Secret</h4>
            <p>维护签名令牌和监听仓库。支持从 GitLab 拉取项目，也保留手动映射。</p>
          </div>

          {#if showSecretEditor}
            <div class="token-row">
              <div class="token-input">
                <TextInput
                  id="gitlab-secret"
                  label="Webhook Secret Token"
                  placeholder="自定义或随机生成的安全令牌"
                  type="password"
                  bind:value={secretToken}
                  helperText="需同时填入 GitLab Webhook 设置中的 Secret Token 字段。"
                />
              </div>
              <button class="gen-btn" type="button" on:click={generateRandomToken}>
                随机生成
              </button>
            </div>
          {:else}
            <div class="credential-collapsed">
              <div>
                <span>Webhook Secret Token</span>
                <strong>已配置，默认脱敏</strong>
              </div>
              <button type="button" on:click={() => showSecretEditor = true}>更换密钥</button>
            </div>
          {/if}

          {#if apiToken}
            <section class="autoload-section">
              <div class="autoload-header">
                <div>
                  <h5>从 GitLab 拉取项目</h5>
                  <p>按名称或命名空间筛选后导入到监听清单。</p>
                </div>
                <Button size="small" variant="secondary" loading={loadingProjects} on:click={fetchGitLabProjects}>
                  {availableProjects.length > 0 ? '重新拉取' : '自动拉取项目'}
                </Button>
              </div>
              {#if loadProjectsError}
                <div class="error-msg-banner">{loadProjectsError}</div>
              {/if}
              {#if availableProjects.length > 0}
                <div class="project-search-bar">
                  <input
                    type="text"
                    bind:value={projectSearchQuery}
                    placeholder="输入关键字过滤 GitLab 项目"
                  />
                </div>
                <div class="project-cards-container">
                  {#each filteredAvailableProjects as p}
                    <div class="project-card-item">
                      <div class="proj-info">
                        <span class="proj-name">{p.name}</span>
                        <span class="proj-path">{p.path_with_namespace || p.path}</span>
                        <span class="proj-id font-mono">ID {p.id}</span>
                      </div>
                      <Button size="small" variant="ghost" on:click={() => addAutoloadedProject(p)}>
                        导入
                      </Button>
                    </div>
                  {:else}
                    <div class="empty-projects-msg">没有匹配的 GitLab 项目</div>
                  {/each}
                </div>
              {/if}
            </section>
          {/if}

          <section class="repo-mapping-section">
            <div class="config-section-title">
              <div>
                <h5>被监听项目仓库</h5>
                <p>仓库表用于持续维护，不再嵌套成深色卡片。</p>
              </div>
              <span class="repo-count">{repoList.length} repos</span>
            </div>

            {#if repoList.length === 0}
              <div class="empty-repos">
                暂未配置映射仓库。在下方填写表单并添加，以使任务看板能自动匹配提交流程。
              </div>
            {:else}
              <div class="table-container">
                <table class="repo-table">
                  <thead>
                    <tr>
                      <th>项目名称</th>
                      <th>GitLab 路径</th>
                      <th>Project ID</th>
                      <th>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each repoList as repo, index}
                      <tr>
                        <td class="repo-main">
                          <strong>{repo.name}</strong>
                          <span>{repo.path}</span>
                        </td>
                        <td class="font-mono">{repo.path}</td>
                        <td class="font-mono tabular">{repo.project_id}</td>
                        <td>
                          <button class="delete-btn" type="button" on:click={() => removeRepo(index)}>
                            删除
                          </button>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {/if}

            <div class="add-repo-form">
              <h6>添加新的仓库映射</h6>
              <div class="add-repo-inputs">
                <TextInput
                  id="repo-name"
                  placeholder="例如：backend-core"
                  bind:value={newRepoName}
                  label="项目名称"
                />
                <TextInput
                  id="repo-path"
                  placeholder="例如：group/backend-core"
                  bind:value={newRepoPath}
                  label="GitLab 路径"
                />
                <TextInput
                  id="repo-id"
                  placeholder="例如：13"
                  bind:value={newRepoID}
                  label="Project ID"
                />
              </div>
              {#if repoError}
                <span class="error-msg">{repoError}</span>
              {/if}
              <Button variant="secondary" on:click={addRepo}>
                添加到映射清单
              </Button>
            </div>
          </section>

          <div class="actions">
            <Button variant="ghost" on:click={prevStep}>上一步</Button>
            <Button variant="primary" on:click={nextStep}>下一步</Button>
          </div>
        </div>
      {:else if currentStep === 3}
        <div class="step-content">
          <div class="info-block">
            <h4>保存前确认</h4>
            <p>确认 GitLab 地址、凭证状态和监听仓库数量。保存后会生成新的配置版本记录。</p>
          </div>

          <div class="summary-card">
            <div class="summary-row">
              <span class="summary-label">GitLab 连接地址</span>
              <span class="summary-value font-mono">{baseURL || '-'}</span>
            </div>
            <div class="summary-row">
              <span class="summary-label">API 访问令牌</span>
              <span class="summary-value">{apiToken ? '已配置，已脱敏' : '未配置'}</span>
            </div>
            <div class="summary-row">
              <span class="summary-label">Webhook Secret</span>
              <span class="summary-value">{secretToken ? '已配置，已脱敏' : '未配置'}</span>
            </div>
            <div class="summary-row">
              <span class="summary-label">同步项目数量</span>
              <span class="summary-value">{repoList.length} 个项目仓库</span>
            </div>

            {#if repoList.length > 0}
              <div class="summary-repos font-mono">
                {#each repoList as repo}
                  <div class="summary-repo-item">
                    <span>{repo.name} ({repo.path})</span>
                    <span class="badge">ID {repo.project_id}</span>
                  </div>
                {/each}
              </div>
            {/if}
          </div>

          {#if saveError}
            <Alert type="error" title="保存失败" message={saveError} />
          {/if}

          <div class="actions">
            <Button variant="ghost" on:click={prevStep} disabled={saving}>上一步</Button>
            <Button variant="secondary" on:click={finishClose} disabled={saving}>取消</Button>
            <Button variant="primary" loading={saving} on:click={() => saveConfig(false)}>
              保存并应用
            </Button>
          </div>
        </div>
      {/if}
    </section>
  {/if}
</div>

<style>
  .wizard {
    display: flex;
    flex-direction: column;
    width: 100%;
    box-sizing: border-box;
  }

  .step-content {
    display: flex;
    flex-direction: column;
    animation: slideIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .info-block {
    background: rgba(30, 41, 59, 0.4);
    border: 1px solid rgba(99, 102, 241, 0.34);
    padding: 12px 16px;
    border-radius: 8px;
    margin-bottom: 24px;
  }

  .info-block h4 {
    margin: 0 0 6px 0;
    font-size: 0.95rem;
    font-weight: 700;
    color: #cbd5e1;
  }

  .info-block p {
    margin: 0;
    font-size: 0.8rem;
    line-height: 1.5;
    color: #94a3b8;
  }

  .webhook-display {
    background: #0b0f19;
    border: 1px dashed rgba(99, 102, 241, 0.3);
    border-radius: 8px;
    padding: 16px;
    margin-bottom: 24px;
  }

  .webhook-label-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }

  .webhook-label {
    font-size: 0.8rem;
    font-weight: 600;
    color: #94a3b8;
  }

  .copy-link {
    background: transparent;
    border: none;
    color: #818cf8;
    font-size: 0.8rem;
    font-weight: 600;
    cursor: pointer;
    padding: 2px 6px;
    border-radius: 4px;
    transition: all 0.2s;
  }

  .copy-link:hover {
    background: rgba(99, 102, 241, 0.15);
    color: #a5b4fc;
  }

  .webhook-url-box {
    background: #020617;
    border: 1px solid rgba(51, 65, 85, 0.5);
    border-radius: 6px;
    padding: 10px 12px;
    color: #38bdf8;
    font-size: 0.825rem;
    word-break: break-all;
    margin-bottom: 8px;
  }

  .webhook-help {
    margin: 0;
    font-size: 0.75rem;
    color: #64748b;
    line-height: 1.4;
  }

  .config-overview {
    display: flex;
    flex-direction: column;
    gap: 18px;
    animation: slideIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .overview-header {
    display: flex;
    justify-content: space-between;
    gap: 16px;
    align-items: flex-start;
    border-bottom: 1px solid rgba(51, 65, 85, 0.42);
    padding-bottom: 16px;
  }

  .overview-kicker {
    color: #38bdf8;
    font-size: 0.68rem;
    font-weight: 800;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .overview-header p {
    margin: 0;
    color: #94a3b8;
    font-size: 0.8rem;
    line-height: 1.5;
  }

  .status-pill {
    flex: none;
    border-radius: 999px;
    padding: 5px 10px;
    font-size: 0.72rem;
    font-weight: 800;
    border: 1px solid rgba(148, 163, 184, 0.24);
  }

  .status-pill.online {
    color: #34d399;
    background: rgba(16, 185, 129, 0.1);
    border-color: rgba(16, 185, 129, 0.22);
  }

  .status-pill.warning {
    color: #fbbf24;
    background: rgba(245, 158, 11, 0.1);
    border-color: rgba(245, 158, 11, 0.22);
  }

  .overview-grid {
    display: flex;
    flex-direction: column;
    background: rgba(15, 23, 42, 0.25);
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 6px;
    padding: 0 16px;
    margin-top: 10px;
  }

  .overview-row {
    min-width: 0;
    display: flex;
    flex-direction: row;
    justify-content: space-between;
    align-items: center;
    padding: 14px 0;
    border-bottom: 1px solid rgba(51, 65, 85, 0.2);
    background: none;
    border-radius: 0;
    gap: 16px;
  }
  .overview-row:last-child {
    border-bottom: none;
  }

  .credential-collapsed {
    min-width: 0;
    background: rgba(15, 23, 42, 0.52);
    border: 1px solid rgba(51, 65, 85, 0.48);
    border-radius: 8px;
    padding: 12px;
  }

  .overview-row span {
    color: #94a3b8;
    font-size: 0.8rem;
    font-weight: 500;
  }

  .overview-row strong {
    color: #f8fafc;
    font-size: 0.85rem;
    font-weight: 600;
    text-align: right;
  }

  .credential-collapsed span {
    color: #64748b;
    font-size: 0.72rem;
    font-weight: 700;
  }

  .credential-collapsed strong {
    color: #e2e8f0;
    font-size: 0.86rem;
    overflow-wrap: anywhere;
  }

  .overview-actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
    flex-wrap: wrap;
  }

  .credential-collapsed {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    margin-bottom: 18px;
  }

  .credential-collapsed div {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .credential-collapsed button {
    flex: none;
    background: transparent;
    border: 1px solid rgba(99, 102, 241, 0.34);
    color: #a5b4fc;
    border-radius: 6px;
    padding: 7px 10px;
    font-size: 0.78rem;
    font-weight: 700;
    cursor: pointer;
  }

  .credential-collapsed button:hover {
    background: rgba(99, 102, 241, 0.12);
  }

  .webhook-automation-panel {
    width: 100%;
    background: rgba(15, 23, 42, 0.48);
    border: 1px solid rgba(51, 65, 85, 0.48);
    border-radius: 8px;
    padding: 14px;
    margin: 18px 0;
    box-sizing: border-box;
    text-align: left;
  }

  .webhook-auto-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 16px;
  }

  .webhook-auto-header h5 {
    margin: 0 0 4px 0;
    color: #e2e8f0;
    font-size: 0.9rem;
  }

  .webhook-auto-header p,
  .webhook-auto-hint {
    margin: 0;
    color: #64748b;
    font-size: 0.74rem;
    line-height: 1.45;
  }

  .webhook-auto-actions {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .webhook-auto-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 12px;
  }

  .webhook-auto-meta span {
    border: 1px solid rgba(51, 65, 85, 0.42);
    background: rgba(2, 6, 23, 0.36);
    border-radius: 6px;
    padding: 5px 8px;
    color: #94a3b8;
    font-size: 0.68rem;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .webhook-auto-error {
    margin-top: 10px;
    border: 1px solid rgba(248, 113, 113, 0.24);
    background: rgba(127, 29, 29, 0.14);
    color: #fca5a5;
    border-radius: 6px;
    padding: 8px 10px;
    font-size: 0.7rem;
    white-space: pre-wrap;
  }

  .webhook-result-table {
    width: 100%;
    overflow-x: auto;
    margin-top: 12px;
    border: 1px solid rgba(51, 65, 85, 0.34);
    border-radius: 8px;
  }

  .webhook-result-table table {
    width: 100%;
    border-collapse: collapse;
    min-width: 620px;
    font-size: 0.75rem;
  }

  .webhook-result-table th,
  .webhook-result-table td {
    padding: 9px 10px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.25);
    vertical-align: top;
  }

  .webhook-result-table tr:last-child td {
    border-bottom: none;
  }

  .webhook-result-table th {
    color: #64748b;
    background: rgba(2, 6, 23, 0.28);
    font-weight: 700;
  }

  .webhook-result-table td {
    color: #cbd5e1;
  }

  .repo-name-cell,
  .repo-path-cell {
    display: block;
  }

  .repo-name-cell {
    color: #f1f5f9;
    font-weight: 700;
  }

  .repo-path-cell {
    color: #64748b;
    font-size: 0.68rem;
    margin-top: 2px;
  }

  .webhook-status-pill {
    display: inline-flex;
    align-items: center;
    min-height: 22px;
    border-radius: 999px;
    padding: 2px 8px;
    font-size: 0.68rem;
    font-weight: 700;
    border: 1px solid rgba(148, 163, 184, 0.22);
    color: #cbd5e1;
    background: rgba(148, 163, 184, 0.1);
    white-space: nowrap;
  }

  .webhook-status-pill.ok,
  .webhook-status-pill.created,
  .webhook-status-pill.updated {
    color: #86efac;
    border-color: rgba(34, 197, 94, 0.28);
    background: rgba(22, 101, 52, 0.16);
  }

  .webhook-status-pill.drift,
  .webhook-status-pill.missing {
    color: #fbbf24;
    border-color: rgba(245, 158, 11, 0.28);
    background: rgba(120, 53, 15, 0.18);
  }

  .webhook-status-pill.error {
    color: #fca5a5;
    border-color: rgba(248, 113, 113, 0.26);
    background: rgba(127, 29, 29, 0.16);
  }

  .webhook-auto-hint {
    margin-top: 10px;
  }

  .details-pre {
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 6px;
    padding: 10px;
    font-size: 0.725rem;
    color: #94a3b8;
    max-height: 120px;
    overflow-y: auto;
    margin: 8px 0 0 0;
    white-space: pre-wrap;
    word-break: break-all;
  }

  .project-cards-container,
  .summary-repos,
  .table-container,
  .webhook-result-table,
  .details-pre {
    scrollbar-width: thin;
    scrollbar-color: rgba(56, 189, 248, 0.42) rgba(15, 23, 42, 0.72);
  }

  .project-cards-container::-webkit-scrollbar,
  .summary-repos::-webkit-scrollbar,
  .table-container::-webkit-scrollbar,
  .webhook-result-table::-webkit-scrollbar,
  .details-pre::-webkit-scrollbar {
    width: 8px;
    height: 8px;
  }

  .project-cards-container::-webkit-scrollbar-track,
  .summary-repos::-webkit-scrollbar-track,
  .table-container::-webkit-scrollbar-track,
  .webhook-result-table::-webkit-scrollbar-track,
  .details-pre::-webkit-scrollbar-track {
    background: rgba(2, 6, 23, 0.42);
    border-radius: 999px;
  }

  .project-cards-container::-webkit-scrollbar-thumb,
  .summary-repos::-webkit-scrollbar-thumb,
  .table-container::-webkit-scrollbar-thumb,
  .webhook-result-table::-webkit-scrollbar-thumb,
  .details-pre::-webkit-scrollbar-thumb {
    background: linear-gradient(180deg, rgba(56, 189, 248, 0.54), rgba(99, 102, 241, 0.42));
    border: 2px solid rgba(2, 6, 23, 0.42);
    border-radius: 999px;
  }

  .project-cards-container::-webkit-scrollbar-thumb:hover,
  .summary-repos::-webkit-scrollbar-thumb:hover,
  .table-container::-webkit-scrollbar-thumb:hover,
  .webhook-result-table::-webkit-scrollbar-thumb:hover,
  .details-pre::-webkit-scrollbar-thumb:hover {
    background: linear-gradient(180deg, rgba(125, 211, 252, 0.72), rgba(129, 140, 248, 0.58));
  }

  .token-row {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    width: 100%;
  }

  .token-input {
    flex-grow: 1;
  }

  .gen-btn {
    background: #1e293b;
    border: 1px solid #334155;
    color: #cbd5e1;
    padding: 10px 14px;
    font-size: 0.825rem;
    font-weight: 600;
    border-radius: 8px;
    cursor: pointer;
    margin-top: 25px; /* Alignment with label */
    transition: all 0.2s;
    height: 38px;
  }

  .gen-btn:hover {
    background: #334155;
    color: #f1f5f9;
  }

  .repo-mapping-section {
    margin-top: 12px;
    margin-bottom: 24px;
    border-top: 1px solid rgba(51, 65, 85, 0.4);
    padding-top: 20px;
  }

  .section-subtitle {
    margin: 0 0 12px 0;
    font-size: 0.9rem;
    font-weight: 600;
    color: #cbd5e1;
  }

  .empty-repos {
    background: rgba(30, 41, 59, 0.2);
    border: 1px dashed rgba(51, 65, 85, 0.6);
    border-radius: 8px;
    padding: 24px;
    text-align: center;
    font-size: 0.8rem;
    color: #64748b;
    line-height: 1.5;
    margin-bottom: 20px;
  }

  .table-container {
    width: 100%;
    overflow-x: auto;
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 8px;
    margin-bottom: 20px;
    background: rgba(11, 15, 25, 0.4);
  }

  .repo-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8rem;
    text-align: left;
  }

  .repo-table th, .repo-table td {
    padding: 10px 14px;
    border-bottom: 1px solid rgba(51, 65, 85, 0.4);
  }

  .repo-table th {
    background: rgba(30, 41, 59, 0.5);
    font-weight: 600;
    color: #94a3b8;
  }

  .repo-table td {
    color: #cbd5e1;
  }

  .repo-table tr:last-child td {
    border-bottom: none;
  }

  .delete-btn {
    background: transparent;
    border: none;
    color: #f87171;
    font-weight: 600;
    cursor: pointer;
    padding: 2px 6px;
    border-radius: 4px;
    transition: all 0.2s;
  }

  .delete-btn:hover {
    background: rgba(239, 68, 68, 0.15);
  }

  .add-repo-form {
    background: rgba(15, 23, 42, 0.4);
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 8px;
    padding: 16px;
  }

  .add-repo-form h6 {
    margin: 0 0 12px 0;
    font-size: 0.825rem;
    font-weight: 600;
    color: #cbd5e1;
  }

  .add-repo-inputs {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
    margin-bottom: 12px;
  }

  .add-repo-inputs :global(.input-group) {
    margin-bottom: 0;
  }

  .error-msg {
    color: #f87171;
    font-size: 0.75rem;
    margin-bottom: 12px;
    display: block;
    font-weight: 500;
  }

  .summary-card {
    background: #0b0f19;
    border: 1px solid rgba(51, 65, 85, 0.5);
    border-radius: 8px;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    margin-bottom: 24px;
  }

  .summary-row {
    display: flex;
    justify-content: space-between;
    font-size: 0.85rem;
    border-bottom: 1px solid rgba(51, 65, 85, 0.3);
    padding-bottom: 8px;
  }

  .summary-row:last-of-type {
    border-bottom: none;
    padding-bottom: 0;
  }

  .summary-label {
    color: #64748b;
    font-weight: 500;
  }

  .summary-value {
    color: #cbd5e1;
    font-weight: 600;
    min-width: 0;
    overflow-wrap: anywhere;
    text-align: right;
  }

  .summary-repos {
    background: rgba(30, 41, 59, 0.4);
    border-radius: 6px;
    padding: 12px;
    font-size: 0.75rem;
    max-height: 120px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 8px;
    border: 1px solid rgba(51, 65, 85, 0.2);
  }

  .summary-repo-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 10px;
    color: #94a3b8;
    min-width: 0;
  }

  .summary-repo-item span:first-child {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .badge {
    flex: none;
    font-size: 0.65rem;
    background: #1e293b;
    color: #38bdf8;
    padding: 2px 6px;
    border-radius: 4px;
    border: 1px solid rgba(56, 189, 248, 0.2);
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    margin-top: 12px;
    border-top: 1px solid rgba(51, 65, 85, 0.4);
    padding-top: 16px;
  }

  .font-mono {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  @keyframes slideIn {
    from {
      opacity: 0;
      transform: translateX(8px);
    }
    to {
      opacity: 1;
      transform: translateX(0);
    }
  }

  /* Success Screen styles */
  .success-screen {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 32px 16px;
    text-align: center;
    animation: fadeIn 0.4s ease-out;
  }

  .success-title {
    font-size: 1.25rem;
    font-weight: 700;
    margin: 0 0 8px 0;
    color: var(--wa-text-strong, #0d1722);
  }

  .success-desc {
    font-size: 0.8rem;
    color: #38bdf8;
    margin: 0 0 24px 0;
    max-width: 380px;
    line-height: 1.5;
  }

  .success-icon {
    width: 64px;
    height: 64px;
    margin-bottom: 20px;
  }

  .checkmark-svg {
    width: 64px;
    height: 64px;
    border-radius: 50%;
    display: block;
    stroke-width: 3;
    stroke: #34d399;
    stroke-miterlimit: 10;
    box-shadow: inset 0px 0px 0px #34d399;
    animation: fill .4s ease-in-out .4s forwards, scale .3s ease-in-out .9s both;
  }

  .checkmark-circle {
    stroke-dasharray: 166;
    stroke-dashoffset: 166;
    stroke-width: 3;
    stroke-miterlimit: 10;
    stroke: #34d399;
    fill: none;
    animation: stroke 0.6s cubic-bezier(0.65, 0, 0.45, 1) forwards;
  }

  .checkmark-check {
    transform-origin: 50% 50%;
    stroke-dasharray: 48;
    stroke-dashoffset: 48;
    animation: stroke 0.3s cubic-bezier(0.65, 0, 0.45, 1) 0.6s forwards;
  }

  .success-actions {
    display: flex;
    justify-content: center;
    width: 100%;
  }

  @keyframes stroke {
    100% {
      stroke-dashoffset: 0;
    }
  }

  @keyframes fill {
    100% {
      box-shadow: inset 0px 0px 0px 32px rgba(52, 211, 153, 0.1);
    }
  }

  @keyframes scale {
    0%, 100% {
      transform: none;
    }
    50% {
      transform: scale3d(1.1, 1.1, 1);
    }
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }
  .status-indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    display: inline-block;
  }
  .indicator-online {
    background-color: #10b981;
    box-shadow: 0 0 8px #10b981;
  }
  .indicator-warning {
    background-color: #f59e0b;
    box-shadow: 0 0 8px #f59e0b;
  }

  /* Phase 47 GitLab settings workbench. Native light-admin contract for the main content path. */
  .gitlab-workbench {
    --config-line: rgba(106, 126, 145, 0.16);
    --config-line-strong: rgba(74, 97, 118, 0.26);
    --config-surface-soft: rgba(246, 249, 250, 0.78);
    --config-ink: var(--wa-text-strong, #0d1722);
    --config-text: var(--wa-text-main, #293847);
    --config-muted: var(--wa-text-muted, #667789);
    --config-subtle: var(--wa-text-subtle, #8a99aa);
    --config-accent: var(--wa-accent, #008f96);
    --config-accent-strong: var(--wa-accent-strong, #006f76);
    --config-accent-soft: var(--wa-accent-soft, rgba(0, 143, 150, 0.12));
    --config-success: var(--wa-success, #04966f);
    --config-warning: var(--wa-warning, #d88700);
    --config-danger: var(--wa-danger, #dd4b3e);
    width: 100%;
    min-width: 0;
    grid-template-columns: minmax(0, 1fr);
    color: var(--config-text);
  }

  .gitlab-workbench,
  .config-overview,
  .edit-workbench,
  .step-content {
    width: 100%;
    min-width: 0;
    max-width: 100%;
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 18px;
  }

  .config-section,
  .webhook-automation-panel,
  .autoload-section,
  .repo-mapping-section,
  .add-repo-form,
  .summary-card,
  .webhook-display,
  .credential-collapsed,
  .info-block {
    min-width: 0;
    max-width: 100%;
  }

  .config-section-header,
  .webhook-auto-header,
  .config-section-title,
  .webhook-label-row,
  .autoload-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
    min-width: 0;
  }

  .config-section-header {
    min-height: 66px;
    padding: 0 0 16px;
    border-bottom: 1px solid var(--config-line);
  }

  .section-kicker,
  .overview-kicker {
    color: var(--config-accent-strong);
    font-size: 11px;
    font-weight: 780;
    letter-spacing: 0;
    text-transform: none;
  }

  .config-section-header h4,
  .config-section-title h5,
  .autoload-header h5,
  .webhook-auto-header h5,
  .info-block h4 {
    margin: 3px 0 0;
    color: var(--config-ink);
    font-size: 16px;
    line-height: 1.3;
  }

  .config-section-header p,
  .config-section-title p,
  .autoload-header p,
  .webhook-auto-header p,
  .info-block p,
  .webhook-help {
    margin: 5px 0 0;
    color: var(--config-muted);
    font-size: 12px;
    line-height: 1.55;
  }

  .switch-action {
    display: flex;
    align-items: center;
    gap: 10px;
    color: var(--config-muted);
    font-size: 12px;
    font-weight: 760;
    white-space: nowrap;
  }

  .switch-action :global(.switch-container) {
    margin: 0;
  }

  .switch-action :global(.switch-label) {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }

  .state-banner {
    display: grid;
    grid-template-columns: minmax(140px, 0.28fr) minmax(0, 1fr);
    gap: 14px;
    align-items: center;
    border: 1px solid var(--config-line);
    border-radius: 8px;
    background: var(--config-surface-soft);
    padding: 12px 14px;
  }

  .state-banner span,
  .overview-row span,
  .summary-label,
  .repo-count {
    color: var(--config-muted);
    font-size: 11px;
    font-weight: 700;
  }

  .state-banner strong,
  .overview-row strong,
  .summary-value {
    color: var(--config-ink);
    font-size: 13px;
    font-weight: 740;
    line-height: 1.45;
    overflow-wrap: anywhere;
  }

  .state-banner p {
    margin: 0;
    color: var(--config-text);
    font-size: 12px;
    line-height: 1.5;
  }

  .state-banner.tone-success {
    border-color: rgba(4, 150, 111, 0.22);
    background: rgba(4, 150, 111, 0.08);
  }

  .state-banner.tone-error {
    border-color: rgba(221, 75, 62, 0.24);
    background: rgba(221, 75, 62, 0.08);
  }

  .state-banner.tone-incomplete,
  .state-banner.tone-unchecked,
  .state-banner.tone-checking {
    border-color: rgba(216, 135, 0, 0.2);
    background: rgba(216, 135, 0, 0.07);
  }

  .overview-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0 28px;
    margin: 0;
    padding: 0;
    border: 0;
    background: transparent;
  }

  .overview-row,
  .summary-row {
    min-width: 0;
    min-height: 64px;
    display: grid;
    align-content: center;
    gap: 5px;
    padding: 12px 0;
    border: 0;
    border-bottom: 1px solid var(--config-line);
    background: transparent;
  }

  .config-section,
  .webhook-automation-panel,
  .autoload-section,
  .repo-mapping-section,
  .add-repo-form,
  .summary-card,
  .webhook-display,
  .credential-collapsed,
  .info-block {
    margin: 0;
    padding: 16px 0;
    border: 0;
    border-top: 1px solid var(--config-line);
    border-radius: 0;
    background: transparent;
    box-shadow: none;
  }

  .webhook-display.compact {
    padding-bottom: 0;
  }

  .webhook-url-box,
  .details-pre {
    display: block;
    max-height: 152px;
    overflow: auto;
    margin: 8px 0 0;
    padding: 12px;
    border: 1px solid var(--config-line);
    border-radius: 7px;
    background: rgba(246, 249, 250, 0.86);
    color: var(--config-text);
    font-size: 12px;
    line-height: 1.5;
    word-break: break-all;
    box-shadow: none;
  }

  .copy-link,
  .credential-collapsed button,
  .delete-btn,
  .gen-btn {
    min-height: 32px;
    border: 1px solid var(--config-line);
    border-radius: 7px;
    background: rgba(255, 255, 255, 0.72);
    color: var(--config-accent-strong);
    padding: 0 10px;
    font-size: 12px;
    font-weight: 760;
    cursor: pointer;
    box-shadow: none;
  }

  .delete-btn {
    color: var(--config-danger);
    border-color: rgba(221, 75, 62, 0.18);
    background: rgba(221, 75, 62, 0.08);
  }

  .copy-link:hover,
  .credential-collapsed button:hover,
  .gen-btn:hover {
    background: rgba(0, 143, 150, 0.08);
    color: var(--config-accent-strong);
  }

  .webhook-auto-actions,
  .overview-actions,
  .actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    flex-wrap: wrap;
    margin: 0;
    padding: 16px 0 0;
    border-top: 1px solid var(--config-line);
  }

  .overview-actions {
    margin-top: 0;
  }

  .webhook-auto-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 12px;
  }

  .webhook-auto-meta span,
  .repo-count,
  .badge {
    display: inline-flex;
    align-items: center;
    min-height: 24px;
    max-width: 100%;
    border: 1px solid var(--config-line);
    border-radius: 999px;
    background: rgba(255, 255, 255, 0.72);
    color: var(--config-muted);
    padding: 0 9px;
    font-size: 11px;
    font-weight: 760;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .webhook-auto-error,
  .error-msg-banner,
  .error-msg {
    margin-top: 10px;
    border: 1px solid rgba(221, 75, 62, 0.2);
    border-radius: 7px;
    background: rgba(221, 75, 62, 0.08);
    color: var(--config-danger);
    padding: 9px 11px;
    font-size: 12px;
    white-space: pre-wrap;
  }

  .webhook-auto-hint,
  .empty-repos,
  .empty-projects-msg {
    border: 1px dashed var(--config-line-strong);
    border-radius: 8px;
    background: rgba(246, 249, 250, 0.72);
    color: var(--config-muted);
    padding: 18px 14px;
    text-align: center;
    font-size: 12px;
    line-height: 1.5;
  }

  .table-container,
  .webhook-result-table {
    width: 100%;
    overflow: auto;
    border: 1px solid var(--config-line);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.46);
    box-shadow: none;
  }

  .repo-overview-table {
    max-height: 360px;
    overflow: auto;
    scrollbar-gutter: stable;
  }

  .repo-overview-table .repo-table thead {
    position: sticky;
    top: 0;
    z-index: 2;
  }

  .repo-table,
  .webhook-result-table table {
    width: 100%;
    min-width: 0;
    border-collapse: collapse;
    color: var(--config-text);
    font-size: 12px;
  }

  .repo-table th,
  .repo-table td,
  .webhook-result-table th,
  .webhook-result-table td {
    padding: 10px 12px;
    border-bottom: 1px solid var(--config-line);
    background: transparent;
    color: var(--config-text);
    text-align: left;
    vertical-align: top;
  }

  .repo-table th,
  .webhook-result-table th {
    background: rgba(242, 247, 248, 0.76);
    color: var(--config-muted);
    font-size: 11px;
    font-weight: 800;
  }

  .repo-table tr:last-child td,
  .webhook-result-table tr:last-child td {
    border-bottom: 0;
  }

  .repo-main {
    min-width: 0;
  }

  .repo-main strong,
  .repo-name-cell,
  .proj-name {
    display: block;
    color: var(--config-ink);
    font-weight: 760;
  }

  .repo-main span,
  .repo-path-cell,
  .proj-path,
  .proj-id {
    display: block;
    max-width: 42ch;
    margin-top: 3px;
    color: var(--config-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .tabular {
    font-variant-numeric: tabular-nums;
  }

  .webhook-status-pill {
    display: inline-flex;
    align-items: center;
    min-height: 24px;
    border-radius: 999px;
    padding: 0 9px;
    border: 1px solid var(--config-line);
    background: rgba(102, 119, 137, 0.08);
    color: var(--config-muted);
    font-size: 11px;
    font-weight: 760;
    white-space: nowrap;
  }

  .webhook-status-pill.ok,
  .webhook-status-pill.created,
  .webhook-status-pill.updated {
    color: var(--config-success);
    border-color: rgba(4, 150, 111, 0.2);
    background: rgba(4, 150, 111, 0.1);
  }

  .webhook-status-pill.drift,
  .webhook-status-pill.missing,
  .webhook-status-pill.skipped {
    color: var(--config-warning);
    border-color: rgba(216, 135, 0, 0.2);
    background: rgba(216, 135, 0, 0.1);
  }

  .webhook-status-pill.error {
    color: var(--config-danger);
    border-color: rgba(221, 75, 62, 0.2);
    background: rgba(221, 75, 62, 0.1);
  }

  .workflow-steps {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 3px;
    border: 1px solid var(--config-line);
    border-radius: 9px;
    background: rgba(240, 245, 247, 0.76);
    width: fit-content;
    max-width: 100%;
  }

  .workflow-steps button {
    min-height: 32px;
    display: inline-flex;
    align-items: center;
    gap: 7px;
    border: 0;
    border-radius: 7px;
    background: transparent;
    color: var(--config-muted);
    padding: 0 11px;
    font-weight: 760;
    cursor: pointer;
  }

  .workflow-steps button.active,
  .workflow-steps button.completed {
    background: rgba(255, 255, 255, 0.96);
    color: var(--config-accent-strong);
    box-shadow: 0 1px 3px rgba(28, 54, 67, 0.1);
  }

  .workflow-steps span {
    width: 18px;
    height: 18px;
    display: inline-grid;
    place-items: center;
    border-radius: 999px;
    background: var(--config-accent-soft);
    color: var(--config-accent-strong);
    font-size: 10px;
  }

  .form-grid,
  .add-repo-inputs {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px 20px;
  }

  .add-repo-inputs {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    margin-bottom: 12px;
  }

  .token-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: end;
    gap: 12px;
    width: 100%;
  }

  .gen-btn {
    height: 38px;
    margin: 0 0 23px;
  }

  .credential-collapsed {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
  }

  .credential-collapsed div {
    display: grid;
    gap: 4px;
  }

  .credential-collapsed span,
  .credential-collapsed strong,
  .add-repo-form h6,
  .summary-repo-item,
  .summary-card {
    color: var(--config-text);
  }

  .project-search-bar {
    margin: 12px 0;
  }

  .project-search-bar input {
    width: 100%;
    min-height: 38px;
    border: 1px solid var(--config-line-strong);
    border-radius: 7px;
    background: rgba(255, 255, 255, 0.82);
    color: var(--config-ink);
    padding: 0 12px;
    outline: none;
  }

  .project-search-bar input:focus {
    border-color: var(--config-accent);
    box-shadow: 0 0 0 3px rgba(0, 143, 150, 0.1);
  }

  .project-cards-container,
  .summary-repos {
    display: grid;
    gap: 0;
    max-height: 240px;
    overflow: auto;
    border: 1px solid var(--config-line);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.46);
  }

  .project-card-item,
  .summary-repo-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    min-width: 0;
    padding: 11px 12px;
    border-bottom: 1px solid var(--config-line);
    background: transparent;
  }

  .project-card-item:last-child,
  .summary-repo-item:last-child {
    border-bottom: 0;
  }

  .proj-info {
    min-width: 0;
    display: grid;
    gap: 3px;
  }

  .summary-card {
    display: grid;
    gap: 0;
  }

  .summary-repos {
    margin-top: 14px;
    max-height: 180px;
  }

  .summary-repo-item span:first-child {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .font-mono {
    font-family: var(--wa-font-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
  }

  @media (max-width: 860px) {
    .config-section-header,
    .webhook-auto-header,
    .config-section-title,
    .autoload-header,
    .credential-collapsed,
    .token-row {
      display: grid;
      grid-template-columns: 1fr;
    }

    .state-banner,
    .overview-grid,
    .form-grid,
    .add-repo-inputs {
      grid-template-columns: 1fr;
    }

    .workflow-steps {
      width: 100%;
    }

    .workflow-steps button {
      flex: 1;
      justify-content: center;
    }

    .actions,
    .overview-actions,
    .webhook-auto-actions {
      align-items: stretch;
      flex-direction: column;
    }

    .gen-btn {
      margin: 0;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .toast,
    .summary-modal,
    .checkmark,
    .checkmark-circle,
    .checkmark-check {
      animation: none !important;
    }
  }
</style>
