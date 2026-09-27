<script lang="ts">
  import { onMount } from 'svelte';
  import { showToast } from '../../lib/toast';

  export let canWrite = false;

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

  $: currentBaseURL = overview?.base_url || window.location.origin;
  $: currentMcpURL = overview?.mcp?.endpoint_url || `${currentBaseURL}/mcp`;
  $: effectiveToken = selectedToken || '<YOUR_API_KEY>';

  // Deep Link for Cursor
  $: cursorDeepLink = `cursor://anysphere.cursor-mcp/install?name=well-ambient&url=${encodeURIComponent(currentMcpURL)}&headers=${encodeURIComponent(JSON.stringify({ Authorization: `Bearer ${effectiveToken}` }))}`;

  // Install command
  $: installCommand = `curl -fsSL "${currentBaseURL}/open/v1/install.sh?token=${effectiveToken}" | bash`;

  async function api(path: string, init: RequestInit = {}) {
    const headers = new Headers(init.headers || {});
    const token = localStorage.getItem('jwt_token');
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
      // Select the first active key if available
      const activeKeys = (data.credentials || []).filter(c => c.status === 'active');
      if (activeKeys.length > 0 && !selectedToken) {
        // We only have the prefix, so we don't overwrite full token unless user creates one
      }
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
      const res = await api('/api/ai-governance/open-capabilities/config', {
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
      showToast('API Key 签发成功，已自动填入安装指引！', { type: 'success' });
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
    if (navigator.clipboard) {
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
      <strong>加载异常：</strong> {error}
      <button type="button" class="btn btn-sm btn-ghost" on:click={loadOverview}>重试</button>
    </div>
  {:else if overview}
    <!-- Status & Metrics Header -->
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
        <button type="button" class="btn btn-ghost btn-sm" on:click={loadOverview} title="刷新状态">
          刷新概览
        </button>
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

      <div class="gov-metric-divider"></div>

      <div class="gov-metric-item">
        <span class="gov-metric-label">MCP 工具矩阵</span>
        <div class="gov-metric-val">
          <strong>{overview.mcp.tools.length} 项工具</strong>
          <span class="gov-metric-sub">{overview.mcp.server_name}</span>
        </div>
      </div>

      <div class="gov-metric-divider"></div>

      <div class="gov-metric-item">
        <span class="gov-metric-label">已签发有效 Key</span>
        <div class="gov-metric-val">
          <strong>{(overview.credentials || []).filter(c => c.status === 'active').length} 个</strong>
          <span class="gov-metric-sub">共 {(overview.sources || []).length} 个集成来源</span>
        </div>
      </div>

      <div class="gov-metric-divider"></div>

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
    <div class="gov-card">
      <div class="card-header">
        <div class="title-wrap">
          <h4>分阶段能力动态开关 (Runtime Feature Switches)</h4>
          <span class="sub-hint">脱离离线环境变量与文件，变更即时广播至全集群实例，无需重启进程。</span>
        </div>
        {#if canWrite}
          <div class="save-bar">
            <button
              type="button"
              class="btn btn-primary"
              disabled={!isDirty || saving}
              on:click={saveConfig}
            >
              {saving ? '正在保存…' : (isDirty ? '保存并应用动态配置*' : '配置已同步')}
            </button>
          </div>
        {/if}
      </div>

      {#if !canWrite}
        <div class="gov-notice info">
          当前账户具有查看权限；修改运行时开关或签发 API Key 需要管理员 <code>config:write</code> 权限。
        </div>
      {/if}

      <div class="switches-grid">
        <!-- Main Switch -->
        <div class="switch-box main-switch" class:enabled={draftConfig.enabled}>
          <div class="switch-info">
            <div class="switch-title">全局开放能力主控</div>
            <p>总控总闸。开启后激活 <code>/mcp</code> 协议端点与 <code>/open/v1/*</code> 接口体系。</p>
          </div>
          <label class="toggle-switch">
            <input
              type="checkbox"
              bind:checked={draftConfig.enabled}
              disabled={!canWrite}
            />
            <span class="slider round"></span>
          </label>
        </div>

        <!-- Read Switch -->
        <div class="switch-box" class:disabled={!draftConfig.enabled}>
          <div class="switch-info">
            <div class="switch-title">只读分析能力 (Read Phase)</div>
            <p>Jira 权威投影多维检索/聚合、字段规约感知与 GitLab 代码评审过滤读取。</p>
          </div>
          <label class="toggle-switch">
            <input
              type="checkbox"
              bind:checked={draftConfig.read_enabled}
              disabled={!canWrite || !draftConfig.enabled}
            />
            <span class="slider round"></span>
          </label>
        </div>

        <!-- Prepare Switch -->
        <div class="switch-box" class:disabled={!draftConfig.enabled}>
          <div class="switch-info">
            <div class="switch-title">决策预备能力 (Prepare Phase)</div>
            <p>冻结带远端前置校验与时间戳的单事项决策执行计划，生成确定性 Plan ID。</p>
          </div>
          <label class="toggle-switch">
            <input
              type="checkbox"
              bind:checked={draftConfig.prepare_enabled}
              disabled={!canWrite || !draftConfig.enabled}
            />
            <span class="slider round"></span>
          </label>
        </div>

        <!-- Execute Switch -->
        <div class="switch-box warning" class:disabled={!draftConfig.enabled}>
          <div class="switch-info">
            <div class="switch-title">决策执行写入 (Execute Phase)</div>
            <p>执行已冻结的决策计划写入 Jira。<strong>开启后系统将自动互斥接管旧版转派改期通道</strong>。</p>
          </div>
          <label class="toggle-switch">
            <input
              type="checkbox"
              bind:checked={draftConfig.execute_enabled}
              disabled={!canWrite || !draftConfig.enabled}
            />
            <span class="slider round"></span>
          </label>
        </div>
      </div>
    </div>

    <!-- Section 2: Online Client Installation Combo (Core Feature) -->
    <div class="gov-card highlight-card">
      <div class="card-header">
        <div class="title-wrap">
          <h4>🚀 远程客户端接入与在线安装组合拳</h4>
          <span class="sub-hint">采用“系统协议唤起 + 远程安装脚本 + 在线 Skill 仓库 + 规范配置导出”四大通路无缝接入。</span>
        </div>
      </div>

      <!-- Quick Action 1: Deep Link -->
      <div class="combo-grid">
        <div class="combo-box deep-link-box">
          <div class="combo-icon">⚡</div>
          <div class="combo-content">
            <div class="combo-title">通道一：Cursor 协议一键拉起 (Deep Link)</div>
            <p>点击后直接通过系统 URL Scheme 拉起本地 Cursor 并一键添加 <code>well-ambient</code> MCP Server。</p>
            <div class="action-row">
              <a
                href={cursorDeepLink}
                class="btn btn-primary btn-sm"
                target="_blank"
                rel="noreferrer"
              >
                在 Cursor 中一键安装 MCP
              </a>
              <button
                type="button"
                class="btn btn-ghost btn-sm"
                on:click={() => copyText(cursorDeepLink, 'Cursor 深链')}
              >
                复制深链
              </button>
            </div>
          </div>
        </div>

        <!-- Quick Action 2: Remote Install Script -->
        <div class="combo-box install-script-box">
          <div class="combo-icon">📦</div>
          <div class="combo-content">
            <div class="combo-title">通道二：远程安装脚本 (One-line Bash Installer)</div>
            <p>一行命令自动探查本地 Claude Desktop 与 Cursor 环境，安全合并配置并解压官方 Skill：</p>
            <div class="code-snippet-row">
              <code>{installCommand}</code>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                on:click={() => copyText(installCommand, '一键安装命令')}
              >
                复制命令
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Quick Action 3: Official Skill Registry -->
      <div class="skills-registry-section">
        <div class="section-sub-header">
          <span class="sub-title">通道三：官方微内核 Skill 在线仓库 (Online Skill Registry)</span>
          <span class="sub-desc">服务端动态打包分发，支持标准 Agent 架构与 DSH / Codex 离线解构：</span>
        </div>

        <div class="skills-cards-grid">
          {#each overview.skills as skill}
            <div class="skill-registry-card">
              <div class="skill-card-top">
                <span class="skill-title">{skill.title}</span>
                <span class="skill-ver">v{skill.version}</span>
              </div>
              <div class="skill-name-code"><code>{skill.name}</code></div>
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
                  class="btn btn-sm btn-ghost"
                  download
                  title="下载离线 tar.gz 安装包"
                >
                  下载 .tar.gz
                </a>
                <button
                  type="button"
                  class="btn btn-sm btn-ghost"
                  on:click={() => copyText(`curl -fsSL "${skill.archive_url}" | tar -xz -C .agents/skills`, '安装命令')}
                >
                  复制安装指令
                </button>
              </div>
            </div>
          {/each}
        </div>
      </div>

      <!-- Quick Action 4: Manual Config JSON Tabs -->
      <div class="manual-config-section">
        <div class="section-sub-header">
          <span class="sub-title">通道四：主流客户端手动配置规范 (Manual MCP JSON)</span>
          <div class="config-tabs-nav">
            <button
              type="button"
              class="tab-btn"
              class:active={activeConfigTab === 'cursor'}
              on:click={() => activeConfigTab = 'cursor'}
            >
              Cursor
            </button>
            <button
              type="button"
              class="tab-btn"
              class:active={activeConfigTab === 'claude'}
              on:click={() => activeConfigTab = 'claude'}
            >
              Claude Desktop
            </button>
            <button
              type="button"
              class="tab-btn"
              class:active={activeConfigTab === 'windsurf'}
              on:click={() => activeConfigTab = 'windsurf'}
            >
              Windsurf
            </button>
            <button
              type="button"
              class="tab-btn"
              class:active={activeConfigTab === 'cline'}
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
            <button
              type="button"
              class="btn btn-sm btn-secondary copy-code-btn"
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
            </button>
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
            <button
              type="button"
              class="btn btn-sm btn-secondary copy-code-btn"
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
            </button>
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
            <button
              type="button"
              class="btn btn-sm btn-secondary copy-code-btn"
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
            </button>
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
            <button
              type="button"
              class="btn btn-sm btn-secondary copy-code-btn"
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
            </button>
          {/if}
        </div>
      </div>
    </div>

    <!-- Section 3: Credentials & Integration Key Lifecycle -->
    <div class="gov-card">
      <div class="card-header">
        <div class="title-wrap">
          <h4>集成凭证 (API Keys) 生命周期管理</h4>
          <span class="sub-hint">签发带有效期的集成凭证，所有密钥共享来源租约配额与安全审计。</span>
        </div>
        {#if canWrite}
          <button
            type="button"
            class="btn btn-primary btn-sm"
            on:click={() => { showIssueModal = true; issuedKeyResult = null; }}
          >
            + 签发新 API Key
          </button>
        {/if}
      </div>

      {#if (overview.credentials || []).length === 0}
        <div class="empty-keys-box">
          <p>暂无已签发的 API Key。点击右上角“+ 签发新 API Key”为外部 Agent 或 MCP 客户端创建访问密钥。</p>
        </div>
      {:else}
        <div class="table-wrapper">
          <table class="wa-admin-table" aria-label="API Key 凭证列表">
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
                  <td><code>{cred.key_id}</code></td>
                  <td><code>{cred.prefix}...</code></td>
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
                      <button
                        type="button"
                        class="btn btn-ghost btn-sm text-danger"
                        disabled={revokingKeyID === cred.key_id}
                        on:click={() => revokeKey(cred.key_id)}
                      >
                        {revokingKeyID === cred.key_id ? '撤销中…' : '撤销'}
                      </button>
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
    </div>

    <!-- Section 4: Jira Bindings & Execution Protection -->
    <div class="gov-card">
      <div class="card-header">
        <div class="title-wrap">
          <h4>Jira 写入执行绑定 (Execution Bindings)</h4>
          <span class="sub-hint">定义单事项排期决策被执行时，所使用的 Jira 服务账号与连接器。</span>
        </div>
      </div>

      {#if (overview.bindings || []).length === 0}
        <div class="empty-keys-box">
          <p>暂无针对项目的执行绑定。请在 CLI 或策略中为项目配置 <code>reassign</code> 与 <code>reschedule</code> 绑定。</p>
        </div>
      {:else}
        <div class="table-wrapper">
          <table class="wa-admin-table" aria-label="Jira 写入执行绑定">
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
    </div>
  {/if}

  <!-- Issue New Key Modal -->
  {#if showIssueModal}
    <div class="modal-backdrop" role="presentation" on:click={() => showIssueModal = false}></div>
    <div class="gov-modal" role="dialog" aria-modal="true" aria-label="签发新 API Key">
      <div class="modal-header">
        <h2>签发开放能力集成 API Key</h2>
        <button type="button" class="close-btn" aria-label="关闭对话框" on:click={() => showIssueModal = false}>×</button>
      </div>

      <div class="modal-body">
        {#if issuedKeyResult}
          <div class="gov-notice success">
            <strong>✓ 密钥生成成功！</strong> 请务必立即复制并保存，该密钥只会完整展示一次。
          </div>

          <div class="token-result-box">
            <label for="new-issued-key">完整 API Key (Bearer Token):</label>
            <div class="token-copy-row">
              <input
                id="new-issued-key"
                type="text"
                readonly
                value={issuedKeyResult.key}
              />
              <button
                type="button"
                class="btn btn-primary"
                on:click={() => copyText(issuedKeyResult?.key || '', '完整 API Key')}
              >
                复制 Key
              </button>
            </div>
            <p class="token-tip">该 Key 已自动同步至上方的远程安装命令与配置示例中。</p>
          </div>
        {:else}
          <div class="form-group">
            <label for="issue-source-id">集成来源标识 (Source ID):</label>
            <input
              id="issue-source-id"
              type="text"
              bind:value={newKeySourceID}
              placeholder="例如 target-agent, cursor-dev, ai-platform"
            />
            <span class="field-hint">对应集成方身份，同一来源下的多个轮换 Key 共享统一配额。</span>
          </div>

          <div class="form-group">
            <label for="issue-ttl">有效期 (TTL 时长):</label>
            <select id="issue-ttl" bind:value={newKeyTTLHours}>
              <option value={168}>7 天 (168h)</option>
              <option value={720}>30 天 (720h - 默认推荐)</option>
              <option value={2160}>90 天 (2160h)</option>
              <option value={8760}>1 年 (8760h)</option>
            </select>
          </div>
        {/if}
      </div>

      <div class="modal-footer">
        {#if issuedKeyResult}
          <button type="button" class="btn btn-primary" on:click={() => showIssueModal = false}>
            完成并关闭
          </button>
        {:else}
          <button type="button" class="btn btn-ghost" on:click={() => showIssueModal = false}>
            取消
          </button>
          <button
            type="button"
            class="btn btn-primary"
            disabled={issuingKey || !newKeySourceID.trim()}
            on:click={issueNewKey}
          >
            {issuingKey ? '正在签发…' : '确认签发'}
          </button>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .open-capabilities-view {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .gov-loading-pane {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 60px 20px;
    color: var(--wa-text-muted);
    gap: 12px;
  }

  .spinner {
    width: 24px;
    height: 24px;
    border: 2px solid var(--wa-border-soft);
    border-top-color: var(--wa-accent);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .open-gov-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 16px;
    background: var(--wa-chrome-0);
    padding: 16px 20px;
    border-radius: 12px;
    border: 1px solid var(--wa-border-soft);
  }

  .header-title-row {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
  }

  .header-title-row h3 {
    margin: 0;
    font-size: 1.15rem;
    font-weight: 600;
    color: var(--wa-text-strong);
  }

  .header-desc {
    margin: 6px 0 0 0;
    font-size: 0.88rem;
    color: var(--wa-text-muted);
    line-height: 1.45;
  }

  /* Metrics Strip */
  .gov-metrics-strip {
    display: flex;
    align-items: center;
    background: var(--wa-chrome-0);
    border: 1px solid var(--wa-border-soft);
    border-radius: 10px;
    padding: 12px 20px;
    gap: 16px;
    flex-wrap: wrap;
  }

  .gov-metric-item {
    flex: 1;
    min-width: 140px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .gov-metric-label {
    font-size: 0.76rem;
    color: var(--wa-text-muted);
    font-weight: 500;
  }

  .gov-metric-val {
    display: flex;
    align-items: baseline;
    gap: 6px;
  }

  .gov-metric-val strong {
    font-size: 1.1rem;
    color: var(--wa-text-strong);
  }

  .gov-metric-sub {
    font-size: 0.76rem;
    color: var(--wa-text-subtle);
  }

  .gov-metric-divider {
    width: 1px;
    height: 28px;
    background: var(--wa-border-divider);
  }

  .highlight-cyan { color: var(--wa-palette-primary) !important; }
  .highlight-green { color: var(--wa-palette-success) !important; }
  .highlight-gray { color: var(--wa-text-subtle) !important; }

  /* Gov Card */
  .gov-card {
    background: var(--wa-chrome-0);
    border: 1px solid var(--wa-border-soft);
    border-radius: 12px;
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .highlight-card {
    border-color: rgba(1, 139, 141, 0.35);
    background: linear-gradient(180deg, rgba(248, 251, 254, 0.95) 0%, rgba(255, 255, 255, 0.98) 100%);
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
    font-size: 1rem;
    font-weight: 600;
    color: var(--wa-text-strong);
  }

  .sub-hint {
    font-size: 0.82rem;
    color: var(--wa-text-muted);
    margin-top: 4px;
    display: block;
  }

  /* Switches Grid */
  .switches-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 14px;
  }

  .switch-box {
    background: var(--wa-surface-flat);
    border: 1px solid var(--wa-border-soft);
    border-radius: 8px;
    padding: 14px 16px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    transition: border-color 0.2s;
  }

  .switch-box.main-switch {
    grid-column: 1 / -1;
    background: rgba(1, 139, 141, 0.04);
    border-color: rgba(1, 139, 141, 0.25);
  }

  .switch-box.disabled {
    opacity: 0.55;
    background: var(--wa-surface-inset);
  }

  .switch-info {
    flex: 1;
  }

  .switch-title {
    font-size: 0.92rem;
    font-weight: 600;
    color: var(--wa-text-strong);
  }

  .switch-info p {
    margin: 4px 0 0 0;
    font-size: 0.8rem;
    color: var(--wa-text-muted);
    line-height: 1.4;
  }

  /* Toggle Switch Control */
  .toggle-switch {
    position: relative;
    display: inline-block;
    width: 44px;
    height: 24px;
    flex-shrink: 0;
  }

  .toggle-switch input {
    opacity: 0;
    width: 0;
    height: 0;
  }

  .slider {
    position: absolute;
    cursor: pointer;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: #cbd5e1;
    transition: 0.25s;
    border-radius: 24px;
  }

  .slider:before {
    position: absolute;
    content: "";
    height: 18px;
    width: 18px;
    left: 3px;
    bottom: 3px;
    background-color: white;
    transition: 0.25s;
    border-radius: 50%;
  }

  input:checked + .slider {
    background-color: var(--wa-palette-primary);
  }

  input:checked + .slider:before {
    transform: translateX(20px);
  }

  /* Combo Grid */
  .combo-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
    gap: 16px;
  }

  .combo-box {
    background: var(--wa-surface-flat);
    border: 1px solid var(--wa-border-soft);
    border-radius: 10px;
    padding: 16px;
    display: flex;
    gap: 14px;
  }

  .combo-icon {
    font-size: 1.6rem;
    line-height: 1;
  }

  .combo-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .combo-title {
    font-size: 0.95rem;
    font-weight: 600;
    color: var(--wa-text-strong);
  }

  .combo-content p {
    margin: 0;
    font-size: 0.82rem;
    color: var(--wa-text-muted);
    line-height: 1.45;
  }

  .action-row {
    display: flex;
    gap: 8px;
    margin-top: 6px;
  }

  .code-snippet-row {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--wa-surface-inset);
    border: 1px solid var(--wa-border-soft);
    border-radius: 6px;
    padding: 6px 10px;
    margin-top: 6px;
    overflow-x: auto;
  }

  .code-snippet-row code {
    flex: 1;
    font-family: var(--wa-font-mono);
    font-size: 0.78rem;
    color: var(--wa-palette-info-deep);
    white-space: nowrap;
  }

  /* Skills Registry Cards */
  .skills-registry-section, .manual-config-section {
    display: flex;
    flex-direction: column;
    gap: 12px;
    margin-top: 8px;
  }

  .section-sub-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
  }

  .sub-title {
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--wa-text-strong);
  }

  .sub-desc {
    font-size: 0.8rem;
    color: var(--wa-text-muted);
  }

  .skills-cards-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 14px;
  }

  .skill-registry-card {
    background: var(--wa-surface-flat);
    border: 1px solid var(--wa-border-soft);
    border-radius: 8px;
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
    font-size: 0.92rem;
    font-weight: 600;
    color: var(--wa-text-strong);
  }

  .skill-ver {
    font-size: 0.72rem;
    background: var(--wa-surface-inset);
    border: 1px solid var(--wa-border-soft);
    padding: 2px 6px;
    border-radius: 4px;
    color: var(--wa-text-muted);
  }

  .skill-name-code code {
    font-size: 0.78rem;
    color: var(--wa-palette-primary);
  }

  .skill-desc {
    margin: 0;
    font-size: 0.8rem;
    color: var(--wa-text-muted);
    line-height: 1.4;
    flex: 1;
  }

  .skill-tools-preview {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    align-items: center;
  }

  .tool-tag-label {
    font-size: 0.72rem;
    color: var(--wa-text-subtle);
  }

  .tool-chip {
    font-size: 0.7rem;
    background: rgba(1, 139, 141, 0.08);
    color: var(--wa-palette-primary);
    padding: 1px 5px;
    border-radius: 3px;
    font-family: var(--wa-font-mono);
  }

  .skill-card-actions {
    display: flex;
    gap: 8px;
    margin-top: 4px;
    border-top: 1px solid var(--wa-border-divider);
    padding-top: 8px;
  }

  /* Manual Config Tabs */
  .config-tabs-nav {
    display: flex;
    gap: 6px;
  }

  .tab-btn {
    border: none;
    background: var(--wa-surface-inset);
    color: var(--wa-text-muted);
    font-size: 0.8rem;
    padding: 4px 10px;
    border-radius: 6px;
    cursor: pointer;
  }

  .tab-btn.active {
    background: var(--wa-palette-primary);
    color: white;
    font-weight: 500;
  }

  .config-code-preview {
    position: relative;
    background: #0f172a;
    border-radius: 8px;
    padding: 14px 16px;
    color: #e2e8f0;
  }

  .file-hint {
    font-size: 0.76rem;
    color: #94a3b8;
    margin-bottom: 8px;
  }

  .file-hint code {
    color: #38bdf8;
  }

  .config-code-preview pre {
    margin: 0;
    font-family: var(--wa-font-mono);
    font-size: 0.82rem;
    line-height: 1.45;
    overflow-x: auto;
  }

  .copy-code-btn {
    position: absolute;
    top: 10px;
    right: 12px;
  }

  /* Status Pills */
  .status-pill {
    font-size: 0.72rem;
    padding: 2px 8px;
    border-radius: 12px;
    font-weight: 500;
  }

  .status-pill.active {
    background: rgba(110, 204, 84, 0.15);
    color: #2b7a15;
  }

  .status-pill.inactive {
    background: rgba(148, 163, 184, 0.2);
    color: var(--wa-text-muted);
  }

  .status-pill.warning {
    background: rgba(235, 92, 32, 0.15);
    color: #b43b08;
  }

  /* Tables */
  .table-wrapper {
    overflow-x: auto;
    border: 1px solid var(--wa-border-soft);
    border-radius: 8px;
  }

  .wa-admin-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.84rem;
    text-align: left;
  }

  .wa-admin-table th, .wa-admin-table td {
    padding: 10px 14px;
    border-bottom: 1px solid var(--wa-border-divider);
  }

  .wa-admin-table th {
    background: var(--wa-surface-inset);
    color: var(--wa-text-muted);
    font-weight: 600;
    font-size: 0.78rem;
  }

  .source-tag {
    background: var(--wa-surface-inset);
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 0.76rem;
  }

  .text-danger { color: var(--wa-palette-danger) !important; }

  /* Modals */
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(15, 23, 42, 0.5);
    z-index: 999;
  }

  .gov-modal {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    background: var(--wa-surface-flat);
    border-radius: 12px;
    box-shadow: 0 20px 40px rgba(0, 0, 0, 0.2);
    width: 90%;
    max-width: 520px;
    z-index: 1000;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px;
    border-bottom: 1px solid var(--wa-border-divider);
  }

  .modal-header h2 {
    margin: 0;
    font-size: 1.1rem;
    font-weight: 600;
  }

  .close-btn {
    border: none;
    background: transparent;
    font-size: 1.5rem;
    cursor: pointer;
    color: var(--wa-text-muted);
  }

  .modal-body {
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
    padding: 14px 20px;
    border-top: 1px solid var(--wa-border-divider);
    background: var(--wa-surface-inset);
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .form-group label {
    font-size: 0.84rem;
    font-weight: 600;
    color: var(--wa-text-strong);
  }

  .form-group input, .form-group select {
    padding: 8px 12px;
    border: 1px solid var(--wa-border-soft);
    border-radius: 6px;
    font-size: 0.88rem;
  }

  .field-hint {
    font-size: 0.76rem;
    color: var(--wa-text-muted);
  }

  .token-result-box {
    background: var(--wa-surface-inset);
    border: 1px solid var(--wa-border-soft);
    border-radius: 8px;
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .token-copy-row {
    display: flex;
    gap: 8px;
  }

  .token-copy-row input {
    flex: 1;
    font-family: var(--wa-font-mono);
    font-size: 0.84rem;
    padding: 8px 10px;
  }

  .token-tip {
    margin: 0;
    font-size: 0.78rem;
    color: var(--wa-text-muted);
  }

  /* Responsive Rules (Finesse UI Gate) */
  @media (max-width: 760px) {
    .open-gov-header {
      flex-direction: column;
    }
    .gov-metrics-strip {
      flex-direction: column;
      align-items: stretch;
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
  }
</style>
