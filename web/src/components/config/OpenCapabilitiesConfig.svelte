<script lang="ts">
  import { onMount } from 'svelte';
  import { showToast } from '../../lib/toast';
  import { copyToClipboard } from '../../lib/clipboard';
  import { buildCodexMcpConfig, buildOpenCapabilityInstall, OPEN_API_KEY_PLACEHOLDER } from '../../lib/openCapabilityInstall';
  import { prefixError } from '../../lib/openSkillRegistry';
  import Button from '../shared/Button.svelte';
  import Switch from '../shared/Switch.svelte';
  import Modal from '../shared/Modal.svelte';
  import Select from '../shared/Select.svelte';

  export let canWrite = false;
  export let onOpenRegistry: () => void = () => {};

  const ttlOptions = [
    { value: '0', label: '永久有效 (Permanent)' },
    { value: '168', label: '7 天 (168h)' },
    { value: '720', label: '30 天 (720h - 默认推荐)' },
    { value: '2160', label: '90 天 (2160h)' },
    { value: '8760', label: '1 年 (8760h)' }
  ];

  interface OpenCapabilitiesConfigState {
    name_prefix: string;
    enabled: boolean;
    read_enabled: boolean;
    prepare_enabled: boolean;
    execute_enabled: boolean;
  }

  interface OpenCapabilitySource {
    id: string;
    name: string;
    owner: string;
    quota_profile: string;
    created_at: string;
  }

  interface OpenCapabilityCredential {
    key_id: string;
    prefix: string;
    source_id: string;
    status: string;
    expires_at?: string;
    last_used_at?: string;
    created_at: string;
  }

  interface OpenCapabilityBinding {
    project_ref: string;
    action_class: string;
    connector_ref: string;
    executor_ref: string;
    status: string;
  }

  interface OpenSkillItem {
    name: string;
    version: string;
    title: string;
    description: string;
    tools: string[];
    archive_url: string;
    manifest_url: string;
    doc_url: string;
  }

  interface OverviewData {
    config: OpenCapabilitiesConfigState;
    version: number;
    active_features: {
      Read: boolean;
      Prepare: boolean;
      Execute: boolean;
    };
    base_url: string;
    mcp: {
      endpoint_url: string;
      server_name: string;
      server_version: string;
      tools: string[];
    };
    policy?: {
      version: number;
      digest: string;
      allowed_projects: string[];
      allowed_repositories: string[];
      actions: string[];
    };
    sources: OpenCapabilitySource[];
    credentials: OpenCapabilityCredential[];
    bindings: OpenCapabilityBinding[];
    skills: OpenSkillItem[];
  }

  let loading = true;
  let saving = false;
  let error = '';
  let overview: OverviewData | null = null;

  // Local draft of switches
  let draftConfig: OpenCapabilitiesConfigState = {
    name_prefix: 'well-ambient',
    enabled: false,
    read_enabled: false,
    prepare_enabled: false,
    execute_enabled: false
  };

  let activeConfigTab: 'codex' | 'cursor' | 'claude' | 'windsurf' | 'cline' = 'codex';

  // Issue Key Modal State
  let showIssueModal = false;
  let newKeySourceID = 'target-agent';
  let newKeyTTLHours = 720;
  let issuingKey = false;
  let issueError = '';
  let issuedKeyResult: { key_id: string; key: string; expires_at: string } | null = null;

  // Revoking key
  let revokingKeyID = '';

  $: isDirty = overview ? (
    draftConfig.enabled !== overview.config.enabled ||
    draftConfig.read_enabled !== overview.config.read_enabled ||
    draftConfig.prepare_enabled !== overview.config.prepare_enabled ||
    draftConfig.execute_enabled !== overview.config.execute_enabled ||
    draftConfig.name_prefix.trim() !== overview.config.name_prefix
  ) : false;

  $: namePrefixError = prefixError(draftConfig.name_prefix);
  $: prefixPreview = draftConfig.name_prefix.trim() || 'well-ambient';
  $: activeRuntimeStages = [
    { name: 'Read', label: '只读', enabled: overview?.active_features.Read },
    { name: 'Prepare', label: '预备', enabled: overview?.active_features.Prepare },
    { name: 'Execute', label: '执行', enabled: overview?.active_features.Execute }
  ].filter(stage => stage.enabled);
  $: effectiveOpenEnabled = activeRuntimeStages.length > 0;

  $: currentBaseURL = overview?.base_url || (typeof window !== 'undefined' ? window.location.origin : '');
  $: installation = buildOpenCapabilityInstall(currentBaseURL, overview?.mcp?.endpoint_url || '');
  $: currentMcpURL = installation.mcpURL;
  $: cursorDeepLink = installation.cursorDeepLink;
  $: installCommand = installation.installCommand;
  $: codexConfig = buildCodexMcpConfig(currentMcpURL);
  $: codexInstallCommand = installCommand ? `${installCommand} -s -- --root "$HOME/.codex" --mcp` : '';
  const effectiveToken = OPEN_API_KEY_PLACEHOLDER;

  function openCodexCredentialModal() {
    if (!canWrite) return;
    newKeySourceID = 'codex-dev';
    issueError = '';
    issuedKeyResult = null;
    showIssueModal = true;
  }

  async function api(path: string, init: RequestInit = {}) {
    const headers = new Headers(init.headers || {});
    const token = typeof localStorage !== 'undefined' ? localStorage.getItem('jwt_token') : null;
    if (token) headers.set('Authorization', `Bearer ${token}`);
    const res = await fetch(path, { ...init, headers });
    if (!res.ok) {
      let message = `HTTP ${res.status}`;
      try {
        const payload = await res.json();
        message = payload.error?.message || payload.error || payload.message || message;
      } catch {}
      throw new Error(message);
    }
    return res.json();
  }

  async function loadOverview() {
    loading = true;
    error = '';
    try {
      const data: OverviewData = await api('/api/ai-governance/open-capabilities/overview');
      overview = data;
      draftConfig = { ...data.config };
    } catch (err: any) {
      error = err.message || '加载开放能力概览失败';
    } finally {
      loading = false;
    }
  }

  async function saveConfig() {
    if (namePrefixError) return;
    if (!overview || !canWrite) return;
    saving = true;
    error = '';
    try {
      await api('/api/ai-governance/open-capabilities/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ...draftConfig,
          expected_version: overview.version
        })
      });
      showToast('开放能力配置已成功更新并动态生效！', { type: 'success' });
      await loadOverview();
    } catch (err: any) {
      error = err.message || '更新配置失败';
      showToast(`保存失败: ${error}`, { type: 'error' });
    } finally {
      saving = false;
    }
  }

  async function issueNewKey() {
    if (!canWrite) return;
    issuingKey = true;
    issueError = '';
    error = '';
    try {
      const res = await api('/api/ai-governance/open-capabilities/credentials', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          source_id: newKeySourceID,
          ttl_hours: Number(newKeyTTLHours)
        })
      });
      issuedKeyResult = res;
      showToast('API Key 已签发，请复制保存并填写到客户端 Authorization 字段。', { type: 'success' });
      await loadOverview();
    } catch (err: any) {
      issueError = err.message || '签发 Key 失败';
      showToast(issueError, { type: 'error' });
    } finally {
      issuingKey = false;
    }
  }

  async function revokeKey(keyID: string) {
    if (!canWrite || !confirm(`确定撤销凭证 ${keyID} 吗？撤销后该客户端将立即无法访问。`)) return;
    revokingKeyID = keyID;
    try {
      await api(`/api/ai-governance/open-capabilities/credentials/${encodeURIComponent(keyID)}/revoke`, {
        method: 'POST'
      });
      showToast('凭证已成功撤销', { type: 'success' });
      await loadOverview();
    } catch (err: any) {
      showToast(err.message || '撤销凭证失败', { type: 'error' });
    } finally {
      revokingKeyID = '';
    }
  }

  let manualCopy: { text: string; label: string } | null = null;
  let issuedManualCopy: { text: string; label: string } | null = null;

  async function copyText(text: string, label = '内容') {
    if (await copyToClipboard(text)) {
      issuedManualCopy = null;
      showToast(`${label}已复制到剪贴板`, { type: 'success' });
      return;
    }
    if (showIssueModal) issuedManualCopy = { text, label };
    else manualCopy = { text, label };
  }

  function selectCopyText(event: FocusEvent) {
    (event.currentTarget as HTMLTextAreaElement).select();
  }

  function closeIssueModal() {
    if (issuingKey) return;
    showIssueModal = false;
    issuedKeyResult = null;
    issuedManualCopy = null;
  }

  onMount(() => {
    loadOverview();
  });
</script>

<div class="open-capabilities-view" role="region" aria-label="开放能力与 MCP 集成管理">
  {#if loading}
    <div class="gov-loading-pane" role="status" aria-live="polite">
      <span class="spinner" aria-hidden="true"></span>
      <p>正在拉取开放能力配置与服务概览…</p>
    </div>
  {:else if error && !overview}
    <div class="gov-notice error" role="alert">
      <span><strong>加载异常：</strong> {error}</span>
      <Button variant="ghost" size="small" on:click={loadOverview}>重试</Button>
    </div>
  {:else if overview}
    <!-- Top Header: Identity & Master Status -->
    <div class="open-gov-header">
      <div class="header-main">
        <div class="header-title-row">
          <span class="status-pill" class:active={effectiveOpenEnabled} class:inactive={!effectiveOpenEnabled}>
            {effectiveOpenEnabled ? '运行能力已启用' : '未开放运行阶段'}
          </span>
          {#if overview.active_features.Execute}
            <span class="status-pill warning" title="执行写入已开启，旧版页面直接写入已被全局互斥拦截">
              决策写入互斥生效中
            </span>
          {/if}
        </div>
        <p class="header-desc">
          管理能力开关、客户端接入、集成凭证与 Jira 执行绑定。
        </p>
      </div>

      <div class="header-actions">
        <Button variant="secondary" size="small" on:click={loadOverview}>
          刷新概览
        </Button>
      </div>
    </div>

    {#if !overview.config.enabled && effectiveOpenEnabled}
      <div class="gov-notice info" role="status">
        配置总开关已关闭，但部署环境变量仍启用：{activeRuntimeStages.map(stage => stage.name).join('、')}。运行状态以上述生效能力为准。
      </div>
    {/if}

    <!-- Top Key Metrics Strip -->
    <div class="gov-metrics-strip" aria-label="开放能力运行指标">
      <div class="gov-metric-item">
        <span class="gov-metric-label">当前运行阶段</span>
        <div class="gov-metric-val">
          {#if effectiveOpenEnabled}
            <strong class="highlight-cyan">{activeRuntimeStages.map(stage => stage.label).join(' / ')}</strong>
            <span class="gov-metric-sub">{activeRuntimeStages.map(stage => stage.name).join(' / ')} 已生效</span>
          {:else}
            <strong class="highlight-gray">未开放运行阶段</strong>
            <span class="gov-metric-sub">Read、Prepare、Execute 均未生效</span>
          {/if}
        </div>
      </div>

      <div class="gov-metric-divider" aria-hidden="true"></div>

      <div class="gov-metric-item">
        <span class="gov-metric-label">MCP 工具矩阵</span>
        <div class="gov-metric-val">
          <strong>{overview.mcp.tools.length} 项工具</strong>
          <span class="gov-metric-sub">{overview.mcp.server_name}</span>
        </div>
      </div>

      <div class="gov-metric-divider" aria-hidden="true"></div>

      <div class="gov-metric-item">
        <span class="gov-metric-label">标记启用的 Key</span>
        <div class="gov-metric-val">
          <strong>{(overview.credentials || []).filter(c => c.status === 'active').length} 个</strong>
          <span class="gov-metric-sub">有效性同时取决于到期时间与来源状态</span>
        </div>
      </div>

      <div class="gov-metric-divider" aria-hidden="true"></div>

      <div class="gov-metric-item">
        <span class="gov-metric-label">安全发布策略</span>
        <div class="gov-metric-val">
          {#if overview.policy}
            <strong class="highlight-green">v{overview.policy.version} 活跃</strong>
            <span class="gov-metric-sub">{overview.policy.allowed_projects.length} 项目 / {overview.policy.allowed_repositories.length} 仓库</span>
          {:else}
            <strong class="highlight-gray">未激活</strong>
            <span class="gov-metric-sub">需发布策略后放行</span>
          {/if}
        </div>
      </div>
    </div>

    <!-- Section 1: Dynamic Feature Switches -->
    <section class="gov-card" aria-labelledby="switches-section-title">
      <div class="card-header">
        <div class="title-wrap">
          <h4 id="switches-section-title">分阶段能力动态开关 (Runtime Feature Switches)</h4>
          <span class="sub-hint">脱离离线环境变量与文件，变更即时广播至全集群实例，无需重启进程。</span>
        </div>
        {#if canWrite}
          <div class="save-bar">
            {#if isDirty}
              <Button
                variant="primary"
                size="small"
                loading={saving}
                disabled={!!namePrefixError}
                on:click={saveConfig}
              >
                保存并应用动态配置*
              </Button>
            {:else}
              <span class="synced-tag" title="当前配置与数据库生效版本完全一致">✓ 配置已同步</span>
            {/if}
          </div>
        {/if}
      </div>

      {#if !canWrite}
        <div class="gov-notice info">
          当前账户具有查看权限；修改运行时开关或签发 API Key 需要管理员 <code>config:write</code> 权限。
        </div>
      {/if}

      <div class="name-prefix-setting">
    <div class="name-prefix-field">
      <strong>统一主技能 well-ambient</strong>
      <p id="name-prefix-help">主技能、连接名与工具命名空间统一为 well-ambient。四个业务子技能在“Skill 中心”按需加载、统一安装并查看调用观测。</p>
    </div>
    <div id="name-prefix-preview" class="name-prefix-preview"><span>对外输出</span><code>well-ambient</code><code>well-ambient-jira_search_issues</code></div>
  </div>

  <div class="switches-container">
        <!-- Global Master Switch -->
        <div class="master-switch-row" class:is-active={draftConfig.enabled}>
          <div class="switch-content">
            <div class="switch-heading">
              <span class="switch-name">全局开放能力主控</span>
              <span class="switch-badge">{draftConfig.enabled ? '已开启' : '已关闭'}</span>
            </div>
          <p class="switch-desc">配置优先控制运行阶段；关闭后，部署环境变量可能继续启用对应能力。以上方实际运行状态为准。</p>
          </div>
          <div class="switch-action">
            <Switch
              label="开放能力总开关"
              expandedHitArea
              checked={draftConfig.enabled}
              disabled={!canWrite}
              on:change={(e) => draftConfig.enabled = e.detail}
            />
          </div>
        </div>

        <!-- 3 Phase Sub-switches Grid -->
        <div class="phases-grid">
          <!-- Read Phase -->
          <div class="phase-card" class:is-disabled={!draftConfig.enabled}>
            <div class="phase-card-head">
              <span class="phase-title">只读分析能力 (Read Phase)</span>
              <Switch
                label="只读分析能力"
                expandedHitArea
                checked={draftConfig.read_enabled}
                disabled={!canWrite || !draftConfig.enabled}
                on:change={(e) => draftConfig.read_enabled = e.detail}
              />
            </div>
            <p class="phase-desc">
              Jira 权威投影多维检索/聚合、字段规约感知与 GitLab 代码评审过滤读取。
            </p>
          </div>

          <!-- Prepare Phase -->
          <div class="phase-card" class:is-disabled={!draftConfig.enabled}>
            <div class="phase-card-head">
              <span class="phase-title">决策预备能力 (Prepare Phase)</span>
              <Switch
                label="决策预备能力"
                expandedHitArea
                checked={draftConfig.prepare_enabled}
                disabled={!canWrite || !draftConfig.enabled}
                on:change={(e) => draftConfig.prepare_enabled = e.detail}
              />
            </div>
            <p class="phase-desc">
              冻结带远端前置校验与时间戳的单事项决策执行计划，生成确定性 Plan ID。
            </p>
          </div>

          <!-- Execute Phase -->
          <div class="phase-card is-execute" class:is-disabled={!draftConfig.enabled}>
            <div class="phase-card-head">
              <div class="phase-title-with-badge">
                <span class="phase-title">决策执行写入 (Execute Phase)</span>
                <span class="exclusive-badge">互斥接管</span>
              </div>
              <Switch
                label="决策执行写入"
                expandedHitArea
                checked={draftConfig.execute_enabled}
                disabled={!canWrite || !draftConfig.enabled}
                on:change={(e) => draftConfig.execute_enabled = e.detail}
              />
            </div>
            <p class="phase-desc">
              执行已冻结的决策计划写入 Jira。<strong>开启后系统将自动互斥接管旧版转派改期通道</strong>。
            </p>
          </div>
        </div>
      </div>
    </section>

    <!-- Section 2: Online Client Installation Combo -->
    <section class="gov-card" aria-labelledby="install-section-title">
      <div class="card-header">
        <div class="title-wrap">
          <h4 id="install-section-title">远程客户端接入与在线安装</h4>
          <span class="sub-hint">完成凭证签发、客户端配置和连接验证，即可在 Codex 中使用开放工具。</span>
        </div>
      </div>

      <ol class="onboarding-steps" aria-label="Codex 接入步骤">
        <li>
          <strong>签发集成凭证</strong>
          <p>来源填写 <code>codex-dev</code>，选择有效期后签发。完整 API Key 只显示一次，请立即保存；使用完整密钥，不使用 Key ID 或网页登录令牌。</p>
          {#if canWrite}
            <Button variant="primary" size="small" on:click={openCodexCredentialModal}>签发 Codex API Key</Button>
          {:else}
            <span class="field-hint">当前为只读权限，请联系管理员签发集成凭证。</span>
          {/if}
        </li>
        <li>
          <strong>安装 Skill 并配置 MCP</strong>
          <p>在终端执行下方命令，按提示隐藏输入 API Key。脚本安装 Skill、验证 MCP，再写入 <code>~/.codex/config.toml</code>；已有配置会备份，其他服务保留。</p>
          <div class="code-snippet-bar">
            <code>{codexInstallCommand}</code>
            <Button variant="secondary" size="small" disabled={!codexInstallCommand} on:click={() => copyText(codexInstallCommand, 'Codex 安装与配置命令')}>复制 Codex 命令</Button>
          </div>
          <p class="field-hint">自动配置需要 Python 3.11+（或 Python 3 + tomli）。自定义根目录请替换命令中的路径；已安装 Skill 可将 <code>--mcp</code> 改为 <code>--mcp-only</code>。也可使用下方 Codex 模板手动配置。</p>
        </li>
        <li>
          <strong>验证客户端连接</strong>
          <p>重启 Codex 或重新加载会话，在 <code>/mcp</code> 查看连接与工具列表。<code>codex mcp list</code> 仅确认配置存在。401 表示凭证无效、过期或撤销；403 需管理员检查开放能力开关、当前生效策略及来源权限。</p>
        </li>
      </ol>

      <!-- Channels 1 & 2: Top Quick Connect Grid -->
      <div class="combo-grid">
        <!-- Channel 1: Deep Link -->
        <div class="combo-card">
          <div class="combo-header">
            <span class="combo-tag">通道一</span>
            <span class="combo-title">Cursor 协议一键拉起 (Deep Link)</span>
          </div>
          <p class="combo-desc">
            添加 <code>well-ambient</code> MCP 占位配置后，在 Cursor 中填写 Authorization。安装链接不包含真实密钥。
          </p>
          <div class="combo-actions">
            <Button
              variant="primary"
              size="small"
              disabled={!cursorDeepLink}
              on:click={() => window.open(cursorDeepLink, '_blank')}
            >
              在 Cursor 中添加 MCP 配置
            </Button>
            <Button
              variant="secondary"
              size="small"
              disabled={!cursorDeepLink}
              on:click={() => copyText(cursorDeepLink, 'Cursor 深链')}
            >
              复制深链
            </Button>
          </div>
        </div>

        <!-- Channel 2: Remote Install Script -->
        <div class="combo-card">
          <div class="combo-header">
            <span class="combo-tag">通道二</span>
            <span class="combo-title">远程安装脚本 (One-line Bash Installer)</span>
          </div>
          <p class="combo-desc">
            下载官方 Skill，探测 Codex / Claude Code 技能目录，并在终端中选择或输入安装根目录。指定目录可在命令末尾加 <code>-s -- --root "$HOME/.codex"</code>；Skill 将放入该目录下的 <code>skills</code>。选择 Codex 根目录后会询问是否配置 MCP；其他客户端使用下方模板。
          </p>
          <div class="code-snippet-bar">
            <code>{installCommand}</code>
            <Button
              variant="secondary"
              size="small"
              disabled={!installCommand}
              on:click={() => copyText(installCommand, '一键安装命令')}
            >
              复制命令
            </Button>
          </div>
        </div>
      </div>

      <div class="registry-entry">
        <div><h5>统一 Skill 中心</h5><p>一个 well-ambient 主技能，包含四个业务子技能；集中查看文档、发布状态与调用观测。</p></div>
        <Button variant="secondary" on:click={onOpenRegistry}>打开 Skill 中心</Button>
      </div>

      <!-- Channel 4: Manual Config JSON Tabs -->
      <div class="sub-section">
        <div class="sub-section-header">
          <div class="sub-section-title-wrap">
            <span class="combo-tag">通道三</span>
            <h5 class="sub-section-title">客户端手动配置</h5>
          </div>
          <div class="config-tabs-nav" role="group" aria-label="客户端配置文件类型">
            <button type="button" class="tab-btn" class:active={activeConfigTab === 'codex'} aria-pressed={activeConfigTab === 'codex'} on:click={() => activeConfigTab = 'codex'}>Codex</button>
            <button
              type="button"
              class="tab-btn"
              class:active={activeConfigTab === 'cursor'}
              aria-pressed={activeConfigTab === 'cursor'}
              on:click={() => activeConfigTab = 'cursor'}
            >
              Cursor
            </button>
            <button
              type="button"
              class="tab-btn"
              class:active={activeConfigTab === 'claude'}
              aria-pressed={activeConfigTab === 'claude'}
              on:click={() => activeConfigTab = 'claude'}
            >
              Claude Desktop
            </button>
            <button
              type="button"
              class="tab-btn"
              class:active={activeConfigTab === 'windsurf'}
              aria-pressed={activeConfigTab === 'windsurf'}
              on:click={() => activeConfigTab = 'windsurf'}
            >
              Windsurf
            </button>
            <button
              type="button"
              class="tab-btn"
              class:active={activeConfigTab === 'cline'}
              aria-pressed={activeConfigTab === 'cline'}
              on:click={() => activeConfigTab = 'cline'}
            >
              Cline / Roo Code
            </button>
          </div>
        </div>

        <div class="config-code-preview">
          {#if activeConfigTab === 'codex'}
            <div class="file-hint">合并到 <code>~/.codex/config.toml</code>（自定义 CODEX_HOME 或项目 <code>.codex</code> 时使用对应根目录）。将占位符替换为完整 API Key，并设置文件权限为 <code>600</code>。</div>
            <pre><code>{codexConfig}</code></pre>
            <div class="code-action-pos">
              <Button variant="secondary" size="small" on:click={() => copyText(codexConfig, 'Codex 配置模板')}>复制 Codex 模板</Button>
            </div>
          {:else if activeConfigTab === 'cursor'}
            <div class="file-hint">写入 <code>.cursor/mcp.json</code> 或系统全局 MCP 配置：</div>
            <pre><code>{`{
  "mcpServers": {
    "well-ambient": {
      "url": "${currentMcpURL}",
      "headers": {
        "Authorization": "Bearer ${effectiveToken}"
      }
    }
  }
}`}</code></pre>
            <div class="code-action-pos">
              <Button
                variant="secondary"
                size="small"
                on:click={() => copyText(`{
  "mcpServers": {
    "well-ambient": {
      "url": "${currentMcpURL}",
      "headers": {
        "Authorization": "Bearer ${effectiveToken}"
      }
    }
  }
}`, 'Cursor 配置')}
              >
                复制代码
              </Button>
            </div>
          {:else if activeConfigTab === 'claude'}
            <div class="file-hint">写入 <code>~/Library/Application Support/Claude/claude_desktop_config.json</code>：</div>
            <pre><code>{`{
  "mcpServers": {
    "well-ambient": {
      "url": "${currentMcpURL}",
      "headers": {
        "Authorization": "Bearer ${effectiveToken}"
      }
    }
  }
}`}</code></pre>
            <div class="code-action-pos">
              <Button
                variant="secondary"
                size="small"
                on:click={() => copyText(`{
  "mcpServers": {
    "well-ambient": {
      "url": "${currentMcpURL}",
      "headers": {
        "Authorization": "Bearer ${effectiveToken}"
      }
    }
  }
}`, 'Claude Desktop 配置')}
              >
                复制代码
              </Button>
            </div>
          {:else if activeConfigTab === 'windsurf'}
            <div class="file-hint">写入 <code>~/.codeium/windsurf/mcp_config.json</code>：</div>
            <pre><code>{`{
  "mcpServers": {
    "well-ambient": {
      "serverUrl": "${currentMcpURL}",
      "headers": {
        "Authorization": "Bearer ${effectiveToken}"
      }
    }
  }
}`}</code></pre>
            <div class="code-action-pos">
              <Button
                variant="secondary"
                size="small"
                on:click={() => copyText(`{
  "mcpServers": {
    "well-ambient": {
      "serverUrl": "${currentMcpURL}",
      "headers": {
        "Authorization": "Bearer ${effectiveToken}"
      }
    }
  }
}`, 'Windsurf 配置')}
              >
                复制代码
              </Button>
            </div>
          {:else}
            <div class="file-hint">VS Code 扩展设置中的 MCP 服务器定义：</div>
            <pre><code>{`{
  "mcpServers": {
    "well-ambient": {
      "transport": "sse",
      "url": "${currentMcpURL}",
      "headers": {
        "Authorization": "Bearer ${effectiveToken}"
      }
    }
  }
}`}</code></pre>
            <div class="code-action-pos">
              <Button
                variant="secondary"
                size="small"
                on:click={() => copyText(`{
  "mcpServers": {
    "well-ambient": {
      "transport": "sse",
      "url": "${currentMcpURL}",
      "headers": {
        "Authorization": "Bearer ${effectiveToken}"
      }
    }
  }
}`, 'Cline 配置')}
              >
                复制代码
              </Button>
            </div>
          {/if}
        </div>
      </div>
    </section>

    <!-- Section 3: Credentials & Integration Key Lifecycle -->
    <section class="gov-card" aria-labelledby="credentials-section-title">
      <div class="card-header">
        <div class="title-wrap">
          <h4 id="credentials-section-title">集成凭证 (API Keys) 生命周期管理</h4>
          <span class="sub-hint">签发带有效期的集成凭证，所有密钥共享来源租约配额与安全审计。</span>
        </div>
        {#if canWrite}
          <Button
            variant="primary"
            size="small"
            on:click={() => { issueError = ''; showIssueModal = true; issuedKeyResult = null; }}
          >
            + 签发新 API Key
          </Button>
        {/if}
      </div>

      {#if (overview.credentials || []).length === 0}
        <div class="empty-box">
          <p>暂无已签发的 API Key。点击右上角“+ 签发新 API Key”为外部 Agent 或 MCP 客户端创建访问密钥。</p>
        </div>
      {:else}
        <!-- svelte-ignore a11y_no_noninteractive_tabindex (Named scroll regions need keyboard focus for horizontal navigation.) -->
        <div class="gov-table-container" role="region" aria-label="API Key 凭证列表，支持横向滚动" tabindex="0">
          <table class="gov-table" aria-label="API Key 凭证列表">
            <thead>
              <tr>
                <th>Key ID</th>
                <th>前缀摘要</th>
                <th>归属来源 (Source)</th>
                <th>状态</th>
                <th>到期时间</th>
                <th>最近调用</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {#each overview.credentials as cred}
                <tr>
                  <td><code class="mono-tag">{cred.key_id}</code></td>
                  <td><code class="mono-prefix">{cred.prefix}...</code></td>
                  <td><span class="source-tag">{cred.source_id}</span></td>
                  <td>
                    <span class="status-pill" class:active={cred.status === 'active'} class:inactive={cred.status !== 'active'}>
                      {cred.status === 'active' ? '正常' : cred.status === 'revoked' ? '已撤销' : '已禁用'}
                    </span>
                  </td>
                  <td>{cred.expires_at ? new Date(cred.expires_at).toLocaleString() : '永久'}</td>
                  <td>{cred.last_used_at ? new Date(cred.last_used_at).toLocaleString() : '尚未调用'}</td>
                  <td>
                    {#if canWrite && cred.status === 'active'}
                      <Button
                        variant="danger"
                        size="small"
                        disabled={revokingKeyID === cred.key_id}
                        loading={revokingKeyID === cred.key_id}
                        on:click={() => revokeKey(cred.key_id)}
                      >
                        撤销
                      </Button>
                    {:else}
                      <span class="text-muted">—</span>
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>

    <!-- Section 4: Jira Bindings & Execution Protection -->
    <section class="gov-card" aria-labelledby="bindings-section-title">
      <div class="card-header">
        <div class="title-wrap">
          <h4 id="bindings-section-title">Jira 写入执行绑定 (Execution Bindings)</h4>
          <span class="sub-hint">定义单事项排期决策被执行时，所使用的 Jira 服务账号与连接器。</span>
        </div>
      </div>

      {#if (overview.bindings || []).length === 0}
        <div class="empty-box">
          <p>暂无针对项目的执行绑定。请在 CLI 或策略中为项目配置 <code>reassign</code> 与 <code>reschedule</code> 绑定。</p>
        </div>
      {:else}
        <!-- svelte-ignore a11y_no_noninteractive_tabindex (Named scroll regions need keyboard focus for horizontal navigation.) -->
        <div class="gov-table-container" role="region" aria-label="Jira 执行绑定列表，支持横向滚动" tabindex="0">
          <table class="gov-table" aria-label="Jira 写入执行绑定">
            <thead>
              <tr>
                <th>项目标识 (Project)</th>
                <th>动作分类 (Action)</th>
                <th>连接器 (Connector)</th>
                <th>执行账号 (Executor)</th>
                <th>状态</th>
              </tr>
            </thead>
            <tbody>
              {#each overview.bindings as binding}
                <tr>
                  <td><strong>{binding.project_ref}</strong></td>
                  <td><code>{binding.action_class}</code></td>
                  <td>{binding.connector_ref}</td>
                  <td>{binding.executor_ref}</td>
                  <td>
                    <span class="status-pill active">
                      {binding.status}
                    </span>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>
  {/if}

  <Modal show={manualCopy !== null} title={`复制${manualCopy?.label || '内容'}`} on:close={() => manualCopy = null}>
    <div class="manual-copy-body">
      <p id="manual-copy-hint">浏览器未允许自动复制。点击下方文本后，按 ⌘C / Ctrl+C，或使用系统的“复制”菜单。</p>
      <label for="manual-copy-text">{manualCopy?.label || '待复制内容'}</label>
      <textarea id="manual-copy-text" class="manual-copy-text" readonly rows="6" value={manualCopy?.text || ''} aria-describedby="manual-copy-hint" on:focus={selectCopyText} on:click={(event) => event.currentTarget.select()}></textarea>
    </div>
    <svelte:fragment slot="footer">
      <Button variant="secondary" on:click={() => document.getElementById('manual-copy-text')?.focus()}>选中文本</Button>
      <Button variant="primary" on:click={() => manualCopy = null}>关闭</Button>
    </svelte:fragment>
  </Modal>

  <!-- Issue New Key Modal (Shared Modal Component) -->
  <Modal
    show={showIssueModal}
    title="签发开放能力集成 API Key"
    size="default"
    on:close={closeIssueModal}
  >
    <div class="modal-form-body">
      {#if issueError}<div class="gov-notice warning" role="alert">签发失败：{issueError}。请检查权限或稍后重试。</div>{/if}
      {#if issuedKeyResult}
        <div class="gov-notice success">
          <span><strong>✓ 密钥生成成功！</strong> 请务必立即复制并保存，该密钥只会完整展示一次。</span>
        </div>

        <div class="token-result-pane">
          <label for="new-issued-key">完整 API Key (Bearer Token):</label>
          <div class="token-copy-row">
            <input
              id="new-issued-key"
              type="text"
              readonly
              value={issuedKeyResult.key}
              class="mono-key-input"
            />
            <Button
              variant="primary"
              size="small"
              on:click={() => copyText(issuedKeyResult?.key || '', '完整 API Key')}
            >
              复制 Key
            </Button>
          </div>
          <p class="token-tip">将密钥填写到客户端本地 Authorization；关闭后无法再次查看完整密钥。安装命令和链接不包含真实密钥。</p>
          <div class="issued-codex-guide">
            <strong>下一步：写入 Codex MCP</strong>
            <p>复制含本次凭证的 TOML，合并到 <code>~/.codex/config.toml</code>（或所选根目录），保留其他配置并将文件权限设为 <code>600</code>。重启 Codex 后用 <code>/mcp</code> 验证连接。</p>
            <Button variant="secondary" size="small" on:click={() => copyText(buildCodexMcpConfig(currentMcpURL, issuedKeyResult?.key || ''), '含凭证的 Codex 配置')}>复制含凭证的 Codex 配置</Button>
            {#if issuedManualCopy}
              <label for="issued-copy-text">浏览器未允许自动复制，请选中并复制{issuedManualCopy.label}：</label>
              <textarea id="issued-copy-text" class="manual-copy-text" readonly rows="5" value={issuedManualCopy.text} on:focus={selectCopyText} on:click={(event) => event.currentTarget.select()}></textarea>
            {/if}
            <p class="field-hint">也可执行接入步骤中的脚本，在隐藏输入提示中粘贴密钥；已有 Skill 使用 <code>--mcp-only</code>。请妥善保管配置，勿分享或提交到代码仓库。</p>
          </div>
        </div>
      {:else}
        <div class="form-item">
          <label for="issue-source-id">集成来源标识 (Source ID):</label>
          <input
            id="issue-source-id"
            type="text"
            bind:value={newKeySourceID}
            placeholder="例如 target-agent, cursor-dev, ai-platform"
            class="gov-input"
          />
          <span class="field-hint">对应集成方身份，同一来源下的多个轮换 Key 共享统一配额。</span>
        </div>

        <div class="form-item">
          <Select
            id="issue-ttl"
            label="有效期 (TTL 时长)"
            value={String(newKeyTTLHours)}
            options={ttlOptions}
            searchable={false}
            compact
            shadowless
            on:change={(e) => newKeyTTLHours = Number(e.detail)}
          />
        </div>
      {/if}
    </div>

    <div slot="footer" class="modal-footer-actions">
      {#if issuedKeyResult}
        <Button variant="primary" on:click={closeIssueModal}>
          完成并关闭
        </Button>
      {:else}
        <Button variant="ghost" on:click={closeIssueModal}>
          取消
        </Button>
        <Button
          variant="primary"
          disabled={issuingKey || !newKeySourceID.trim()}
          loading={issuingKey}
          on:click={issueNewKey}
        >
          确认签发
        </Button>
      {/if}
    </div>
  </Modal>
</div>

<style>
  .name-prefix-setting { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: var(--wa-space-5); padding-block: var(--wa-space-4); border-bottom: 1px solid var(--wa-border-divider); margin-bottom: var(--wa-space-4); }
  .name-prefix-field, .name-prefix-preview { display: grid; gap: var(--wa-space-2); align-content: start; min-width: 0; }
  .name-prefix-field label { font-size: 13px; font-weight: 600; }
  .name-prefix-field input { max-width: 320px; width: 100%; box-sizing: border-box; padding: 10px 12px; border: 1px solid var(--wa-border-divider); border-radius: 8px; background: var(--wa-surface-inset); color: var(--wa-text-main); font: inherit; }
  .name-prefix-field input:focus-visible { outline: 2px solid var(--wa-accent-strong); outline-offset: 2px; }
  .name-prefix-field p, .name-prefix-preview span, .registry-entry p { margin: 0; font-size: 12px; line-height: 1.65; color: var(--wa-text-muted); }
  .name-prefix-field .prefix-error { color: var(--wa-danger, #9c3434); }
  .name-prefix-preview code { font-size: 12px; color: var(--wa-accent-strong); overflow-wrap: anywhere; }
  .registry-entry { display: flex; gap: var(--wa-space-4); align-items: center; justify-content: space-between; padding-block: var(--wa-space-4); border-top: 1px solid var(--wa-border-divider); }
  .registry-entry h5 { margin: 0 0 var(--wa-space-2); font-size: 14px; }
  @media (max-width: 760px) { .name-prefix-setting { grid-template-columns: minmax(0, 1fr); } .name-prefix-field input { min-height: 44px; max-width: none; } .registry-entry { flex-direction: column; align-items: stretch; } }

  .onboarding-steps {
    margin: 0 0 var(--wa-space-6);
    padding-left: var(--wa-space-6);
    color: var(--wa-text-main);
  }
  .onboarding-steps li {
    padding: var(--wa-space-3) 0;
    border-bottom: 1px solid var(--wa-border-soft);
  }
  .onboarding-steps p, .issued-codex-guide p {
    margin: var(--wa-space-2) 0;
    line-height: 1.65;
    overflow-wrap: anywhere;
  }
  .issued-codex-guide {
    margin-top: var(--wa-space-4);
    padding-top: var(--wa-space-4);
    border-top: 1px solid var(--wa-border-soft);
  }
  .issued-codex-guide label {
    display: block;
    margin: var(--wa-space-3) 0 var(--wa-space-2);
  }
  .manual-copy-body {
    display: grid;
    gap: var(--wa-space-3);
    color: var(--wa-text-main);
  }
  .manual-copy-body p { margin: 0; line-height: 1.6; }
  .manual-copy-body label { font-weight: 600; }
  .manual-copy-text {
    box-sizing: border-box;
    width: 100%;
    min-width: 0;
    resize: vertical;
    padding: var(--wa-space-3);
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-sm);
    background: var(--wa-bg-rail-soft);
    color: var(--wa-rail-text-strong);
    font: 12px/1.6 var(--wa-font-mono);
    overflow-wrap: anywhere;
  }
  .manual-copy-text:focus-visible { outline: 2px solid var(--wa-accent); outline-offset: 2px; }

  /* Shared Phase 41 material; only top-level surfaces blur. */

  .onboarding-steps {
    margin: 0 0 var(--wa-space-6);
    padding-left: var(--wa-space-6);
    color: var(--wa-text-main);
  }
  .onboarding-steps li {
    padding: var(--wa-space-3) 0;
    border-bottom: 1px solid var(--wa-border-soft);
  }
  .onboarding-steps p, .issued-codex-guide p {
    margin: var(--wa-space-2) 0;
    line-height: 1.65;
    overflow-wrap: anywhere;
  }
  .issued-codex-guide {
    margin-top: var(--wa-space-4);
    padding-top: var(--wa-space-4);
    border-top: 1px solid var(--wa-border-soft);
  }
  .issued-codex-guide label {
    display: block;
    margin: var(--wa-space-3) 0 var(--wa-space-2);
  }
  .manual-copy-body {
    display: grid;
    gap: var(--wa-space-3);
    color: var(--wa-text-main);
  }
  .manual-copy-body p { margin: 0; line-height: 1.6; }
  .manual-copy-body label { font-weight: 600; }
  .manual-copy-text {
    box-sizing: border-box;
    width: 100%;
    min-width: 0;
    resize: vertical;
    padding: var(--wa-space-3);
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-sm);
    background: var(--wa-bg-rail-soft);
    color: var(--wa-rail-text-strong);
    font: 12px/1.6 var(--wa-font-mono);
    overflow-wrap: anywhere;
  }
  .manual-copy-text:focus-visible { outline: 2px solid var(--wa-accent); outline-offset: 2px; }

  /* Shared Phase 41 material; only top-level surfaces blur. */
  .open-capabilities-view {
    display: flex;
    flex-direction: column;
    gap: var(--wa-space-4);
    width: 100%;
    min-width: 0;
    max-width: 100%;
    color: var(--wa-text-main);
    font-family: var(--wa-font-sans);
  }

  .gov-card,
  .gov-metrics-strip,
  .gov-loading-pane {
    min-width: 0;
    border: 1px solid var(--wa-admin-list-surface-border);
    border-top-color: var(--wa-admin-list-surface-border-top);
    border-radius: var(--wa-admin-list-surface-radius);
    background: var(--wa-admin-list-surface-background);
    box-shadow: var(--wa-admin-list-surface-shadow);
    -webkit-backdrop-filter: var(--wa-admin-list-surface-filter);
    backdrop-filter: var(--wa-admin-list-surface-filter);
  }

  .gov-card {
    display: flex;
    flex-direction: column;
    gap: var(--wa-space-4);
    padding: var(--wa-space-4) var(--wa-space-5);
  }

  .open-gov-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--wa-space-4);
    min-width: 0;
  }

  .header-main,
  .title-wrap,
  .switch-content {
    flex: 1;
    min-width: 0;
  }

  .header-title-row,
  .card-header,
  .sub-section-header {
    display: flex;
    align-items: center;
    gap: var(--wa-space-3);
    flex-wrap: wrap;
  }

  .card-header,
  .sub-section-header {
    justify-content: space-between;
  }

  .header-actions,
  .save-bar,
  .switch-action {
    flex: 0 0 auto;
  }

  .header-desc,
  .switch-desc {
    margin: var(--wa-space-2) 0 0;
    color: var(--wa-text-muted);
    font-size: 13px;
    line-height: 1.5;
    overflow-wrap: anywhere;
  }

  .status-pill,
  .synced-tag,
  .switch-badge,
  .exclusive-badge,
  .combo-tag,
  .source-tag {
    display: inline-flex;
    align-items: center;
    gap: var(--wa-space-1);
    padding: var(--wa-space-1) var(--wa-space-2);
    border-radius: var(--wa-radius-sm);
    background: var(--wa-neutral-soft);
    color: var(--wa-text-muted);
    font-size: 12px;
    font-weight: 600;
    line-height: 1.25;
  }

  .status-pill,
  .synced-tag,
  .switch-badge,
  .exclusive-badge,
  .combo-tag{
    white-space: nowrap;
  }

  .status-pill.active,
  .synced-tag,
  .master-switch-row.is-active .switch-badge {
    background: var(--wa-success-soft);
    color: var(--wa-success);
  }

  .status-pill.warning,
  .exclusive-badge {
    background: var(--wa-warning-soft);
    color: var(--wa-warning);
  }

  .gov-metrics-strip {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: var(--wa-space-4);
    padding: var(--wa-space-4) var(--wa-space-5);
  }

  .gov-metric-item {
    display: flex;
    flex-direction: column;
    gap: var(--wa-space-1);
    min-width: 0;
  }

  .gov-metric-divider {
    display: none;
  }

  .gov-metric-label,
  .gov-metric-sub {
    color: var(--wa-text-muted);
    font-size: 12px;
    line-height: 1.45;
  }

  .gov-metric-label {
    font-weight: 600;
  }

  .gov-metric-val {
    display: flex;
    align-items: baseline;
    flex-wrap: wrap;
    gap: var(--wa-space-1) var(--wa-space-2);
    overflow-wrap: anywhere;
  }

  .gov-metric-val strong {
    color: var(--wa-text-strong);
    font-size: 18px;
    font-weight: 700;
  }

  .highlight-cyan {
    color: var(--wa-accent-strong) !important;
  }

  .highlight-green {
    color: var(--wa-success) !important;
  }

  .highlight-gray,
  .text-muted {
    color: var(--wa-text-muted) !important;
  }

  .title-wrap h4,
  .sub-section-title {
    margin: 0;
    color: var(--wa-text-strong);
    font-size: 16px;
    font-weight: 650;
    line-height: 1.4;
    text-wrap: balance;
    overflow-wrap: anywhere;
  }

  .sub-section-title {
    font-size: 14px;
  }

  .sub-hint,
  .field-hint,
  .token-tip {
    display: block;
    margin: var(--wa-space-1) 0 0;
    color: var(--wa-text-muted);
    font-size: 13px;
    line-height: 1.5;
    overflow-wrap: anywhere;
  }

  .switches-container,
  .sub-section {
    display: flex;
    flex-direction: column;
    gap: var(--wa-space-4);
    min-width: 0;
  }

  .sub-section {
    border-top: 1px solid var(--wa-border-divider);
    padding-top: var(--wa-space-4);
  }

  .master-switch-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--wa-space-4);
    padding-block: var(--wa-space-2) var(--wa-space-3);
    border-bottom: 1px solid var(--wa-border-divider);
  }

  .switch-heading,
  .combo-header,
  .sub-section-title-wrap,
  .phase-title-with-badge {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--wa-space-2);
    min-width: 0;
  }

  .switch-name,
  .phase-title,
  .combo-title{
    color: var(--wa-text-strong);
    font-size: 14px;
    font-weight: 650;
    line-height: 1.4;
    overflow-wrap: anywhere;
  }

  .open-capabilities-view :global(.switch-label) {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
    border: 0;
  }

  .phases-grid{
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: var(--wa-space-4);
    min-width: 0;
  }

  .combo-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--wa-space-4);
    min-width: 0;
  }

  .phase-card,
  .combo-card{
    display: flex;
    flex-direction: column;
    gap: var(--wa-space-3);
    min-width: 0;
    padding-block: var(--wa-space-3);
    border-top: 1px solid var(--wa-border-divider);
    background: transparent;
  }

  .phase-card {
    padding-block-start: 0;
    border-top: 0;
  }

  .phase-card.is-disabled {
    opacity: 0.6;
  }

  .phase-card-head{
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--wa-space-3);
    min-width: 0;
  }

  .phase-desc,
  .combo-desc{
    flex: 1;
    margin: 0;
    color: var(--wa-text-muted);
    font-size: 13px;
    line-height: 1.5;
    overflow-wrap: anywhere;
  }

  .combo-actions{
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--wa-space-2);
    min-width: 0;
  }





  .code-snippet-bar,
  .config-code-preview {
    min-width: 0;
    max-width: 100%;
    padding: var(--wa-space-3);
    border-radius: var(--wa-radius-sm);
    background: var(--wa-bg-rail-soft);
    color: var(--wa-rail-text-main);
  }

  .code-snippet-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--wa-space-3);
  }

  .code-snippet-bar code {
    flex: 1;
    min-width: 0;
    overflow-x: auto;
    color: var(--wa-rail-text-strong);
    font-family: var(--wa-font-mono);
    font-size: 12px;
    line-height: 1.6;
    white-space: nowrap;
  }

  .code-snippet-bar :global(.btn) {
    flex: 0 0 auto;
  }

  .config-tabs-nav {
    display: flex;
    flex-wrap: wrap;
    gap: var(--wa-space-1);
    min-width: 0;
  }

  .tab-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: var(--wa-control-h);
    padding: var(--wa-space-2) var(--wa-space-3);
    border: 1px solid transparent;
    border-radius: var(--wa-radius-sm);
    background: transparent;
    color: var(--wa-text-muted);
    font: 600 13px var(--wa-font-sans);
    white-space: nowrap;
    cursor: pointer;
    transition: background-color var(--wa-duration-fast) var(--wa-ease),
      color var(--wa-duration-fast) var(--wa-ease),
      border-color var(--wa-duration-fast) var(--wa-ease);
  }

  .tab-btn:hover {
    background: var(--wa-row-hover);
    color: var(--wa-text-strong);
  }

  .tab-btn.active {
    border-color: var(--wa-border-soft);
    background: var(--wa-surface-lift);
    color: var(--wa-accent-strong);
  }

  .config-code-preview {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--wa-space-3);
    padding: var(--wa-space-4);
  }

  .file-hint {
    grid-column: 1;
    grid-row: 1;
    min-width: 0;
    color: var(--wa-rail-text-main);
    font-size: 12px;
    line-height: 1.5;
    overflow-wrap: anywhere;
  }

  .file-hint code {
    color: var(--wa-rail-text-strong);
  }

  .config-code-preview pre {
    grid-column: 1 / -1;
    grid-row: 2;
    min-width: 0;
    margin: 0;
    overflow-x: auto;
    color: var(--wa-rail-text-strong);
    font-family: var(--wa-font-mono);
    font-size: 12px;
    line-height: 1.6;
  }

  .code-action-pos {
    grid-column: 2;
    grid-row: 1;
  }

  /* Lists are embedded in their parent glass surface. */
  .gov-table-container {
    width: 100%;
    min-width: 0;
    max-width: 100%;
    overflow-x: auto;
    border-top: 1px solid var(--wa-border-divider);
    background: transparent;
  }

  .gov-table {
    width: 100%;
    min-width: 760px;
    border-collapse: collapse;
    font-size: 13px;
  }

  .gov-table th,
  .gov-table td {
    padding: var(--wa-space-3) var(--wa-space-4);
    border-bottom: 1px solid var(--wa-border-divider);
    text-align: left;
    vertical-align: middle;
    white-space: nowrap;
  }

  .gov-table th {
    background: var(--wa-surface-lift);
    color: var(--wa-text-muted);
    font-size: 12px;
    font-weight: 650;
  }

  .gov-table tbody tr:last-child td {
    border-bottom: 0;
  }

  .gov-table tbody tr:hover {
    background: var(--wa-row-hover);
  }

  .mono-tag,
  .mono-prefix {
    color: var(--wa-text-strong);
    font-family: var(--wa-font-mono);
    font-size: 12px;
  }

  .mono-prefix {
    color: var(--wa-text-muted);
  }

  .source-tag {
    color: var(--wa-text-main);
  }

  .empty-box {
    padding: var(--wa-space-5) 0;
    border-top: 1px solid var(--wa-border-divider);
    color: var(--wa-text-muted);
    font-size: 13px;
    line-height: 1.6;
  }

  .empty-box p {
    max-width: 72ch;
    margin: 0;
  }

  .gov-loading-pane {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--wa-space-3);
    min-height: 160px;
    padding: var(--wa-space-5);
    color: var(--wa-text-muted);
    font-size: 13px;
  }

  .gov-loading-pane p {
    margin: 0;
    line-height: 1.5;
  }

  .spinner {
    flex: 0 0 auto;
    width: 24px;
    height: 24px;
    border: 2px solid var(--wa-border-soft);
    border-top-color: var(--wa-accent);
    border-radius: var(--wa-radius-pill);
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .gov-notice {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--wa-space-3);
    min-width: 0;
    padding: var(--wa-space-3) var(--wa-space-4);
    border-radius: var(--wa-radius-sm);
    font-size: 13px;
    line-height: 1.5;
    overflow-wrap: anywhere;
  }

  .gov-notice.error {
    background: var(--wa-danger-soft);
    color: var(--wa-danger);
  }

  .gov-notice.info {
    background: var(--wa-info-soft);
    color: var(--wa-info-strong);
  }

  .gov-notice.success {
    background: var(--wa-success-soft);
    color: var(--wa-success);
  }

  .modal-form-body,
  .form-item,
  .token-result-pane {
    display: flex;
    flex-direction: column;
    gap: var(--wa-space-3);
    min-width: 0;
  }

  .modal-form-body {
    gap: var(--wa-space-5);
  }

  .form-item label,
  .token-result-pane label {
    color: var(--wa-text-strong);
    font-size: 13px;
    font-weight: 650;
  }

  .gov-input,
  .mono-key-input {
    box-sizing: border-box;
    min-width: 0;
    width: 100%;
    min-height: var(--wa-control-h);
    padding: var(--wa-space-2) var(--wa-space-3);
    border: 1px solid var(--wa-border-soft);
    border-radius: var(--wa-radius-sm);
    background: var(--wa-surface-lift);
    color: var(--wa-text-main);
    font-family: var(--wa-font-sans);
    font-size: 13px;
  }

  .gov-input::placeholder {
    color: var(--wa-text-muted);
  }

  .token-result-pane {
    border-top: 1px solid var(--wa-border-divider);
    padding-top: var(--wa-space-4);
  }

  .token-copy-row {
    display: flex;
    align-items: center;
    gap: var(--wa-space-2);
    min-width: 0;
  }

  .mono-key-input {
    flex: 1;
    font-family: var(--wa-font-mono);
  }

  .modal-footer-actions {
    display: flex;
    justify-content: flex-end;
    flex-wrap: wrap;
    gap: var(--wa-space-3);
  }


  .tab-btn:focus-visible,
  .gov-table-container:focus-visible,
  .gov-input:focus-visible,
  .mono-key-input:focus-visible {
    outline: 2px solid var(--wa-border-focus);
    outline-offset: 2px;
  }

  @media (max-width: 1024px) {
    .gov-metrics-strip{
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .phases-grid {
      grid-template-columns: minmax(0, 1fr);
    }

    .phase-card + .phase-card {
      border-top: 1px solid var(--wa-border-divider);
      padding-top: var(--wa-space-3);
    }
  }

  @media (max-width: 760px) {
    .open-gov-header {
      align-items: flex-start;
      flex-wrap: wrap;
    }

    .gov-card,
    .gov-metrics-strip {
      padding: var(--wa-space-4);
    }

    .combo-grid{
      grid-template-columns: minmax(0, 1fr);
    }

    .open-capabilities-view :global(.btn),
    .modal-footer-actions :global(.btn),
    .open-capabilities-view :global(.select-trigger),
    .tab-btn,
    .gov-input,
    .mono-key-input {
      min-height: var(--wa-touch-h);
    }


    .code-snippet-bar {
      flex-direction: column;
      align-items: stretch;
    }

    .code-snippet-bar code {
      width: 100%;
    }

    .code-snippet-bar :global(.btn) {
      align-self: flex-end;
    }
  }

  @media (max-width: 480px) {
    .gov-metrics-strip {
      grid-template-columns: minmax(0, 1fr);
      gap: var(--wa-space-3);
    }

    .config-code-preview {
      grid-template-columns: minmax(0, 1fr);
    }

    .code-action-pos {
      grid-column: 1;
      grid-row: 2;
      justify-self: end;
    }

    .config-code-preview pre {
      grid-row: 3;
    }

    .token-copy-row {
      flex-wrap: wrap;
    }

    .mono-key-input {
      flex-basis: 100%;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .spinner {
      animation: none;
    }
    .tab-btn {
      transition: none;
    }
  }

  @media (prefers-reduced-transparency: reduce) {
    .gov-card,
    .gov-metrics-strip,
    .gov-loading-pane {
      background: var(--wa-admin-list-surface-solid-background);
      -webkit-backdrop-filter: none;
      backdrop-filter: none;
    }
  }
</style>
