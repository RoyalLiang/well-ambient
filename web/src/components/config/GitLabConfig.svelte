<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import Steps from '../shared/Steps.svelte';
  import TextInput from '../shared/TextInput.svelte';
  import Switch from '../shared/Switch.svelte';
  import Button from '../shared/Button.svelte';
  import Alert from '../shared/Alert.svelte';

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
  const steps = ['连接地址', '项目仓库映射', '完成应用'];
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

  async function saveConfig() {
    const updatedGitLab = {
      enabled: enabled,
      base_url: baseURL,
      secret_token: secretToken,
      api_token: apiToken,
      repos: repoList
    };

    dispatch('save', {
      key: 'gitlab',
      data: updatedGitLab
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

<div class="wizard">
  {#if saveSuccess}
    <div class="success-screen">
      <div class="success-icon">
        <svg xmlns="http://www.w3.org/2000/svg" class="checkmark-svg" viewBox="0 0 52 52">
          <circle class="checkmark-circle" cx="26" cy="26" r="25" fill="none"/>
          <path class="checkmark-check" fill="none" d="M14.1 27.2l7.1 7.2 16.7-16.8"/>
        </svg>
      </div>
      <h4 class="success-title">GitLab Webhook 配置已成功应用！</h4>
      <p class="success-desc font-mono">Webhook 令牌与仓库监听清单已更新生效。</p>
      <div class="webhook-automation-panel">
        <div class="webhook-auto-header">
          <div>
            <h5>项目 Webhook 自动配置</h5>
            <p>对已保存的仓库清单执行安装、更新或状态巡检。</p>
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

        <div class="webhook-auto-meta font-mono">
          <span>REPOS {repoList.length}</span>
          <span>URL {webhookResultURL || webhookURL}</span>
          {#if Object.keys(webhookSummary).length > 0}
            <span>
              {#each Object.entries(webhookSummary) as [status, count], index}
                {index > 0 ? ' / ' : ''}{webhookStatusLabel(status)} {count}
              {/each}
            </span>
          {/if}
        </div>

        {#if webhookSyncError}
          <div class="webhook-auto-error font-mono">{webhookSyncError}</div>
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
                    <td class="font-mono">{item.project_id || item.hook_id || '-'}</td>
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
      </div>
      <div class="success-actions">
        <Button variant="primary" on:click={finishClose}>
          完成并关闭
        </Button>
      </div>
    </div>
  {:else if !editing && isConfigured}
    <div class="config-overview">
      <div class="overview-header">
        <div>
          <span class="overview-kicker font-mono">GitLab Integration</span>
          <h4>GitLab 配置状态摘要</h4>
          <p>默认以只读安全呈现各配置字段详情，支持右上角快速启用/禁用。</p>
        </div>
        <div style="display: flex; align-items: center; gap: 10px;">
          <span class="status-pill {enabled ? 'online' : 'warning'}">{enabled ? '已启用' : '已禁用'}</span>
          <Switch id="gitlab-overview-toggle" bind:checked={enabled} on:change={saveConfig} />
        </div>
      </div>

      <div class="overview-grid">
        <div class="overview-row">
          <span>GitLab 基础 URL 地址</span>
          <strong class="font-mono">{baseURL || '-'}</strong>
        </div>
        <div class="overview-row">
          <span>项目仓库绑定</span>
          <strong>已绑定 {repoList.length} 个代码多仓</strong>
        </div>
        <div class="overview-row">
          <span>API 访问令牌</span>
          <strong>{apiToken ? '已配置 (已脱敏保护)' : '未配置'}</strong>
        </div>
        <div class="overview-row">
          <span>Webhook 密钥凭证</span>
          <strong>{secretToken ? '已配置 (已脱敏保护)' : '未配置'}</strong>
        </div>
        <div class="overview-row">
          <span>最近更新时间</span>
          <strong>{formatUpdated(lastUpdated)}</strong>
        </div>
        <div class="overview-row">
          <span>健康检查状态</span>
          <strong class="text-success">{testSuccess || testError || '已就绪'}</strong>
        </div>
      </div>

      {#if testDetails}
        <pre class="details-pre font-mono">{testDetails}</pre>
      {/if}

      <div class="overview-actions">
        <Button variant="secondary" loading={testingConnection} on:click={testConnection}>健康检查</Button>
        <Button variant="primary" on:click={openEditor}>编辑配置</Button>
      </div>
    </div>
  {:else}
    <Steps {currentStep} {steps} />

    {#if currentStep === 1}
    <div class="step-content">
      <div class="info-block">
        <h4>GitLab System Webhook 配置引导</h4>
        <p>well-ambient 系统需要依靠 GitLab 的 Webhook 触发来自动同步代码提交、分支及合并请求事件。请按照以下步骤完成连接配置：</p>
      </div>

      <TextInput
        id="gitlab-url"
        label="GitLab 基础 URL 地址"
        placeholder="https://gitlab.yourdomain.com"
        bind:value={baseURL}
        required
        helperText="请输入 GitLab 的基础 URL 地址。系统将通过对此地址进行基础 HTTP 请求测试可达性。"
        error={testError}
      />

      {#if showAPITokenEditor}
        <TextInput
          id="gitlab-api-token"
          label="GitLab API 访问令牌 (Personal Access Token)"
          placeholder="输入用于自动拉取仓库列表的 Private Token"
          type="password"
          bind:value={apiToken}
          helperText="可选。若需支持在下一步中自动拉取并勾选仓库项目，请输入具有 read_api 权限的 Personal Access Token。"
        />
      {:else}
        <div class="credential-collapsed">
          <div>
            <span>GitLab API Token</span>
            <strong>已配置，当前默认脱敏折叠</strong>
          </div>
          <button type="button" on:click={() => showAPITokenEditor = true}>编辑凭证/高级配置</button>
        </div>
      {/if}

      <div class="webhook-display">
        <div class="webhook-label-row">
          <span class="webhook-label">系统 Webhook 接收地址 (Payload URL)</span>
          <button class="copy-link" type="button" on:click={copyWebhookURL}>{copied ? '已复制!' : '复制链接'}</button>
        </div>
        <div class="webhook-url-box font-mono">{webhookURL}</div>
        <p class="webhook-help">请在 GitLab 的管理中心或对应项目设置中的 <strong>Webhooks</strong> 页面，将上述链接粘贴至 <strong>URL</strong> 输入框中。</p>
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
        <h4>安全性与代码库范围映射</h4>
        <p>配置安全令牌 (Secret Token) 以防止非法请求，并添加需要被 well-ambient 追踪的项目仓库。</p>
      </div>

      {#if showSecretEditor}
        <div class="token-row">
          <div class="token-input">
            <TextInput
              id="gitlab-secret"
              label="Webhook 安全令牌 (Secret Token)"
              placeholder="自定义或随机生成的安全令牌"
              type="password"
              bind:value={secretToken}
              helperText="设置后，需同时填入 GitLab Webhook 设置中的 Secret Token 字段，用于签名校验。"
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
            <strong>已配置，当前默认脱敏折叠</strong>
          </div>
          <button type="button" on:click={() => showSecretEditor = true}>编辑凭证/高级配置</button>
        </div>
      {/if}

      <!-- Autoload GitLab Repositories -->
      {#if apiToken}
        <div class="autoload-section font-mono" style="margin-bottom: 24px; background: rgba(30, 41, 59, 0.2); border: 1px solid rgba(51, 65, 85, 0.4); border-radius: 8px; padding: 16px;">
          <div class="autoload-header" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
            <h5 style="margin: 0; font-size: 0.9rem; color: #cbd5e1; font-weight: 700;">🔍 自动拉取 GitLab 项目并映射</h5>
            <Button size="small" variant="ghost" loading={loadingProjects} on:click={fetchGitLabProjects}>
              {availableProjects.length > 0 ? '🔄 重新拉取' : '📥 自动拉取项目'}
            </Button>
          </div>
          {#if loadProjectsError}
            <div class="error-msg-banner" style="background: rgba(239, 68, 68, 0.1); border: 1px solid rgba(239, 68, 68, 0.2); color: #f87171; padding: 8px 12px; border-radius: 6px; font-size: 0.8rem; margin-bottom: 12px;">❌ {loadProjectsError}</div>
          {/if}
          {#if availableProjects.length > 0}
            <div class="project-search-bar" style="margin-bottom: 12px;">
              <input 
                type="text" 
                bind:value={projectSearchQuery} 
                placeholder="输入关键字过滤 GitLab 项目 (如 backend)..." 
                style="width: 100%; padding: 8px 12px; background: #0b0f19; border: 1px solid rgba(51, 65, 85, 0.6); border-radius: 6px; color: #f1f5f9; font-size: 0.85rem; outline: none;" 
              />
            </div>
            <div class="project-cards-container" style="display: grid; grid-template-columns: repeat(1, 1fr); gap: 8px; max-height: 200px; overflow-y: auto; padding-right: 4px;">
              {#each availableProjects.filter(p => !projectSearchQuery || p.name.toLowerCase().includes(projectSearchQuery.toLowerCase()) || p.path_with_namespace.toLowerCase().includes(projectSearchQuery.toLowerCase())).slice(0, 6) as p}
                <div class="project-card-item" style="display: flex; justify-content: space-between; align-items: center; background: #0f172a; padding: 8px 12px; border-radius: 6px; border: 1px solid rgba(51, 65, 85, 0.3);">
                  <div class="proj-info" style="display: flex; flex-direction: column; gap: 2px;">
                    <span class="proj-name" style="font-size: 0.85rem; font-weight: 700; color: #f1f5f9;">{p.name}</span>
                    <span class="proj-path" style="font-size: 0.725rem; color: #64748b;">{p.path_with_namespace || p.path}</span>
                    <span class="proj-id" style="font-size: 0.7rem; color: #38bdf8;">ID: {p.id}</span>
                  </div>
                  <Button size="small" variant="ghost" on:click={() => addAutoloadedProject(p)}>
                    ➕ 导入
                  </Button>
                </div>
              {:else}
                <div class="empty-projects-msg" style="text-align: center; color: #64748b; font-size: 0.8rem; padding: 12px;">没有匹配的 GitLab 项目</div>
              {/each}
            </div>
          {/if}
        </div>
      {/if}

      <!-- Repo Mappings -->
      <div class="repo-mapping-section">
        <h5 class="section-subtitle">被监听项目仓库映射清单</h5>
        
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
                  <th>GitLab 路径 (Path)</th>
                  <th>Project ID</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {#each repoList as repo, index}
                  <tr>
                    <td>{repo.name}</td>
                    <td class="font-mono">{repo.path}</td>
                    <td class="font-mono">{repo.project_id}</td>
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
            + 添加到映射清单
          </Button>
        </div>
      </div>

      <div class="actions">
        <Button variant="ghost" on:click={prevStep}>上一步</Button>
        <Button variant="primary" on:click={nextStep}>下一步</Button>
      </div>
    </div>
  {:else if currentStep === 3}
    <div class="step-content">
      <div class="info-block">
        <h4>GitLab 配置摘要汇总</h4>
        <p>确认无误后点击下方按钮应用并应用配置：</p>
      </div>

      <div class="summary-card">
        <div class="summary-row">
          <span class="summary-label">GitLab 连接地址:</span>
          <span class="summary-value font-mono">{baseURL}</span>
        </div>
        <div class="summary-row">
          <span class="summary-label">Webhook 安全令牌:</span>
          <span class="summary-value font-mono">{secretToken ? '已配置 (••••••••)' : '未配置'}</span>
        </div>
        <div class="summary-row">
          <span class="summary-label">同步项目数量:</span>
          <span class="summary-value">{repoList.length} 个项目仓库</span>
        </div>

        {#if repoList.length > 0}
          <div class="summary-repos font-mono">
            {#each repoList as repo}
              <div class="summary-repo-item">
                <span>{repo.name} ({repo.path})</span>
                <span class="badge">ID: {repo.project_id}</span>
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
        <Button variant="primary" loading={saving} on:click={saveConfig}>
          保存并应用
        </Button>
      </div>
    </div>
  {/if}
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
    border-left: 4px solid #6366f1;
    padding: 12px 16px;
    border-radius: 0 8px 8px 0;
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

  .overview-header h4 {
    margin: 4px 0 6px 0;
    color: #f8fafc;
    font-size: 1.05rem;
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
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 10px;
  }

  .overview-row,
  .credential-collapsed {
    min-width: 0;
    background: rgba(15, 23, 42, 0.52);
    border: 1px solid rgba(51, 65, 85, 0.48);
    border-radius: 8px;
    padding: 12px;
  }

  .overview-row {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .overview-row span,
  .credential-collapsed span {
    color: #64748b;
    font-size: 0.72rem;
    font-weight: 700;
  }

  .overview-row strong,
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
    background: linear-gradient(135deg, #ffffff 0%, #cbd5e1 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
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
</style>
