<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import Steps from '../shared/Steps.svelte';
  import TextInput from '../shared/TextInput.svelte';
  import Button from '../shared/Button.svelte';
  import Alert from '../shared/Alert.svelte';

  const dispatch = createEventDispatcher();

  export let config: {
    base_url: string;
    secret_token: string;
    api_token?: string;
    repos: Array<{ name: string; path: string; project_id: string }>;
  } = { base_url: '', secret_token: '', api_token: '', repos: [] };

  export let saving = false;
  export let saveError = '';
  export let saveSuccess = false;

  let currentStep = 1;
  const steps = ['连接地址', '项目仓库映射', '完成应用'];

  // Step 1 states
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

  async function saveConfig() {
    const updatedGitLab = {
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
      <div class="success-actions">
        <Button variant="primary" on:click={() => dispatch('close')}>
          完成并关闭
        </Button>
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

      <TextInput
        id="gitlab-api-token"
        label="GitLab API 访问令牌 (Personal Access Token)"
        placeholder="输入用于自动拉取仓库列表的 Private Token"
        type="password"
        bind:value={apiToken}
        helperText="可选。若需支持在下一步中自动拉取并勾选仓库项目，请输入具有 read_api 权限的 Personal Access Token。"
      />

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
    color: #94a3b8;
  }

  .badge {
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
