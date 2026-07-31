<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import TextInput from '../shared/TextInput.svelte';
  import Switch from '../shared/Switch.svelte';
  import Button from '../shared/Button.svelte';
  import Alert from '../shared/Alert.svelte';
  import { resetSettingsWorkspaceScroll } from '../../lib/settings-ui';

  const dispatch = createEventDispatcher();

  interface JiraVersionSource {
    project_key: string;
    project_name: string;
    version_url: string;
  }

  interface JiraVersionSourceForm extends JiraVersionSource {
    local_id: string;
  }

  interface JiraVersionParseState {
    projectKey: string;
    versionID: string;
    canonicalURL: string;
    error: string;
  }

  export let config: {
    enabled: boolean;
    base_url: string;
    username: string;
    api_token: string;
    sync_projects?: string[];
    sync_users?: string[];
    sync_statuses?: string[];
    custom_jql?: string;
    version_sources?: JiraVersionSource[];
  } = {
    enabled: false,
    base_url: '',
    username: '',
    api_token: '',
    sync_projects: [],
    sync_users: [],
    sync_statuses: [],
    custom_jql: '',
    version_sources: []
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
  let versionSourceSequence = 0;
  let versionSources = createVersionSourceForms(config.version_sources || []);
  let versionSourceValidationMessage = '';

  // Test states
  let testing = false;
  let testError = '';
  let testSuccess = '';
  let testDetails = '';
  $: isConfigured = enabled || !!(baseURL || username || apiToken || syncProjects || syncUsers || syncStatuses || customJQL || versionSources.length);
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
    versionSources = createVersionSourceForms(config.version_sources || []);
    versionSourceValidationMessage = '';
    showTokenEditor = !config.api_token;
  }

  function createVersionSourceForms(sources: JiraVersionSource[]): JiraVersionSourceForm[] {
    return sources.map(source => ({
      project_key: source.project_key || '',
      project_name: source.project_name || '',
      version_url: source.version_url || '',
      local_id: `jira-version-source-${++versionSourceSequence}`
    }));
  }

  function parseVersionURL(rawURL: string): JiraVersionParseState {
    const empty = { projectKey: '', versionID: '', canonicalURL: '', error: '' };
    const site = baseURL.trim();
    const raw = rawURL.trim();
    if (!raw) return { ...empty, error: '请输入 Jira 版本链接' };
    if (!site) return { ...empty, error: '请先填写 Jira 基础 URL' };

    try {
      const base = new URL(site);
      if (!['http:', 'https:'].includes(base.protocol)) {
        return { ...empty, error: 'Jira 基础 URL 仅支持 HTTP 或 HTTPS' };
      }
      const resolveBase = new URL(base.toString());
      if (!resolveBase.pathname.endsWith('/')) resolveBase.pathname += '/';
      const candidate = new URL(raw, resolveBase);
      if (candidate.origin !== base.origin) {
        return { ...empty, error: '版本链接必须属于已配置的 Jira 站点' };
      }

      const basePath = base.pathname.replace(/\/$/, '');
      let relativePath = candidate.pathname;
      if (basePath) {
        if (!relativePath.startsWith(`${basePath}/`)) {
          return { ...empty, error: '版本链接不在 Jira 基础 URL 的路径下' };
        }
        relativePath = relativePath.slice(basePath.length);
      }
      const segments = relativePath.split('/').filter(Boolean);
      if (segments.length !== 4 || segments[0] !== 'projects' || segments[2] !== 'versions') {
        return { ...empty, error: '链接格式应为 /projects/{项目号}/versions/{版本ID}' };
      }
      const projectKey = decodeURIComponent(segments[1]).toUpperCase();
      const versionID = decodeURIComponent(segments[3]);
      if (!/^[A-Z][A-Z0-9_]*$/.test(projectKey)) {
        return { ...empty, error: '链接中的 Jira 项目号无效' };
      }
      if (!/^[0-9]+$/.test(versionID) || Number(versionID) <= 0) {
        return { ...empty, error: '链接中的 Jira 版本 ID 无效' };
      }
      return {
        projectKey,
        versionID,
        canonicalURL: `${candidate.origin}${candidate.pathname}`,
        error: ''
      };
    } catch {
      return { ...empty, error: 'Jira 版本链接无效' };
    }
  }

  function getVersionSourceState(source: JiraVersionSourceForm): JiraVersionParseState {
    const hasAnyValue = !!(source.project_key.trim() || source.project_name.trim() || source.version_url.trim());
    if (!hasAnyValue) return { projectKey: '', versionID: '', canonicalURL: '', error: '' };
    const parsed = parseVersionURL(source.version_url);
    if (parsed.error) return parsed;
    if (!source.project_name.trim()) {
      return { ...parsed, error: '请输入项目名称' };
    }
    if (source.project_key.trim() && source.project_key.trim().toUpperCase() !== parsed.projectKey) {
      return { ...parsed, error: `项目号应为 ${parsed.projectKey}` };
    }
    if (!source.project_key.trim()) {
      return { ...parsed, error: '请输入项目号，或使用链接中识别出的项目号' };
    }
    return parsed;
  }

  function addVersionSource() {
    versionSources = [
      ...versionSources,
      {
        project_key: '',
        project_name: '',
        version_url: '',
        local_id: `jira-version-source-${++versionSourceSequence}`
      }
    ];
    versionSourceValidationMessage = '';
  }

  function removeVersionSource(index: number) {
    versionSources = versionSources.filter((_, sourceIndex) => sourceIndex !== index);
    versionSourceValidationMessage = '';
  }

  function useParsedProjectKey(index: number) {
    const source = versionSources[index];
    if (!source) return;
    const parsed = parseVersionURL(source.version_url);
    if (parsed.error || !parsed.projectKey) return;
    versionSources[index].project_key = parsed.projectKey;
    versionSources = [...versionSources];
  }

  function validateVersionSources() {
    for (let index = 0; index < versionSources.length; index += 1) {
      const source = versionSources[index];
      if (!source.project_key.trim() && !source.project_name.trim() && !source.version_url.trim()) {
        versionSourceValidationMessage = `第 ${index + 1} 个版本来源：请填写项目号、项目名称与 Jira 版本链接`;
        return false;
      }
      const state = getVersionSourceState(versionSources[index]);
      if (state.error) {
        versionSourceValidationMessage = `第 ${index + 1} 个版本来源：${state.error}`;
        return false;
      }
    }
    const identities = new Set<string>();
    for (let index = 0; index < versionSources.length; index += 1) {
      const state = getVersionSourceState(versionSources[index]);
      const identity = `${state.projectKey}:${state.versionID}`;
      if (identities.has(identity)) {
        versionSourceValidationMessage = `第 ${index + 1} 个版本来源与前面的配置重复`;
        return false;
      }
      identities.add(identity);
    }
    versionSourceValidationMessage = '';
    return true;
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
    if (!validateVersionSources()) return;
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
      custom_jql: customJQL,
      version_sources: versionSources.map(source => ({
        project_key: source.project_key.trim().toUpperCase(),
        project_name: source.project_name.trim(),
        version_url: parseVersionURL(source.version_url).canonicalURL || source.version_url.trim()
      }))
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
      if (!validateVersionSources()) return;
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

      {#if versionSources.length > 0}
        <section class="jira-version-overview" aria-labelledby="jira-version-overview-title">
          <div class="scw-section-head">
            <div class="scw-section-copy">
              <h5 id="jira-version-overview-title">版本链接来源</h5>
              <p>这些版本中的 Jira 会作为额外来源加入同步范围。</p>
            </div>
            <strong class="jira-version-count">{versionSources.length} 个版本</strong>
          </div>
          <dl class="jira-version-read-list">
            {#each versionSources as source}
              {@const sourceState = getVersionSourceState(source)}
              <div>
                <dt><span class="font-mono">{source.project_key}</span> {source.project_name}</dt>
                <dd>
                  <a href={source.version_url} target="_blank" rel="noopener noreferrer">版本 {sourceState.versionID || '-'}</a>
                  <span class="font-mono">{source.version_url}</span>
                </dd>
              </div>
            {/each}
          </dl>
        </section>
      {/if}

      {#if customJQL}
        <section class="scw-code-section">
          <div class="scw-section-head">
            <div class="scw-section-copy">
              <h5>自定义 JQL</h5>
              <p>该查询会覆盖普通项目、成员和状态筛选；版本链接来源仍会追加。</p>
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

          <section class="jira-version-editor" aria-labelledby="jira-version-editor-title">
            <div class="jira-version-editor-head">
              <div>
                <h5 id="jira-version-editor-title">Jira 版本链接来源</h5>
                <p>配置项目版本页后，该版本中的全部 Jira 会额外加入同步。链接只解析为 JQL，不抓取页面内容。</p>
              </div>
              <Button variant="secondary" size="small" on:click={addVersionSource}>添加项目版本</Button>
            </div>

            {#if versionSources.length === 0}
              <p class="jira-version-empty">尚未配置版本链接。普通项目、成员、状态和自定义 JQL 的行为保持不变。</p>
            {:else}
              <div class="jira-version-source-list">
                {#each versionSources as source, index (source.local_id)}
                  {@const sourceState = getVersionSourceState(source)}
                  <fieldset class="jira-version-source">
                    <legend>版本来源 {index + 1}</legend>
                    <div class="jira-version-field">
                      <label for={`${source.local_id}-key`}>项目号</label>
                      <input
                        id={`${source.local_id}-key`}
                        class="jira-version-input font-mono"
                        class:invalid={!!sourceState.error && sourceState.projectKey !== source.project_key.trim().toUpperCase()}
                        placeholder="PRJ25024"
                        bind:value={source.project_key}
                        aria-invalid={sourceState.error ? 'true' : undefined}
                        aria-describedby={`${source.local_id}-status`}
                      />
                      {#if sourceState.projectKey && !source.project_key.trim()}
                        <button class="jira-use-parsed" type="button" on:click={() => useParsedProjectKey(index)}>使用 {sourceState.projectKey}</button>
                      {/if}
                    </div>
                    <div class="jira-version-field">
                      <label for={`${source.local_id}-name`}>项目名称</label>
                      <input
                        id={`${source.local_id}-name`}
                        class="jira-version-input"
                        placeholder="ReeWell 版本发布"
                        bind:value={source.project_name}
                        aria-invalid={sourceState.error ? 'true' : undefined}
                        aria-describedby={`${source.local_id}-status`}
                      />
                    </div>
                    <div class="jira-version-field jira-version-url-field">
                      <label for={`${source.local_id}-url`}>Jira 版本链接</label>
                      <input
                        id={`${source.local_id}-url`}
                        class="jira-version-input font-mono"
                        class:invalid={!!sourceState.error && !!source.version_url.trim()}
                        type="url"
                        placeholder="https://jira.example.com/projects/PRJ25024/versions/13622"
                        bind:value={source.version_url}
                        aria-invalid={sourceState.error ? 'true' : undefined}
                        aria-describedby={`${source.local_id}-status`}
                      />
                    </div>
                    <button class="jira-version-remove" type="button" on:click={() => removeVersionSource(index)} aria-label={`移除版本来源 ${index + 1}`}>
                      移除
                    </button>
                    <p id={`${source.local_id}-status`} class:error={!!sourceState.error} class="jira-version-status">
                      {#if sourceState.error}
                        {sourceState.error}
                      {:else if sourceState.versionID}
                        已识别 {sourceState.projectKey}，版本 ID {sourceState.versionID}
                      {:else}
                        填写项目号、项目名称和 Jira 版本链接。
                      {/if}
                    </p>
                  </fieldset>
                {/each}
              </div>
            {/if}

            {#if versionSourceValidationMessage}
              <Alert type="error" title="版本来源配置无效" message={versionSourceValidationMessage} />
            {/if}
          </section>

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
              自定义 JQL 筛选器 (覆盖普通过滤条件 - 高级)
            </label>
            <textarea
              id="jira-custom-jql"
              class="scw-native-textarea scw-mono"
              rows="4"
              placeholder="project = PROJ AND status = 'In Progress'"
              spellcheck="false"
              bind:value={customJQL}
            ></textarea>
            <span class="scw-helper">自定义 Jira 检索语句 (JQL)。填写后会覆盖普通项目、用户和状态筛选；上方版本链接来源仍会作为额外范围追加。</span>
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
          <div class="scw-summary-row wide">
            <span class="summary-label">版本链接来源:</span>
            <span class="summary-value">
              {#if versionSources.length === 0}
                未配置
              {:else}
                {versionSources.map(source => `${source.project_key} ${source.project_name}`).join('；')}
              {/if}
            </span>
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

  .jira-version-editor,
  .jira-version-overview {
    min-width: 0;
    display: grid;
    gap: 12px;
    padding: 16px 0;
    border-top: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
    border-bottom: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
  }

  .jira-version-editor-head {
    min-width: 0;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
  }

  .jira-version-editor-head > div {
    min-width: 0;
  }

  .jira-version-editor h5,
  .jira-version-overview h5 {
    margin: 0;
    color: var(--wa-text-strong, #0d1722);
    font-size: 14px;
    line-height: 1.4;
    text-wrap: balance;
  }

  .jira-version-editor-head p,
  .jira-version-empty {
    max-width: 76ch;
    margin: 4px 0 0;
    color: var(--wa-text-muted, #667789);
    font-size: 12px;
    line-height: 1.55;
    text-wrap: pretty;
  }

  .jira-version-empty {
    margin: 0;
    padding: 12px 0;
  }

  .jira-version-source-list,
  .jira-version-read-list {
    min-width: 0;
    display: grid;
    margin: 0;
  }

  .jira-version-source {
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(132px, 0.72fr) minmax(168px, 1fr) minmax(280px, 1.8fr) auto;
    gap: 8px 12px;
    align-items: end;
    margin: 0;
    padding: 16px 0;
    border: 0;
  }

  .jira-version-source + .jira-version-source {
    border-top: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
  }

  .jira-version-source legend {
    grid-column: 1 / -1;
    margin: 0;
    padding: 0;
    color: var(--wa-text-main, #293847);
    font-size: 12px;
    font-weight: 760;
  }

  .jira-version-field {
    position: relative;
    min-width: 0;
    display: grid;
    gap: 7px;
  }

  .jira-version-field label {
    color: var(--wa-text-main, #293847);
    font-size: 12px;
    font-weight: 700;
  }

  .jira-version-input {
    width: 100%;
    min-width: 0;
    min-height: var(--wa-control-h, 36px);
    padding: 0 11px;
    border: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
    border-radius: var(--wa-radius-md, 7px);
    outline: none;
    background: rgba(255, 255, 255, 0.84);
    color: var(--wa-text-main, #293847);
    font: inherit;
    font-size: 12px;
    box-sizing: border-box;
    transition: border-color var(--wa-duration-fast, 140ms) var(--wa-ease, ease), box-shadow var(--wa-duration-fast, 140ms) var(--wa-ease, ease), background var(--wa-duration-fast, 140ms) var(--wa-ease, ease);
  }

  .jira-version-input::placeholder {
    color: var(--wa-text-muted, #667789);
    opacity: 1;
  }

  .jira-version-input:hover {
    border-color: var(--wa-border-strong, rgba(85, 106, 128, 0.32));
  }

  .jira-version-input:focus {
    border-color: var(--wa-border-focus, rgba(0, 143, 150, 0.86));
    background: rgba(255, 255, 255, 0.96);
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.1);
  }

  .jira-version-input.invalid {
    border-color: rgba(200, 22, 29, 0.46);
  }

  .jira-use-parsed {
    position: absolute;
    right: 6px;
    bottom: 5px;
    min-height: 26px;
    padding: 0 7px;
    border: 0;
    border-radius: var(--wa-radius-sm, 6px);
    background: var(--wa-accent-soft, rgba(0, 143, 150, 0.12));
    color: var(--wa-accent-strong, #006f76);
    font-size: 10px;
    font-weight: 760;
    cursor: pointer;
  }

  .jira-use-parsed:focus-visible,
  .jira-version-remove:focus-visible {
    outline: 2px solid rgba(0, 143, 150, 0.28);
    outline-offset: 2px;
  }

  .jira-version-remove {
    min-height: var(--wa-control-h, 36px);
    padding: 0 10px;
    border: 1px solid rgba(200, 22, 29, 0.2);
    border-radius: var(--wa-radius-md, 7px);
    background: rgba(200, 22, 29, 0.06);
    color: var(--wa-danger, #c8161d);
    font: inherit;
    font-size: 12px;
    font-weight: 760;
    cursor: pointer;
  }

  .jira-version-remove:hover {
    border-color: rgba(200, 22, 29, 0.34);
    background: rgba(200, 22, 29, 0.1);
  }

  .jira-version-status {
    grid-column: 1 / -1;
    margin: 0;
    color: var(--wa-success, #04966f);
    font-size: 11px;
    line-height: 1.45;
  }

  .jira-version-status.error {
    color: var(--wa-danger, #c8161d);
  }

  .jira-version-count {
    flex: none;
    color: var(--wa-accent-strong, #006f76);
    font-size: 12px;
    font-variant-numeric: tabular-nums;
  }

  .jira-version-read-list > div {
    min-width: 0;
    display: grid;
    grid-template-columns: minmax(180px, 0.8fr) minmax(0, 1.5fr);
    gap: 16px;
    padding: 10px 0;
  }

  .jira-version-read-list > div + div {
    border-top: 1px solid var(--wa-border-soft, rgba(123, 143, 160, 0.18));
  }

  .jira-version-read-list dt,
  .jira-version-read-list dd {
    min-width: 0;
    margin: 0;
  }

  .jira-version-read-list dt {
    color: var(--wa-text-strong, #0d1722);
    font-size: 12px;
    font-weight: 760;
  }

  .jira-version-read-list dd {
    display: grid;
    gap: 3px;
    color: var(--wa-text-muted, #667789);
    font-size: 11px;
  }

  .jira-version-read-list a {
    width: fit-content;
    color: var(--wa-accent-strong, #006f76);
    font-weight: 760;
    text-underline-offset: 3px;
  }

  .jira-version-read-list dd span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  @media (max-width: 960px) {
    .jira-version-source {
      grid-template-columns: minmax(132px, 0.72fr) minmax(168px, 1fr) auto;
    }

    .jira-version-url-field {
      grid-column: 1 / 3;
    }
  }

  @media (max-width: 760px) {
    .jira-version-editor-head {
      align-items: stretch;
      flex-direction: column;
    }

    .jira-version-source {
      grid-template-columns: minmax(0, 1fr);
      gap: 12px;
    }

    .jira-version-source legend,
    .jira-version-url-field,
    .jira-version-status {
      grid-column: 1;
    }

    .jira-version-input,
    .jira-version-remove {
      min-height: 44px;
    }

    .jira-use-parsed {
      min-height: 32px;
      bottom: 6px;
    }

    .jira-version-read-list > div {
      grid-template-columns: minmax(0, 1fr);
      gap: 5px;
    }

    .jira-version-read-list dd span {
      white-space: normal;
      overflow-wrap: anywhere;
    }
  }
</style>
