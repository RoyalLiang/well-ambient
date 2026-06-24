<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import Steps from '../shared/Steps.svelte';
  import TextInput from '../shared/TextInput.svelte';
  import Switch from '../shared/Switch.svelte';
  import Button from '../shared/Button.svelte';
  import Alert from '../shared/Alert.svelte';

  const dispatch = createEventDispatcher();

  export let config: {
    enabled?: boolean;
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
  } = {
    enabled: false,
    app_id: '',
    app_secret: '',
    bot: { enabled: false, chat_group: '' },
    bitable: {
      enabled: false,
      app_token: '',
      table_id: '',
      status_column: '任务状态',
      task_id_column: '任务ID'
    }
  };

  export let saving = false;
  export let saveError = '';
  export let saveSuccess = false;
  export let lastUpdated = '';

  let currentStep = 1;
  const steps = ['应用凭证', '机器人设置', '多维表格', '保存应用'];
  let editing = false;
  let showAppSecretEditor = !config.app_secret;
  let showBitableTokenEditor = !config.bitable?.app_token;

  // Step 1 states
  let enabled = config.enabled ?? false;
  let appID = config.app_id || '';
  let appSecret = config.app_secret || '';
  let testingCreds = false;
  let credsError = '';
  let credsSuccess = '';
  let credsDetails = '';

  // Step 2 states
  let botEnabled = config.bot?.enabled ?? false;
  let chatGroup = config.bot?.chat_group || '';

  // Step 3 states
  let bitableEnabled = config.bitable?.enabled ?? false;
  let appToken = config.bitable?.app_token || '';
  let tableID = config.bitable?.table_id || '';
  let statusCol = config.bitable?.status_column || '任务状态';
  let taskIDCol = config.bitable?.task_id_column || '任务ID';
  let testingBitable = false;
  let bitableError = '';
  let bitableSuccess = '';
  let bitableDetails = '';
  $: isConfigured = !!(appID || appSecret || botEnabled || bitableEnabled || chatGroup || appToken || tableID);
  $: if (!editing && !saveSuccess) {
    enabled = config.enabled ?? false;
    appID = config.app_id || '';
    appSecret = config.app_secret || '';
    botEnabled = config.bot?.enabled ?? false;
    chatGroup = config.bot?.chat_group || '';
    bitableEnabled = config.bitable?.enabled ?? false;
    appToken = config.bitable?.app_token || '';
    tableID = config.bitable?.table_id || '';
    statusCol = config.bitable?.status_column || '任务状态';
    taskIDCol = config.bitable?.task_id_column || '任务ID';
    showAppSecretEditor = !config.app_secret;
    showBitableTokenEditor = !config.bitable?.app_token;
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
    credsError = '';
    credsSuccess = '';
    credsDetails = '';
    bitableError = '';
    bitableSuccess = '';
    bitableDetails = '';
  }

  function finishClose() {
    editing = false;
    dispatch('close');
  }

  async function testCredentials() {
    if (!appID || !appSecret) {
      credsError = '请填写 App ID 和 App Secret';
      return;
    }

    testingCreds = true;
    credsError = '';
    credsSuccess = '';
    credsDetails = '';

    try {
      const res = await fetch('/api/config/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: 'feishu',
          feishu: {
            app_id: appID,
            app_secret: appSecret,
            bot: { enabled: false, chat_group: '' },
            bitable: { enabled: false, app_token: '', table_id: '', status_column: '', task_id_column: '' }
          }
        })
      });

      if (!res.ok) throw new Error(`HTTP 错误: ${res.status}`);
      const data = await res.json();

      if (data.success) {
        credsSuccess = '凭证校验通过，成功获取 Feishu 接口访问令牌。';
        credsDetails = data.details || '';
      } else {
        credsError = data.message;
        credsDetails = data.details || '';
      }
    } catch (e: any) {
      credsError = '连接测试请求失败，请检查网络或后端状态';
      credsDetails = e.message;
    } finally {
      testingCreds = false;
    }
  }

  async function testBitable() {
    if (!appToken || !tableID) {
      bitableError = '请填写多维表格的 App Token 和 Table ID';
      return;
    }

    testingBitable = true;
    bitableError = '';
    bitableSuccess = '';
    bitableDetails = '';

    try {
      const res = await fetch('/api/config/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: 'feishu',
          feishu: {
            app_id: appID,
            app_secret: appSecret,
            bot: { enabled: false, chat_group: '' },
            bitable: {
              enabled: true,
              app_token: appToken,
              table_id: tableID,
              status_column: statusCol,
              task_id_column: taskIDCol
            }
          }
        })
      });

      if (!res.ok) throw new Error(`HTTP 错误: ${res.status}`);
      const data = await res.json();

      if (data.success) {
        bitableSuccess = '多维表格鉴权及连通性测试成功！';
        bitableDetails = data.details || '';
      } else {
        bitableError = data.message;
        bitableDetails = data.details || '';
      }
    } catch (e: any) {
      bitableError = '连通性测试失败';
      bitableDetails = e.message;
    } finally {
      testingBitable = false;
    }
  }

  async function saveConfig() {

    const updatedFeishu = {
      enabled: enabled,
      app_id: appID,
      app_secret: appSecret,
      bot: {
        enabled: botEnabled,
        chat_group: chatGroup
      },
      bitable: {
        enabled: bitableEnabled,
        app_token: appToken,
        table_id: tableID,
        status_column: statusCol,
        task_id_column: taskIDCol
      }
    };

    dispatch('save', {
      key: 'feishu',
      data: updatedFeishu
    });
  }

  function nextStep() {
    if (currentStep === 1) {
      if (!appID || !appSecret) {
        credsError = '请填写 App ID 和 App Secret';
        return;
      }
      currentStep = 2;
    } else if (currentStep === 2) {
      currentStep = 3;
    } else if (currentStep === 3) {
      currentStep = 4;
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
      <h4 class="success-title">飞书集成配置已成功应用！</h4>
      <p class="success-desc font-mono">机器人通知及多维表格同步功能配置已热重载生效。</p>
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
          <span class="overview-kicker font-mono">Feishu Integration</span>
          <h4>飞书配置状态摘要</h4>
          <p>默认以只读安全呈现各配置字段详情，支持右上角快速启用/禁用。</p>
        </div>
        <div style="display: flex; align-items: center; gap: 10px;">
          <span class="status-pill {enabled ? 'online' : 'warning'}">{enabled ? '已启用' : '已禁用'}</span>
          <Switch id="feishu-overview-toggle" bind:checked={enabled} on:change={saveConfig} />
        </div>
      </div>

      <div class="overview-grid">
        <div class="overview-row">
          <span>Feishu App ID</span>
          <strong class="font-mono">{appID || '-'}</strong>
        </div>
        <div class="overview-row">
          <span>App Secret 密钥</span>
          <strong>{appSecret ? '已配置 (已脱敏保护)' : '未配置'}</strong>
        </div>
        <div class="overview-row">
          <span>机器人通知</span>
          <strong>{botEnabled ? '已开启' : '未开启'} {chatGroup ? `(${chatGroup})` : ''}</strong>
        </div>
        <div class="overview-row">
          <span>多维表格同步</span>
          <strong>{bitableEnabled ? '已开启' : '未开启'}</strong>
        </div>
        {#if bitableEnabled}
          <div class="overview-row">
            <span>Bitable App Token</span>
            <strong class="font-mono">{appToken ? appToken.substring(0, 10) + '...' : '-'}</strong>
          </div>
          <div class="overview-row">
            <span>Table ID</span>
            <strong class="font-mono">{tableID || '-'}</strong>
          </div>
          <div class="overview-row">
            <span>任务 ID 列名 / 状态列名</span>
            <strong class="font-sans">{taskIDCol} / {statusCol}</strong>
          </div>
        {/if}
        <div class="overview-row">
          <span>最近更新时间</span>
          <strong>{formatUpdated(lastUpdated)}</strong>
        </div>
        <div class="overview-row">
          <span>健康状态</span>
          <strong class="text-success">{credsSuccess || credsError || bitableSuccess || bitableError || '已就绪'}</strong>
        </div>
      </div>

      {#if credsDetails || bitableDetails}
        <pre class="details-pre font-mono">{credsDetails || bitableDetails}</pre>
      {/if}

      <div class="overview-actions">
        <Button variant="secondary" loading={testingCreds} on:click={testCredentials}>健康检查</Button>
        {#if bitableEnabled}
          <Button variant="secondary" loading={testingBitable} on:click={testBitable}>Bitable 检查</Button>
        {/if}
        <Button variant="primary" on:click={openEditor}>编辑配置</Button>
      </div>
    </div>
  {:else}
    <Steps {currentStep} {steps} />

    {#if currentStep === 1}
    <div class="step-content">
      <div class="info-block">
        <h4>飞书开放平台应用凭证</h4>
        <p>请输入在飞书开放平台 (open.feishu.cn) 申请的<strong>企业自建应用</strong>凭证，以授权 well-ambient 调用飞书机器人及多维表格 API。</p>
      </div>

      <TextInput
        id="feishu-appid"
        label="Feishu App ID"
        placeholder="例如: cli_a1b2c3d4e5f6g7h8"
        bind:value={appID}
        required
        error={credsError}
      />

      {#if showAppSecretEditor}
        <TextInput
          id="feishu-secret"
          label="Feishu App Secret"
          placeholder="请输入应用的 App Secret"
          type="password"
          bind:value={appSecret}
          required
        />
      {:else}
        <div class="credential-collapsed">
          <div>
            <span>Feishu App Secret</span>
            <strong>已配置，当前默认脱敏折叠</strong>
          </div>
          <button type="button" on:click={() => showAppSecretEditor = true}>编辑凭证/高级配置</button>
        </div>
      {/if}

      {#if credsSuccess}
        <Alert type="success" title="凭证校验通过" message={credsSuccess}>
          {#if credsDetails}
            <pre class="details-pre font-mono">{credsDetails}</pre>
          {/if}
        </Alert>
      {/if}

      {#if credsError && credsDetails}
        <Alert type="error" title="校验失败" message={credsError}>
          <pre class="details-pre font-mono">{credsDetails}</pre>
        </Alert>
      {/if}

      <div class="actions">
        <Button variant="secondary" loading={testingCreds} on:click={testCredentials}>
          测试凭证有效性
        </Button>
        <Button variant="primary" on:click={nextStep}>
          下一步
        </Button>
      </div>
    </div>
  {:else if currentStep === 2}
    <div class="step-content">
      <div class="info-block">
        <h4>飞书机器人群组推送设置</h4>
        <p>启用群聊机器人后，well-ambient 系统将在捕获到 GitLab 代码状态变更（如 Merge Request 合并、分支推送）后，向群聊内无感推送同步交互卡片。</p>
      </div>

      <Switch
        id="bot-enabled"
        label="启用飞书机器人通知功能"
        bind:checked={botEnabled}
      />

      {#if botEnabled}
        <div class="form-sub-section">
          <TextInput
            id="chat-group"
            label="默认通知群组 Chat ID"
            placeholder="例如: oc_chatgroupid12345"
            bind:value={chatGroup}
            required={botEnabled}
            helperText="请输入飞书群聊的 chat_id。可通过在飞书群聊中添加您的自建机器人并向其发送 @ 消息或通过飞书开发者工具获取该 ID。"
          />
          
          <div class="bot-notice-box">
            <span class="notice-title">注意事项:</span>
            <ul class="notice-list">
              <li>请在飞书群设置 -> 群机器人中，添加您当前绑定的自建应用。</li>
              <li>如果机器人未加入该群聊，即使配置了正确的 Chat ID，消息也会推送失败。</li>
            </ul>
          </div>
        </div>
      {/if}

      <div class="actions">
        <Button variant="ghost" on:click={prevStep}>上一步</Button>
        <Button variant="primary" on:click={nextStep}>下一步</Button>
      </div>
    </div>
  {:else if currentStep === 3}
    <div class="step-content">
      <div class="info-block">
        <h4>飞书多维表格 (Bitable) 数据同步</h4>
        <p>启用此项，开发状态提交流转时，well-ambient 会自动更新映射的飞书多维数据表的列状态，实现多维看板双向无感式同步。</p>
      </div>

      <Switch
        id="bitable-enabled"
        label="启用多维表格双向同步"
        bind:checked={bitableEnabled}
      />

      {#if bitableEnabled}
        <div class="form-sub-section">
          {#if showBitableTokenEditor}
            <TextInput
              id="bitable-token"
              label="多维表格 App Token"
              placeholder="例如: bascnYourBitableAppToken"
              bind:value={appToken}
              required={bitableEnabled}
              helperText="多维表格的 App Token (URL 中 /base/ 后面的一串字符)。"
            />
          {:else}
            <div class="credential-collapsed">
              <div>
                <span>多维表格 App Token</span>
                <strong>已配置，当前默认脱敏折叠</strong>
              </div>
              <button type="button" on:click={() => showBitableTokenEditor = true}>编辑凭证/高级配置</button>
            </div>
          {/if}

          <TextInput
            id="bitable-tableid"
            label="数据表 Table ID"
            placeholder="例如: tblYourTableID"
            bind:value={tableID}
            required={bitableEnabled}
            helperText="表格标签页内的 Table ID (URL 中 table= 后面的一串字符)。"
          />

          <div class="column-mapping-grid">
            <TextInput
              id="col-status"
              label="状态列字段名 (Status Column)"
              placeholder="任务状态"
              bind:value={statusCol}
              required={bitableEnabled}
            />
            <TextInput
              id="col-taskid"
              label="任务 ID 列字段名 (Task ID Column)"
              placeholder="任务ID"
              bind:value={taskIDCol}
              required={bitableEnabled}
            />
          </div>

          {#if bitableSuccess}
            <Alert type="success" title="连通性测试通过" message={bitableSuccess}>
              {#if bitableDetails}
                <pre class="details-pre font-mono">{bitableDetails}</pre>
              {/if}
            </Alert>
          {/if}

          {#if bitableError}
            <Alert type="error" title="测试失败" message={bitableError}>
              {#if bitableDetails}
                <pre class="details-pre font-mono">{bitableDetails}</pre>
              {/if}
            </Alert>
          {/if}

          <div class="test-row">
            <Button variant="secondary" loading={testingBitable} on:click={testBitable}>
              测试多维表格连接
            </Button>
          </div>
        </div>
      {/if}

      <div class="actions">
        <Button variant="ghost" on:click={prevStep}>上一步</Button>
        <Button variant="primary" on:click={nextStep}>下一步</Button>
      </div>
    </div>
  {:else if currentStep === 4}
    <div class="step-content">
      <div class="info-block">
        <h4>飞书集成配置摘要</h4>
        <p>确认无误后点击下方按钮应用并应用配置：</p>
      </div>

      <div class="summary-card">
        <div class="summary-row">
          <span class="summary-label">App ID:</span>
          <span class="summary-value font-mono">{appID}</span>
        </div>
        <div class="summary-row">
          <span class="summary-label">飞书消息推送机器人:</span>
          <span class="summary-value">
            {#if botEnabled}
              <span class="text-success">已启用</span> (群 ID: <span class="font-mono">{chatGroup}</span>)
            {:else}
              <span class="text-muted">已禁用</span>
            {/if}
          </span>
        </div>
        <div class="summary-row">
          <span class="summary-label">多维表格数据同步:</span>
          <span class="summary-value">
            {#if bitableEnabled}
              <span class="text-success">已启用</span> (Table: <span class="font-mono">{tableID}</span>)
            {:else}
              <span class="text-muted">已禁用</span>
            {/if}
          </span>
        </div>
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

  .form-sub-section {
    border-top: 1px solid rgba(51, 65, 85, 0.4);
    padding-top: 20px;
    margin-top: 8px;
    display: flex;
    flex-direction: column;
  }

  .bot-notice-box {
    background: rgba(245, 158, 11, 0.05);
    border: 1px solid rgba(245, 158, 11, 0.2);
    border-radius: 8px;
    padding: 12px 16px;
    margin-top: 12px;
    margin-bottom: 20px;
  }

  .notice-title {
    color: #fbbf24;
    font-size: 0.8rem;
    font-weight: 600;
    display: block;
    margin-bottom: 6px;
  }

  .notice-list {
    margin: 0;
    padding-left: 20px;
    font-size: 0.775rem;
    color: #cbd5e1;
    line-height: 1.5;
  }

  .column-mapping-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px;
    margin-bottom: 16px;
  }

  .test-row {
    margin-bottom: 20px;
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

  .text-success {
    color: #34d399;
  }

  .text-muted {
    color: #64748b;
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
