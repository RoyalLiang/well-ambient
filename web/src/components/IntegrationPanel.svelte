<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import StatusCard from './StatusCard.svelte';
  import Button from './shared/Button.svelte';

  const dispatch = createEventDispatcher();

  export let currentUserRole = 'member';
  export let currentUserPermissions: string[] = [];

  interface StatusResponse {
    status: string;
    version: string;
    telemetry: {
      active_hooks: number;
    };
  }

  interface GlobalConfig {
    server: { host: string; port: number };
    gitlab: {
      base_url: string;
      secret_token: string;
      repos: Array<{ name: string; path: string; project_id: string }>;
    };
    feishu: {
      app_id: string;
      app_secret: string;
      bot: { enabled: boolean; chat_group: string };
      bitable: {
        enabled: boolean;
        app_token: string;
        table_id: string;
        status_column: string;
        task_id_column: string;
      };
    };
    jira: {
      enabled: boolean;
      base_url: string;
      username: string;
      api_token: string;
      sync_projects?: string[];
      sync_users?: string[];
      sync_statuses?: string[];
      custom_jql?: string;
    };
    ai: {
      enabled: boolean;
      provider: string;
      base_url: string;
      endpoint_type?: string;
      api_token: string;
      model: string;
      project_architecture?: string;
      delivery_workflow?: string;
      implemented_features?: string;
      estimation_guidelines?: string;
      default_work_hours_per_day?: number;
    };
  }

  let serverStatus: 'online' | 'offline' | 'warning' = 'online';
  let activeHooks = 0;
  let intervalId: any;

  // Active configuration loaded from /api/config
  let globalConfig: GlobalConfig = {
    server: { host: '', port: 0 },
    gitlab: { base_url: '', secret_token: '', repos: [] },
    feishu: {
      app_id: '',
      app_secret: '',
      bot: { enabled: false, chat_group: '' },
      bitable: { enabled: false, app_token: '', table_id: '', status_column: '', task_id_column: '' }
    },
    jira: { enabled: false, base_url: '', username: '', api_token: '', sync_projects: [], sync_users: [], sync_statuses: [], custom_jql: '' },
    ai: {
      enabled: false,
      provider: 'openai',
      base_url: '',
      endpoint_type: 'responses',
      api_token: '',
      model: '',
      project_architecture: '',
      delivery_workflow: '',
      implemented_features: '',
      estimation_guidelines: '',
      default_work_hours_per_day: 8
    }
  };

  // Modal display states
  let activeModal: 'gitlab' | 'feishu' | 'jira' | 'ai' | null = null;
  let saving = false;
  let saveError = '';
  let saveSuccess = false;

  async function fetchStatus() {
    try {
      const res = await fetch('/api/status');
      if (!res.ok) throw new Error('Offline');
      const data: StatusResponse = await res.json();
      serverStatus = data.status === 'online' ? 'online' : 'warning';
      activeHooks = data.telemetry?.active_hooks ?? 0;
    } catch (e) {
      serverStatus = 'offline';
      activeHooks = 0;
    }
  }

  async function fetchConfig() {
    try {
      const res = await fetch('/api/config');
      if (!res.ok) throw new Error('Failed to fetch config');
      const data = await res.json();
      if (data) {
        globalConfig = data;
      }
    } catch (e) {
      console.error('Failed to load settings', e);
    }
  }

  async function handleSaveConfig(event: CustomEvent<{ key: string; data: any }>) {
    const { key, data } = event.detail;
    const newConfig = {
      ...globalConfig,
      [key]: data
    };

    saving = true;
    saveError = '';
    saveSuccess = false;

    try {
      const res = await fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newConfig)
      });
      if (!res.ok) throw new Error('Failed to save config');
      const result = await res.json();
      if (result.success) {
        globalConfig = newConfig;
        saveSuccess = true;
        fetchStatus();
        if (typeof window !== 'undefined') {
          window.dispatchEvent(new CustomEvent('config-updated', { detail: newConfig }));
        }
      } else {
        saveError = '保存配置失败: ' + result.message;
      }
    } catch (e: any) {
      saveError = '网络请求失败: ' + e.message;
    } finally {
      saving = false;
    }
  }

  onMount(async () => {
    await fetchConfig();
    await fetchStatus();
    intervalId = setInterval(fetchStatus, 5000);
  });

  onDestroy(() => {
    if (intervalId) {
      clearInterval(intervalId);
    }
  });

  // Dynamically resolve integration cards details
  $: integrations = [
    {
      id: 'gitlab' as const,
      title: 'GitLab',
      status: globalConfig.gitlab.base_url ? serverStatus : ('offline' as const),
      description: globalConfig.gitlab.base_url
        ? `已成功对接 GitLab System Webhook。共监控 ${globalConfig.gitlab.repos?.length || 0} 个多仓项目，自动捕获 branch 与 commit。`
        : '未配置 GitLab 系统 Webhook 连接。请点击配置开始同步。',
      details: globalConfig.gitlab.base_url 
        ? `${globalConfig.gitlab.base_url.replace(/^https?:\/\//, '')} | active_hooks: ${activeHooks}` 
        : '未绑定服务地址'
    },
    {
      id: 'feishu-bot' as const,
      title: '飞书机器人',
      status: !globalConfig.feishu.app_id 
        ? ('offline' as const) 
        : (globalConfig.feishu.bot?.enabled ? ('online' as const) : ('warning' as const)),
      description: !globalConfig.feishu.app_id
        ? '飞书凭证尚未配置，消息推送未激活。'
        : (globalConfig.feishu.bot?.enabled 
            ? `消息推送与交互卡片已启用。默认通知群组：${globalConfig.feishu.bot.chat_group || '未设置'}`
            : '飞书机器人推送已禁用。'),
      details: globalConfig.feishu.app_id 
        ? `app_id: ${globalConfig.feishu.app_id.substring(0, 10)}... | bot: ${globalConfig.feishu.bot?.enabled ? 'enabled' : 'disabled'}` 
        : '凭证未配置'
    },
    {
      id: 'feishu-bitable' as const,
      title: '飞书多维表格 (Bitable)',
      status: !globalConfig.feishu.app_id 
        ? ('offline' as const) 
        : (globalConfig.feishu.bitable?.enabled ? ('online' as const) : ('warning' as const)),
      description: !globalConfig.feishu.app_id
        ? '飞书凭证尚未配置，多维表格同步未激活。'
        : (globalConfig.feishu.bitable?.enabled
            ? `多维表格同步正常。映射状态列 [${globalConfig.feishu.bitable.status_column}] 与任务 ID 列 [${globalConfig.feishu.bitable.task_id_column}]。`
            : '多维表看板自动同步已禁用。'),
      details: globalConfig.feishu.bitable?.enabled 
        ? `table_id: ${globalConfig.feishu.bitable.table_id || '未配置'}` 
        : '同步未激活'
    },
    {
      id: 'jira' as const,
      title: 'Jira API',
      status: globalConfig.jira?.enabled ? ('online' as const) : ('offline' as const),
      description: globalConfig.jira?.enabled
        ? `Jira 自动回写与关联功能正常。正在监听匹配用户名: ${globalConfig.jira.username}`
        : 'Jira 连接同步未启用。',
      details: globalConfig.jira?.enabled 
        ? `${globalConfig.jira.base_url.replace(/^https?:\/\//, '')} | jira: enabled` 
        : '集成未激活'
    },
    {
      id: 'ai' as const,
      title: '需求解构引擎',
      status: globalConfig.ai?.enabled ? ('online' as const) : ('offline' as const),
      description: globalConfig.ai?.enabled
        ? `大模型需求自解构已启用。当前模型：${globalConfig.ai.model || '未设置'}。可自动将自然语言解构并映射到多仓。`
        : '大模型集成未启用，自解构目前返回 Mock 演示数据。点击配置开启真实 AI 解构。',
      details: globalConfig.ai?.enabled 
        ? `${globalConfig.ai.base_url.replace(/^https?:\/\//, '')} | model: ${globalConfig.ai.model}` 
        : 'AI 引擎未激活'
    }
  ];

  function handleNavigate(id: string) {
    let section = id;
    if (id === 'feishu-bot' || id === 'feishu-bitable') {
      section = 'feishu';
    }
    dispatch('navigate-to-settings', { section });
  }

</script>

<section class="integration-section">
  <div class="section-header">
    <div class="header-left">
      <h2 class="section-title">集成状态中枢</h2>
      <span class="badge">Status Hub</span>
    </div>
  </div>

  <div class="grid-layout">
    {#each integrations as integration}
      <StatusCard 
        title={integration.title} 
        status={integration.status} 
        description={integration.description} 
        details={integration.details} 
      >
        <div slot="actions">
          {#if currentUserPermissions.includes('config:write')}
            <Button variant="ghost" on:click={() => handleNavigate(integration.id)}>
              配置
            </Button>
          {:else}
            <span class="text-xs text-slate-500 font-mono" style="opacity:0.6;" title="仅管理员可用">🔒 仅限管理员</span>
          {/if}
        </div>
      </StatusCard>
    {/each}
  </div>
</section>

<style>
  .integration-section {
    margin-bottom: 32px;
  }

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 24px;
    flex-wrap: wrap;
    gap: 16px;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .section-title {
    font-size: 1.5rem;
    font-weight: 700;
    color: #e2e8f0;
    margin: 0;
  }

  .badge {
    font-size: 0.75rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    background: #1e293b;
    color: #94a3b8;
    padding: 4px 8px;
    border-radius: 4px;
    border: 1px solid rgba(51, 65, 85, 0.4);
  }

  .grid-layout {
    display: grid;
    grid-template-columns: repeat(1, minmax(0, 1fr));
    gap: 24px;
  }

  @media (min-width: 768px) {
    .grid-layout {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (min-width: 1024px) {
    .grid-layout {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }
  }

  @media (min-width: 1280px) {
    .grid-layout {
      grid-template-columns: repeat(5, minmax(0, 1fr));
    }
  }
</style>
