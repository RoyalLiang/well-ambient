<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import TextInput from '../shared/TextInput.svelte';
  import Switch from '../shared/Switch.svelte';
  import Button from '../shared/Button.svelte';
  import Alert from '../shared/Alert.svelte';
  import { resetSettingsWorkspaceScroll } from '../../lib/settings-ui';

  const dispatch = createEventDispatcher();

  export let config: {
    enabled: boolean;
    base_url: string;
    username: string;
    api_token: string;
    sync_projects?: string[];
    sync_users?: string[];
    sync_statuses?: string[];
    custom_jql?: string;
  } = {
    enabled: false,
    base_url: '',
    username: '',
    api_token: '',
    sync_projects: [],
    sync_users: [],
    sync_statuses: [],
    custom_jql: ''
  };

  export let saving = false;
  export let saveError = '';
  export let saveSuccess = false;
  export let lastUpdated = '';

  let currentStep = 1;
  const steps = ['连接与凭证', '确认应用'];
  let editing = false;
  let showTokenEditor = !config.api_token;

  // Form states
  let enabled = config.enabled ?? false;
  let baseURL = config.base_url || '';
  let username = config.username || '';
  let apiToken = config.api_token || '';
  let syncProjects = (config.sync_projects || []).join(', ');
  let syncUsers = (config.sync_users || []).join(', ');
  let syncStatuses = (config.sync_statuses || []).join(', ');
  let customJQL = config.custom_jql || '';

  // Test states
  let testing = false;
  let testError = '';
  let testSuccess = '';
  let testDetails = '';
  $: isConfigured = enabled || !!(baseURL || username || apiToken || syncProjects || syncUsers || syncStatuses || customJQL);
  let configurationIssues: string[] = [];
  let healthTone = 'unchecked';
  let healthLabel = '未检测';
  let healthMessage = '尚未执行健康检查。';

  $: configurationIssues = enabled
    ? [
        ...(!baseURL ? ['Jira 基础 URL'] : []),
        ...(!apiToken ? ['API Token / PAT'] : [])
      ]
    : [];
  $: healthTone = saveSuccess
    ? 'success'
    : !isConfigured
      ? 'incomplete'
      : !enabled
        ? 'unchecked'
        : testing
          ? 'checking'
          : testError
            ? 'error'
            : configurationIssues.length > 0
              ? 'incomplete'
              : testSuccess
                ? 'success'
                : 'unchecked';
  $: healthLabel = saveSuccess
    ? '配置已保存'
    : !isConfigured
      ? '尚未配置'
      : !enabled
        ? '同步已停用'
        : testing
          ? '检测中'
          : testError
            ? '检测失败'
            : configurationIssues.length > 0
              ? '配置不完整'
              : testSuccess
                ? '检测通过'
                : '未检测';
  $: healthMessage = saveSuccess
    ? 'Jira 连接与同步范围已写入新的配置版本。'
    : !isConfigured
      ? '尚未录入 Jira 连接信息。进入编辑后完成基础配置。'
      : !enabled
        ? '配置已保留，Jira 双向同步当前不会运行。'
        : testing
          ? '正在验证 Jira 连接与认证凭证。'
          : testError
            ? testError
            : configurationIssues.length > 0
              ? `需要补齐：${configurationIssues.join('、')}。`
              : testSuccess
                ? testSuccess
                : '尚未执行健康检查，不默认判定为已连接。';
  $: if (!editing && !saveSuccess) {
    enabled = config.enabled ?? false;
    baseURL = config.base_url || '';
    username = config.username || '';
    apiToken = config.api_token || '';
    syncProjects = (config.sync_projects || []).join(', ');
    syncUsers = (config.sync_users || []).join(', ');
    syncStatuses = (config.sync_statuses || []).join(', ');
    customJQL = config.custom_jql || '';
    showTokenEditor = !config.api_token;
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

  async function testConnection() {
    if (!baseURL || !apiToken) {
      testError = '请填写连接地址与 API 令牌/PAT';
      return;
    }

    testing = true;
    testError = '';
    testSuccess = '';
    testDetails = '';

    try {
      const res = await fetch('/api/config/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: 'jira',
          jira: {
            enabled: enabled,
            base_url: baseURL,
            username: username,
            api_token: apiToken
          }
        })
      });

      if (!res.ok) throw new Error(`HTTP 错误: ${res.status}`);
      const data = await res.json();

      if (data.success) {
        testSuccess = 'Jira 连通性校验成功！';
        testDetails = data.details || '';
      } else {
        testError = data.message;
        testDetails = data.details || '';
      }
    } catch (e: any) {
      testError = '连通性测试请求失败，请检查网络或后端状态';
      testDetails = e.message;
    } finally {
      testing = false;
    }
  }

  async function saveConfig(isToggle = false) {
    saving = true;
    saveError = '';

    const updatedJira = {
      enabled,
      base_url: baseURL,
      username,
      api_token: apiToken,
      sync_projects: syncProjects.split(',').map(s => s.trim()).filter(Boolean),
      sync_users: syncUsers.split(',').map(s => s.trim()).filter(Boolean),
      sync_statuses: syncStatuses.split(',').map(s => s.trim()).filter(Boolean),
      custom_jql: customJQL
    };

    dispatch('save', {
      key: 'jira',
      data: updatedJira,
      isToggle: isToggle
    });
  }

  function nextStep() {
    if (currentStep === 1) {
      if (enabled && (!baseURL || !apiToken)) {
        testError = '启用 Jira 同步时，必须填写连接地址及 API 令牌/PAT';
        return;
      }
      currentStep = 2;
    }
  }

  function prevStep() {
    if (currentStep > 1) {
      currentStep -= 1;
    }
  }
</script>

<div class="scw-workbench">
  {#if saveSuccess}
    <Alert type="success" title="配置已保存" message="Jira 连接与同步范围已更新，版本审计会记录本次变更。" />
  {/if}

  {#if !editing && isConfigured}
    <section class="scw-overview" aria-label="Jira 配置状态">
      <header class="scw-header">
        <div>
          <span class="scw-kicker">任务源集成</span>
          <h4>Jira 配置状态</h4>
          <p>只读摘要集中展示连接、认证和同步范围，健康状态只来自真实检测或配置完整性判断。</p>
        </div>
        <div class="scw-toggle">
          <span>启用 Jira 同步</span>
          <Switch id="jira-overview-toggle" label="启用 Jira 同步" bind:checked={enabled} on:change={() => saveConfig(true)} />
        </div>
      </header>

      <div class="scw-status tone-{healthTone}">
        <div class="scw-status-main">
          <span>链路健康</span>
          <strong>{healthLabel}</strong>
        </div>
        <p>{healthMessage}</p>
      </div>

      <div class="scw-read-grid">
        <div class="scw-read-item">
          <span>Jira 基础 URL 地址</span>
          <strong class="font-mono">{baseURL || '-'}</strong>
        </div>
        <div class="scw-read-item">
          <span>用户邮箱 (Username)</span>
          <strong class="font-mono">{username || 'Token 认证'}</strong>
        </div>
        <div class="scw-read-item">
          <span>敏感凭证 (API Token)</span>
          <strong>{apiToken ? '已配置，已脱敏' : '未配置'}</strong>
        </div>
        <div class="scw-read-item">
          <span>最近更新时间</span>
          <strong>{formatUpdated(lastUpdated)}</strong>
        </div>
        <div class="scw-read-item">
          <span>同步项目 (Project Keys)</span>
          <strong class="font-mono">{syncProjects || '所有项目'}</strong>
        </div>
        <div class="scw-read-item">
          <span>同步成员 (Assignees)</span>
          <strong class="font-mono">{syncUsers || '所有成员'}</strong>
        </div>
        <div class="scw-read-item">
          <span>状态范围筛选</span>
          <strong class="font-mono">{syncStatuses || '所有状态'}</strong>
        </div>
      </div>

      {#if customJQL}
        <section class="scw-code-section">
          <div class="scw-section-head">
            <div class="scw-section-copy">
              <h5>自定义 JQL</h5>
              <p>该查询会覆盖上方项目、成员和状态筛选。</p>
            </div>
          </div>
          <code class="scw-code scw-mono">{customJQL}</code>
        </section>
      {/if}

      {#if testDetails}
        <pre class="scw-details scw-mono">{testDetails}</pre>
      {/if}

      <div class="scw-actions">
        <Button variant="secondary" loading={testing} on:click={testConnection}>健康检查</Button>
        <Button variant="primary" on:click={openEditor}>编辑配置</Button>
      </div>
    </section>
  {:else}
    <section class="scw-editor" aria-label="编辑 Jira 配置">
      <div class="scw-stepper" aria-label="配置步骤">
        {#each steps as step, index}
          {@const stepNum = index + 1}
          <button
            type="button"
            class:active={currentStep === stepNum}
            class:completed={currentStep > stepNum}
            on:click={() => currentStep = stepNum}
          >
            <span class="scw-step-index">{stepNum}</span>
            {step}
          </button>
        {/each}
      </div>

    {#if currentStep === 1}
      <div class="scw-step-body">
      <div class="scw-info">
        <h4>Jira API 连接与同步设置</h4>
        <p>通过配置 Jira 认证，well-ambient 可以在检测到分支提交中的任务 ID 时，自动查询关联的 Jira Issue 标题，并将同步状态写回 Jira 问题单中。</p>
      </div>

      <Switch
        id="jira-enabled"
        label="启用 Jira 双向集成同步"
        bind:checked={enabled}
      />

      {#if enabled}
        <div class="scw-form-stack">
          <TextInput
            id="jira-url"
            label="Jira 基础 URL 地址"
            placeholder="https://your-domain.atlassian.net"
            bind:value={baseURL}
            required={enabled}
            helperText="请输入 Jira 站点的根 URL 地址。"
          />

          <TextInput
            id="jira-username"
            label="Jira 用户邮箱 (Username / Email)"
            placeholder="example@yourdomain.com"
            bind:value={username}
            helperText="用于连接 API 的 Jira 账号邮箱。若使用个人访问令牌 (PAT) 进行自建 Jira 认证，请将此字段留空。"
          />

          {#if showTokenEditor}
            <TextInput
              id="jira-token"
              label="Jira API 令牌 / 个人访问令牌 (PAT)"
              placeholder="请输入 API Token 或 PAT"
              type="password"
              bind:value={apiToken}
              required={enabled}
              helperText="对于 Jira Cloud，请填写 API 令牌与对应邮箱；对于自建 Jira 服务，请填写个人访问令牌 (PAT) 并将邮箱留空。"
            />
          {:else}
            <div class="scw-credential">
              <div class="scw-credential-copy">
                <span>Jira API Token / PAT</span>
                <strong>已配置，当前默认脱敏折叠</strong>
              </div>
              <button type="button" on:click={() => showTokenEditor = true}>编辑凭证/高级配置</button>
            </div>
          {/if}

          <div class="scw-native-field">
            <label class="scw-native-label" for="jira-sync-projects">
              同步项目键列表 (Project Keys)
            </label>
            <textarea
              id="jira-sync-projects"
              class="scw-native-textarea"
              rows="3"
              placeholder="PROJ, TEAM"
              bind:value={syncProjects}
            ></textarea>
            <span class="scw-helper">需要同步的 Jira 项目键（Key），多个项目用逗号分隔。例如: PROJ, DEVS</span>
          </div>

          <TextInput
            id="jira-sync-users"
            label="分配的用户邮箱/用户名列表"
            placeholder="eddie@company.com, developer@company.com"
            bind:value={syncUsers}
            helperText="只同步指派给这些用户的任务/Bug，多个用户用逗号分隔。"
          />

          <TextInput
            id="jira-sync-statuses"
            label="同步的任务/Bug进度状态筛选"
            placeholder="To Do, In Progress, In Review, Done"
            bind:value={syncStatuses}
            helperText="需要过滤/同步的 Jira 状态，多个状态用逗号分隔。例如: To Do, In Progress, Done"
          />

          <div class="scw-native-field">
            <label class="scw-native-label" for="jira-custom-jql">
              自定义 JQL 筛选器 (覆盖上方所有过滤条件 - 高级)
            </label>
            <textarea
              id="jira-custom-jql"
              class="scw-native-textarea scw-mono"
              rows="4"
              placeholder="project = PROJ AND status = 'In Progress'"
              spellcheck="false"
              bind:value={customJQL}
            ></textarea>
            <span class="scw-helper">自定义 Jira 检索语句 (JQL)。填写后会直接用于同步拉取，并覆盖上方的项目、用户和状态筛选。</span>
          </div>

          {#if testSuccess}
            <Alert type="success" title="测试成功" message={testSuccess}>
              {#if testDetails}
                <pre class="scw-details scw-mono">{testDetails}</pre>
              {/if}
            </Alert>
          {/if}

          {#if testError}
            <Alert type="error" title="测试失败" message={testError}>
              {#if testDetails}
                <pre class="scw-details scw-mono">{testDetails}</pre>
              {/if}
            </Alert>
          {/if}

          <div class="scw-section-actions">
            <Button variant="secondary" loading={testing} on:click={testConnection}>
              测试 Jira 连接
            </Button>
          </div>
        </div>
      {/if}

      <div class="scw-actions">
        <Button variant="primary" on:click={nextStep}>
          下一步
        </Button>
      </div>
    </div>
  {:else if currentStep === 2}
    <div class="scw-step-body">
      <div class="scw-info">
        <h4>Jira 集成配置摘要</h4>
        <p>确认无误后点击下方按钮保存并应用配置：</p>
      </div>

      <div class="scw-summary">
        <div class="scw-summary-row">
          <span class="summary-label">Jira 同步状态:</span>
          <span class="summary-value">
            {#if enabled}
              <span class="text-success">已启用</span>
            {:else}
              <span class="text-muted">已禁用</span>
            {/if}
          </span>
        </div>
        {#if enabled}
          <div class="scw-summary-row">
            <span class="summary-label">Jira 连接地址:</span>
            <span class="summary-value font-mono">{baseURL}</span>
          </div>
          <div class="scw-summary-row">
            <span class="summary-label">认证用户名:</span>
            <span class="summary-value font-mono">{username || '(留空/Token认证)'}</span>
          </div>
          <div class="scw-summary-row">
            <span class="summary-label">同步项目:</span>
            <span class="summary-value font-mono">{syncProjects || '所有项目'}</span>
          </div>
          <div class="scw-summary-row">
            <span class="summary-label">指派用户:</span>
            <span class="summary-value font-mono">{syncUsers || '所有用户'}</span>
          </div>
          <div class="scw-summary-row">
            <span class="summary-label">进度筛选:</span>
            <span class="summary-value font-mono">{syncStatuses || '所有状态'}</span>
          </div>
          {#if customJQL}
            <div class="scw-summary-row">
              <span class="summary-label">自定义 JQL:</span>
              <span class="summary-value font-mono text-warning">{customJQL}</span>
            </div>
          {/if}
        {/if}
      </div>

      {#if saveError}
        <Alert type="error" title="保存失败" message={saveError} />
      {/if}

      <div class="scw-actions">
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
  .text-success {
    color: var(--wa-success, #04966f);
  }

  .text-muted {
    color: var(--wa-text-muted, #667789);
  }

  .text-warning {
    color: var(--wa-warning, #b66d00);
  }

  .font-mono {
    font-family: var(--wa-font-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
  }
</style>
