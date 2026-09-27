<script lang="ts">
  import { onMount } from 'svelte';
  import { showToast } from '../../lib/toast';
  import Button from '../shared/Button.svelte';
  import Switch from '../shared/Switch.svelte';
  import Modal from '../shared/Modal.svelte';
  import Select from '../shared/Select.svelte';

  export let canWrite = false;

  const ttlOptions = [
    { value: '0', label: '永久有效 (Permanent)' },
    { value: '168', label: '7 天 (168h)' },
    { value: '720', label: '30 天 (720h - 默认推荐)' },
    { value: '2160', label: '90 天 (2160h)' },
    { value: '8760', label: '1 年 (8760h)' }
  ];

  interface OpenCapabilitiesConfigState {
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
    enabled: false,
    read_enabled: false,
    prepare_enabled: false,
    execute_enabled: false
  };

  // Selected key for install instructions
  let selectedToken = '';
  let activeConfigTab: 'cursor' | 'claude' | 'windsurf' | 'cline' = 'cursor';

  // Issue Key Modal State
  let showIssueModal = false;
  let newKeySourceID = 'target-agent';
  let newKeyTTLHours = 720;
  let issuingKey = false;
  let issuedKeyResult: { key_id: string; key: string; expires_at: string } | null = null;

  // Revoking key
  let revokingKeyID = '';

  $: isDirty = overview ? (
    draftConfig.enabled !== overview.config.enabled ||
    draftConfig.read_enabled !== overview.config.read_enabled ||
    draftConfig.prepare_enabled !== overview.config.prepare_enabled ||
    draftConfig.execute_enabled !== overview.config.execute_enabled
  ) : false;

  $: currentBaseURL = overview?.base_url || (typeof window !== 'undefined' ? window.location.origin : '');
  $: currentMcpURL = overview?.mcp?.endpoint_url || `${currentBaseURL}/mcp`;
  $: effectiveToken = selectedToken || '<YOUR_API_KEY>';

  // Deep Link for Cursor
  $: cursorDeepLink = `cursor://anysphere.cursor-mcp/install?name=well-ambient&url=${encodeURIComponent(currentMcpURL)}&headers=${encodeURIComponent(JSON.stringify({ Authorization: `Bearer ${effectiveToken}` }))}`;

  // Install command
  $: installCommand = `curl -fsSL "${currentBaseURL}/open/v1/install.sh?token=${effectiveToken}" | bash`;

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
      selectedToken = res.key;
      showToast('API Key 签发成功，已自动同步到上方安装命令！', { type: 'success' });
      await loadOverview();
    } catch (err: any) {
      error = err.message || '签发 Key 失败';
      showToast(error, { type: 'error' });
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

  function copyText(text: string, label = '内容') {
    if (typeof navigator !== 'undefined' && navigator.clipboard) {
      navigator.clipboard.writeText(text).then(() => {
        showToast(`${label}已复制到剪贴板！`, { type: 'success' });
      }).catch(() => {
        prompt('请手动复制:', text);
      });
    } else {
      prompt('请手动复制:', text);
    }
  }

  onMount(() => {
    loadOverview();
  });
</script>

<div class="open-capabilities-view" role="region" aria-label="开放能力与 MCP 集成管理">
  {#if loading}
    <div class="gov-loading-pane">
      <span class="spinner" aria-hidden="true"></span>
      <p>正在拉取开放能力配置与服务概览…</p>
    </div>
  {:else if error && !overview}
    <div class="gov-notice error">
      <span><strong>加载异常：</strong> {error}</span>
      <Button variant="ghost" size="small" on:click={loadOverview}>重试</Button>
    </div>
  {:else if overview}
    <!-- Top Header: Identity & Master Status -->
    <div class="open-gov-header">
      <div class="header-main">
        <div class="header-title-row">
          <h3>开放能力与 MCP 服务治理</h3>
          <span class="status-pill" class:active={overview.config.enabled} class:inactive={!overview.config.enabled}>
            {overview.config.enabled ? '● 服务运行中' : '○ 服务默认未启用'}
          </span>
          {#if overview.active_features.Execute}
            <span class="status-pill warning" title="执行写入已开启，旧版页面直接写入已被全局互斥拦截">
              决策写入互斥生效中
            </span>
          {/if}
        </div>
        <p class="header-desc">
          完全脱离离线文件控制，基于数据库动态版本化配置提供 Jira 分析、排期决策与代码评审能力，支持原生 Streamable HTTP MCP 与标准领域微内核 Skill 分发。
        </p>
      </div>

      <div class="header-actions">
        <Button variant="secondary" size="small" on:click={loadOverview}>
          刷新概览
        </Button>
      </div>
    </div>

    <!-- Top Key Metrics Strip -->
    <div class="gov-metrics-strip" aria-label="开放能力运行指标">
      <div class="gov-metric-item">
        <span class="gov-metric-label">当前运行阶段</span>
        <div class="gov-metric-val">
          {#if !overview.config.enabled}
            <strong class="highlight-gray">已禁用</strong>
            <span class="gov-metric-sub">外部调用全部阻断</span>
          {:else if overview.active_features.Execute}
            <strong class="highlight-cyan">全阶段 (Execute)</strong>
            <span class="gov-metric-sub">只读 + 预备 + 真实写入</span>
          {:else if overview.active_features.Prepare}
            <strong class="highlight-cyan">预备期 (Prepare)</strong>
            <span class="gov-metric-sub">只读 + 决策计划校验</span>
          {:else if overview.active_features.Read}
            <strong class="highlight-green">只读阶段 (Read)</strong>
            <span class="gov-metric-sub">Jira 与评审深度分析</span>
          {:else}
            <strong>已开启</strong>
            <span class="gov-metric-sub">未激活细分阶段</span>
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
        <span class="gov-metric-label">已签发有效 Key</span>
        <div class="gov-metric-val">
          <strong>{(overview.credentials || []).filter(c => c.status === 'active').length} 个</strong>
          <span class="gov-metric-sub">共 {(overview.sources || []).length} 个集成来源</span>
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

      <div class="switches-container">
        <!-- Global Master Switch -->
        <div class="master-switch-row" class:is-active={draftConfig.enabled}>
          <div class="switch-content">
            <div class="switch-heading">
              <span class="switch-name">全局开放能力主控</span>
              <span class="switch-badge">{draftConfig.enabled ? '已开启' : '已关闭'}</span>
            </div>
            <p class="switch-desc">总控总闸。开启后激活 <code>/mcp</code> 协议端点与 <code>/open/v1/*</code> 接口体系。</p>
          </div>
          <div class="switch-action">
            <Switch
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
          <span class="sub-hint">采用“系统协议唤起 + 远程安装脚本 + 在线 Skill 仓库 + 规范配置导出”四大通路无缝接入。</span>
        </div>
      </div>

      <!-- Channels 1 & 2: Top Quick Connect Grid -->
      <div class="combo-grid">
        <!-- Channel 1: Deep Link -->
        <div class="combo-card">
          <div class="combo-header">
            <span class="combo-tag">通道一</span>
            <span class="combo-title">Cursor 协议一键拉起 (Deep Link)</span>
          </div>
          <p class="combo-desc">
            通过系统 URL Scheme 拉起本地 Cursor 并一键添加 <code>well-ambient</code> MCP Server：
          </p>
          <div class="combo-actions">
            <Button
              variant="primary"
              size="small"
              on:click={() => window.open(cursorDeepLink, '_blank')}
            >
              在 Cursor 中一键安装 MCP
            </Button>
            <Button
              variant="secondary"
              size="small"
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
            自动探查本地 Claude Desktop 与 Cursor 环境，安全合并配置并解压官方 Skill：
          </p>
          <div class="code-snippet-bar">
            <code>{installCommand}</code>
            <Button
              variant="secondary"
              size="small"
              on:click={() => copyText(installCommand, '一键安装命令')}
            >
              复制命令
            </Button>
          </div>
        </div>
      </div>

      <!-- Channel 3: Official Skill Registry -->
      <div class="sub-section">
        <div class="sub-section-header">
          <div class="sub-section-title-wrap">
            <span class="combo-tag">通道三</span>
            <h5 class="sub-section-title">官方微内核 Skill 在线仓库 (Online Skill Registry)</h5>
          </div>
          <span class="sub-section-hint">服务端动态打包分发，标准通用 Agent 架构解构：</span>
        </div>

        <div class="skills-cards-grid">
          {#each overview.skills as skill}
            <div class="skill-card">
              <div class="skill-card-top">
                <span class="skill-title">{skill.title}</span>
                <span class="skill-ver">v{skill.version}</span>
              </div>
              <div class="skill-key"><code>{skill.name}</code></div>
              <p class="skill-desc">{skill.description}</p>
              <div class="skill-tools-preview">
                <span class="tool-tag-label">暴露工具 ({skill.tools.length}):</span>
                {#each skill.tools as tool}
                  <span class="tool-chip">{tool}</span>
                {/each}
              </div>
              <div class="skill-card-actions">
                <a
                  href={skill.archive_url}
                  class="btn btn-sm btn-secondary"
                  download
                  title="下载离线 tar.gz 安装包"
                >
                  下载 .tar.gz
                </a>
                <Button
                  variant="ghost"
                  size="small"
                  on:click={() => copyText(`curl -fsSL "${skill.archive_url}" | tar -xz -C .agents/skills`, '安装命令')}
                >
                  复制安装指令
                </Button>
              </div>
            </div>
          {/each}
        </div>
      </div>

      <!-- Channel 4: Manual Config JSON Tabs -->
      <div class="sub-section">
        <div class="sub-section-header">
          <div class="sub-section-title-wrap">
            <span class="combo-tag">通道四</span>
            <h5 class="sub-section-title">主流客户端手动配置规范 (Manual MCP JSON)</h5>
          </div>
          <div class="config-tabs-nav" role="tablist" aria-label="客户端配置文件类型">
            <button
              type="button"
              role="tab"
              class="tab-btn"
              class:active={activeConfigTab === 'cursor'}
              aria-selected={activeConfigTab === 'cursor'}
              on:click={() => activeConfigTab = 'cursor'}
            >
              Cursor
            </button>
            <button
              type="button"
              role="tab"
              class="tab-btn"
              class:active={activeConfigTab === 'claude'}
              aria-selected={activeConfigTab === 'claude'}
              on:click={() => activeConfigTab = 'claude'}
            >
              Claude Desktop
            </button>
            <button
              type="button"
              role="tab"
              class="tab-btn"
              class:active={activeConfigTab === 'windsurf'}
              aria-selected={activeConfigTab === 'windsurf'}
              on:click={() => activeConfigTab = 'windsurf'}
            >
              Windsurf
            </button>
            <button
              type="button"
              role="tab"
              class="tab-btn"
              class:active={activeConfigTab === 'cline'}
              aria-selected={activeConfigTab === 'cline'}
              on:click={() => activeConfigTab = 'cline'}
            >
              Cline / Roo Code
            </button>
          </div>
        </div>

        <div class="config-code-preview">
          {#if activeConfigTab === 'cursor'}
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
            on:click={() => { showIssueModal = true; issuedKeyResult = null; }}
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
        <div class="gov-table-container">
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
        <div class="gov-table-container">
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

  <!-- Issue New Key Modal (Shared Modal Component) -->
  <Modal
    show={showIssueModal}
    title="签发开放能力集成 API Key"
    size="default"
    on:close={() => showIssueModal = false}
  >
    <div class="modal-form-body">
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
          <p class="token-tip">该 Key 已自动同步至上方的远程安装命令与配置示例中。</p>
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
        <Button variant="primary" on:click={() => showIssueModal = false}>
          完成并关闭
        </Button>
      {:else}
        <Button variant="ghost" on:click={() => showIssueModal = false}>
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
  /* Base Viewport Layout */
  .open-capabilities-view {
    display: flex;
    flex-direction: column;
    gap: 16px;
    color: var(--wa-text-main, #293847);
  }

  /* Shared Button Fallback System (Ensures 8-state completeness) */
  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    min-height: var(--wa-control-h, 32px);
    font-size: 13px;
    font-weight: 650;
    padding: 0 12px;
    border-radius: var(--wa-radius-sm, 6px);
    border: 1px solid transparent;
    cursor: pointer;
    text-decoration: none;
    transition: background var(--wa-duration-fast, 140ms) var(--wa-ease, ease),
      border-color var(--wa-duration-fast, 140ms) var(--wa-ease, ease),
      box-shadow var(--wa-duration-fast, 140ms) var(--wa-ease, ease);
    outline: none;
    user-select: none;
    white-space: nowrap;
  }
  .btn:focus-visible {
    box-shadow: 0 0 0 2px rgba(0, 143, 150, 0.2);
  }
  .btn-sm {
    min-height: 28px;
    padding: 0 10px;
    font-size: 12px;
  }
  .btn-primary {
    border-color: var(--wa-accent-fill, #006f76);
    background: var(--wa-accent-fill, #006f76);
    color: var(--wa-accent-fill-ink, #f6fbff);
    box-shadow: 0 2px 4px rgba(0, 143, 150, 0.15);
  }
  .btn-primary:hover:not(:disabled) {
    border-color: var(--wa-accent-fill-hover, #00545a);
    background: var(--wa-accent-fill-hover, #00545a);
  }
  .btn-secondary {
    background: #ffffff;
    border-color: var(--wa-border, #cbd5e0);
    color: var(--wa-text-main, #2d3748);
  }
  .btn-secondary:hover:not(:disabled) {
    background: #f8fafc;
    border-color: #a0aec0;
  }
  .btn-ghost {
    background: transparent;
    border-color: transparent;
    color: var(--wa-text-muted, #718096);
  }
  .btn-ghost:hover:not(:disabled) {
    background: #edf2f7;
    color: var(--wa-text-strong, #1a202c);
  }
  .btn-danger {
    border-color: rgba(221, 75, 62, 0.24);
    background: var(--wa-danger-soft, rgba(221, 75, 62, 0.12));
    color: var(--wa-danger, #dd4b3e);
  }
  .btn-danger:hover:not(:disabled) {
    background: var(--wa-danger, #dd4b3e);
    border-color: var(--wa-danger, #dd4b3e);
    color: #ffffff;
    box-shadow: 0 4px 12px rgba(221, 75, 62, 0.2);
  }
  .btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
    box-shadow: none;
  }

  /* Loading & Notice States */
  .gov-loading-pane {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 60px 20px;
    color: var(--wa-text-muted, #718096);
    gap: 12px;
  }

  .spinner {
    width: 24px;
    height: 24px;
    border: 2px solid var(--wa-border-soft, #e2e8f0);
    border-top-color: var(--wa-accent, #008f96);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .gov-notice {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 10px 16px;
    border-radius: 6px;
    font-size: 13px;
  }
  .gov-notice.error {
    background: #fff5f5;
    border: 1px solid #feb2b2;
    color: #c53030;
  }
  .gov-notice.info {
    background: #ebf8fa;
    border: 1px solid #b2e3e8;
    color: #0c666c;
  }
  .gov-notice.success {
    background: #f0fdf4;
    border: 1px solid #bbf7d0;
    color: #166534;
  }

  /* Header Card */
  .open-gov-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 16px;
    background: #ffffff;
    padding: 16px 20px;
    border-radius: 8px;
    border: 1px solid var(--wa-border, #e2e8f0);
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.02);
  }

  .header-main {
    flex: 1;
    min-width: 0;
  }

  .header-title-row {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }

  .header-title-row h3 {
    margin: 0;
    font-size: 16px;
    font-weight: 650;
    color: var(--wa-text-strong, #0d1722);
  }

  .header-desc {
    margin: 6px 0 0 0;
    font-size: 13px;
    color: var(--wa-text-muted, #718096);
    line-height: 1.5;
  }

  /* Status Pills */
  .status-pill {
    display: inline-flex;
    align-items: center;
    font-size: 11px;
    font-weight: 600;
    padding: 2px 8px;
    border-radius: 12px;
    letter-spacing: 0.02em;
    white-space: nowrap;
  }
  .status-pill.active {
    background: #e6fffa;
    color: #234e52;
    border: 1px solid #b2f5ea;
  }
  .status-pill.inactive {
    background: #f1f5f9;
    color: #64748b;
    border: 1px solid #e2e8f0;
  }
  .status-pill.warning {
    background: #fffaf0;
    color: #744210;
    border: 1px solid #feebc8;
  }

  /* Metrics Strip */
  .gov-metrics-strip {
    display: flex;
    align-items: center;
    background: #ffffff;
    border: 1px solid var(--wa-border, #e2e8f0);
    border-radius: 8px;
    padding: 12px 20px;
    gap: 16px;
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.02);
  }

  .gov-metric-item {
    flex: 1;
    min-width: 140px;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .gov-metric-label {
    font-size: 11px;
    color: var(--wa-text-muted, #718096);
    font-weight: 650;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .gov-metric-val {
    display: flex;
    align-items: baseline;
    gap: 6px;
    flex-wrap: wrap;
  }

  .gov-metric-val strong {
    font-size: 17px;
    font-weight: 700;
    color: var(--wa-text-strong, #0d1722);
  }

  .gov-metric-sub {
    font-size: 11px;
    color: var(--wa-text-subtle, #8a99aa);
  }

  .gov-metric-divider {
    width: 1px;
    height: 28px;
    background: var(--wa-border, #e2e8f0);
  }

  .highlight-cyan { color: var(--wa-accent-strong, #006f76) !important; }
  .highlight-green { color: #166534 !important; }
  .highlight-gray { color: var(--wa-text-subtle, #8a99aa) !important; }

  /* Standard Gov Card Surface (Single Layer, No Nested Cards) */
  .gov-card {
    background: #ffffff;
    border: 1px solid var(--wa-border, #e2e8f0);
    border-radius: 8px;
    padding: 18px 20px;
    display: flex;
    flex-direction: column;
    gap: 16px;
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.02);
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 16px;
    flex-wrap: wrap;
  }

  .title-wrap h4 {
    margin: 0;
    font-size: 15px;
    font-weight: 650;
    color: var(--wa-text-strong, #0d1722);
  }

  .sub-hint {
    font-size: 12.5px;
    color: var(--wa-text-muted, #718096);
    margin-top: 3px;
    display: block;
    line-height: 1.45;
  }

  .synced-tag {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 4px 10px;
    border-radius: 6px;
    font-size: 12px;
    font-weight: 600;
    background: #f0fdf4;
    color: #166534;
    border: 1px solid #bbf7d0;
  }

  /* Switches Container */
  .switches-container {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .master-switch-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    padding: 14px 16px;
    background: #f8fafc;
    border: 1px solid var(--wa-border, #e2e8f0);
    border-radius: 6px;
    transition: all 0.15s ease;
  }

  .master-switch-row.is-active {
    background: rgba(0, 143, 150, 0.04);
    border-color: rgba(0, 143, 150, 0.28);
  }

  .switch-heading {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .switch-name {
    font-size: 14px;
    font-weight: 650;
    color: var(--wa-text-strong, #0d1722);
  }

  .switch-badge {
    font-size: 11px;
    padding: 1px 6px;
    border-radius: 4px;
    font-weight: 600;
    background: #edf2f7;
    color: var(--wa-text-muted, #718096);
  }

  .master-switch-row.is-active .switch-badge {
    background: #e6fffa;
    color: #234e52;
  }

  .switch-desc {
    margin: 4px 0 0 0;
    font-size: 12.5px;
    color: var(--wa-text-muted, #718096);
    line-height: 1.45;
  }

  .phases-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
  }

  .phase-card {
    background: #ffffff;
    border: 1px solid var(--wa-border, #e2e8f0);
    border-radius: 6px;
    padding: 14px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    gap: 10px;
    transition: opacity 0.15s ease;
  }

  .phase-card.is-disabled {
    opacity: 0.55;
    background: #fbfcfd;
  }

  .phase-card-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
  }

  .phase-title-with-badge {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }

  .phase-title {
    font-size: 13.5px;
    font-weight: 650;
    color: var(--wa-text-strong, #0d1722);
  }

  .exclusive-badge {
    font-size: 10.5px;
    padding: 1px 5px;
    border-radius: 3px;
    font-weight: 600;
    background: #fffaf0;
    color: #744210;
    border: 1px solid #feebc8;
  }

  .phase-desc {
    margin: 0;
    font-size: 12px;
    color: var(--wa-text-muted, #718096);
    line-height: 1.45;
    flex: 1;
  }

  /* Installation Combo */
  .combo-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 14px;
  }

  .combo-card {
    background: #f8fafc;
    border: 1px solid var(--wa-border, #e2e8f0);
    border-radius: 6px;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .combo-header {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .combo-tag {
    font-size: 11px;
    font-weight: 700;
    padding: 1px 6px;
    border-radius: 4px;
    background: #edf2f7;
    color: var(--wa-text-muted, #718096);
    letter-spacing: 0.02em;
    white-space: nowrap;
  }

  .combo-title {
    font-size: 13.5px;
    font-weight: 650;
    color: var(--wa-text-strong, #0d1722);
  }

  .combo-desc {
    margin: 0;
    font-size: 12.5px;
    color: var(--wa-text-muted, #718096);
    line-height: 1.45;
    flex: 1;
  }

  .combo-actions {
    display: flex;
    gap: 8px;
    align-items: center;
    flex-wrap: wrap;
    margin-top: 4px;
  }

  .code-snippet-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    background: #0f172a;
    border-radius: 6px;
    padding: 6px 10px;
    margin-top: 4px;
  }

  .code-snippet-bar code {
    font-family: var(--wa-font-mono, monospace);
    font-size: 12px;
    color: #38bdf8;
    overflow-x: auto;
    white-space: nowrap;
    scrollbar-width: none;
  }

  /* Sub Sections */
  .sub-section {
    display: flex;
    flex-direction: column;
    gap: 12px;
    border-top: 1px solid var(--wa-border, #e2e8f0);
    padding-top: 14px;
  }

  .sub-section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px;
  }

  .sub-section-title-wrap {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .sub-section-title {
    margin: 0;
    font-size: 13.5px;
    font-weight: 650;
    color: var(--wa-text-strong, #0d1722);
  }

  .sub-section-hint {
    font-size: 12px;
    color: var(--wa-text-muted, #718096);
  }

  /* Skills Grid */
  .skills-cards-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
  }

  .skill-card {
    background: #ffffff;
    border: 1px solid var(--wa-border, #e2e8f0);
    border-radius: 6px;
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .skill-card-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .skill-title {
    font-size: 13.5px;
    font-weight: 650;
    color: var(--wa-text-strong, #0d1722);
  }

  .skill-ver {
    font-size: 11px;
    font-weight: 600;
    background: #f1f5f9;
    border: 1px solid #e2e8f0;
    padding: 1px 6px;
    border-radius: 4px;
    color: var(--wa-text-muted, #718096);
  }

  .skill-key code {
    font-size: 12px;
    color: var(--wa-accent-strong, #006f76);
  }

  .skill-desc {
    margin: 0;
    font-size: 12px;
    color: var(--wa-text-muted, #718096);
    line-height: 1.45;
    flex: 1;
  }

  .skill-tools-preview {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    align-items: center;
  }

  .tool-tag-label {
    font-size: 11px;
    color: var(--wa-text-subtle, #8a99aa);
  }

  .tool-chip {
    font-size: 11px;
    background: rgba(0, 143, 150, 0.08);
    color: var(--wa-accent-strong, #006f76);
    padding: 1px 5px;
    border-radius: 3px;
    font-family: var(--wa-font-mono, monospace);
  }

  .skill-card-actions {
    display: flex;
    gap: 8px;
    margin-top: 4px;
    border-top: 1px solid var(--wa-border, #edf2f7);
    padding-top: 10px;
    align-items: center;
  }

  /* Config Tabs */
  .config-tabs-nav {
    display: flex;
    gap: 6px;
    background: #f1f5f9;
    padding: 3px;
    border-radius: 6px;
  }

  .tab-btn {
    border: none;
    background: transparent;
    color: var(--wa-text-muted, #718096);
    font-size: 12px;
    font-weight: 600;
    padding: 4px 10px;
    border-radius: 4px;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .tab-btn.active {
    background: #ffffff;
    color: var(--wa-accent-strong, #006f76);
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
  }

  .config-code-preview {
    position: relative;
    background: #0f172a;
    border-radius: 6px;
    padding: 16px;
    color: #e2e8f0;
  }

  .file-hint {
    font-size: 12px;
    color: #94a3b8;
    margin-bottom: 8px;
  }

  .file-hint code {
    color: #38bdf8;
  }

  .config-code-preview pre {
    margin: 0;
    font-family: var(--wa-font-mono, monospace);
    font-size: 12.5px;
    line-height: 1.5;
    overflow-x: auto;
  }

  .code-action-pos {
    position: absolute;
    top: 12px;
    right: 12px;
  }

  /* Table Container & Table (Matching AIGovernanceCenter) */
  .gov-table-container {
    width: 100%;
    min-width: 0;
    max-width: 100%;
    background: #ffffff;
    border: 1px solid var(--wa-border, #e2e8f0);
    border-radius: 6px;
    overflow-x: auto;
    overflow-y: hidden;
  }

  .gov-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }

  .gov-table th {
    text-align: left;
    padding: 10px 14px;
    background: #f8fafc;
    border-bottom: 1px solid var(--wa-border, #e2e8f0);
    font-size: 11.5px;
    font-weight: 650;
    color: var(--wa-text-muted, #718096);
    letter-spacing: 0.03em;
  }

  .gov-table td {
    padding: 12px 14px;
    border-bottom: 1px solid var(--wa-border, #edf2f7);
    vertical-align: middle;
  }

  .gov-table tbody tr:hover {
    background: #fafcff;
  }

  .mono-tag {
    font-family: var(--wa-font-mono, monospace);
    font-size: 12px;
    font-weight: 600;
    color: var(--wa-text-strong, #0d1722);
  }

  .mono-prefix {
    font-family: var(--wa-font-mono, monospace);
    font-size: 12px;
    color: var(--wa-text-muted, #718096);
  }

  .source-tag {
    font-size: 11.5px;
    font-weight: 550;
    background: #f1f5f9;
    padding: 2px 7px;
    border-radius: 4px;
    color: var(--wa-text-main, #293847);
  }

  .empty-box {
    padding: 30px 16px;
    text-align: center;
    color: var(--wa-text-muted, #718096);
    font-size: 13px;
    background: #f8fafc;
    border: 1px dashed var(--wa-border, #cbd5e0);
    border-radius: 6px;
  }

  .empty-box p {
    margin: 0;
  }

  /* Modal Form Content */
  .modal-form-body {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .form-item {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .form-item label {
    font-size: 13px;
    font-weight: 650;
    color: var(--wa-text-strong, #0d1722);
  }

  .gov-input, .gov-select {
    width: 100%;
    box-sizing: border-box;
    padding: 8px 12px;
    border: 1px solid var(--wa-border, #cbd5e0);
    border-radius: 6px;
    font-size: 13px;
    background: #ffffff;
    color: var(--wa-text-main, #293847);
    outline: none;
    transition: border-color 0.15s ease, box-shadow 0.15s ease;
  }

  .gov-input:focus, .gov-select:focus {
    border-color: var(--wa-accent, #008f96);
    box-shadow: 0 0 0 3px rgba(0, 143, 150, 0.15);
  }

  .field-hint {
    font-size: 12px;
    color: var(--wa-text-muted, #718096);
  }

  .token-result-pane {
    background: #f8fafc;
    border: 1px solid var(--wa-border, #e2e8f0);
    border-radius: 6px;
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .token-copy-row {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .mono-key-input {
    flex: 1;
    font-family: var(--wa-font-mono, monospace);
    font-size: 12.5px;
    padding: 8px 10px;
    border: 1px solid var(--wa-border, #cbd5e0);
    border-radius: 6px;
    background: #ffffff;
  }

  .token-tip {
    margin: 0;
    font-size: 12px;
    color: var(--wa-text-muted, #718096);
  }

  .modal-footer-actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
  }

  /* Responsive Rules (Finesse UI & Impeccable) */
  @media (max-width: 1024px) {
    .phases-grid {
      grid-template-columns: 1fr;
    }
    .skills-cards-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (max-width: 760px) {
    .open-gov-header {
      flex-direction: column;
    }
    .gov-metrics-strip {
      flex-direction: column;
      align-items: stretch;
      gap: 12px;
    }
    .gov-metric-divider {
      display: none;
    }
    .combo-grid {
      grid-template-columns: 1fr;
    }
    .skills-cards-grid {
      grid-template-columns: 1fr;
    }
    .config-tabs-nav {
      flex-wrap: wrap;
    }
  }
</style>
